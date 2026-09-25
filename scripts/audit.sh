#!/usr/bin/env bash
# scripts/audit.sh — статический аудит репозитория cardinal
# Запуск: bash scripts/audit.sh [--strict]
# Exit: 0 — чисто (нет FAIL, и нет WARN при --strict)
#       1 — есть FAIL
#       2 — есть WARN и передан --strict
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || { echo "cannot cd to $ROOT" >&2; exit 1; }

STRICT=0
[[ "${1:-}" == "--strict" ]] && STRICT=1

FAIL=0
WARN=0

if [[ -t 1 ]]; then
	C_PASS=$'\033[32m'; C_WARN=$'\033[33m'; C_FAIL=$'\033[31m'; C_RST=$'\033[0m'
else
	C_PASS=""; C_WARN=""; C_FAIL=""; C_RST=""
fi
pass()    { printf "  %sPASS%s  %s\n" "$C_PASS" "$C_RST" "$1"; }
warn()    { printf "  %sWARN%s  %s\n" "$C_WARN" "$C_RST" "$1"; WARN=$((WARN+1)); }
fail()    { printf "  %sFAIL%s  %s\n" "$C_FAIL" "$C_RST" "$1"; FAIL=$((FAIL+1)); }
section() { printf "\n== %s ==\n" "$1"; }

have() { command -v "$1" >/dev/null 2>&1; }

TMPDIR_AUDIT="$(mktemp -d 2>/dev/null || mktemp -d -t cardinal-audit)"
cleanup() { rm -rf "$TMPDIR_AUDIT"; }
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# 1. Secrets & sensitive data in tree
# ---------------------------------------------------------------------------
section "1. Secrets / credentials in repo"

if git ls-files 2>/dev/null \
		| grep -E '(^|/)\.env$|/credentials|/id_rsa|/id_ed25519|\.pem$|\.key$' >/dev/null; then
	fail ".env / private keys / credentials ARE tracked in git"
else
	pass "no tracked .env / private keys"
fi

PW_HITS="$TMPDIR_AUDIT/pw.txt"
grep -RnE '(MYSQL_ROOT_PASSWORD|password|secret|token|api_key)[[:space:]]*[:=][[:space:]]*["'"'"']?[A-Za-z0-9_-]{4,}' \
	compose.yaml docs/en docs/ru 2>/dev/null \
	| grep -v -E '\$\{|\.env|example|EXAMPLE' >"$PW_HITS" || true
if [[ -s "$PW_HITS" ]]; then
	warn "plain-looking password in docs/compose — review $PW_HITS"
else
	pass "no obvious plain credentials in docs/"
fi

# ---------------------------------------------------------------------------
# 2. Supply chain & build
# ---------------------------------------------------------------------------
section "2. Supply chain & build"

# SHA256 verification in installers (errexit checked separately in §6).
for f in install.sh install.ps1 install-appimage.sh scripts/install-apt.sh; do
	[[ -f "$f" ]] || continue
	if grep -qE 'sha256sum|--check|CHECKSUM|Get-FileHash' "$f"; then
		pass "$f verifies SHA256"
	else
		case "$f" in
			install-appimage.sh)
				warn "$f does not verify SHA256 (installs from local AppImage only)" ;;
			install.ps1)
				warn "$f builds from source (no SHA256 verification of clone)" ;;
			*)
				fail "$f does NOT verify SHA256 of downloaded binary" ;;
		esac
	fi
done

if grep -rqE 'cosign|minisign|gpg --armor --detach' .github/ Makefile 2>/dev/null; then
	pass "release artifacts are signed"
else
	fail "release artifacts are NOT signed (no cosign / minisign / gpg)"
fi

if grep -rqE 'syft|spdx|cyclonedx' .github/ Makefile 2>/dev/null; then
	pass "SBOM generation configured"
else
	warn "no SBOM generation step (syft / spdx / cyclonedx)"
fi

if grep -qE 'buildvcs=false|-trimpath' Makefile .goreleaser.yaml 2>/dev/null; then
	pass "reproducible-build flags present"
else
	warn "no -trimpath / buildvcs=false in build flags"
fi

# ---------------------------------------------------------------------------
# 3. Tests
# ---------------------------------------------------------------------------
section "3. Tests"

if grep -rq 'func Fuzz' cmd/ internal/ 2>/dev/null; then
	pass "has fuzz tests"
else
	warn "no fuzz tests (FuzzParseDockerfile, FuzzValidateImageRef, ...)"
fi

if grep -q 'go test -race' .github/workflows/*.yml 2>/dev/null; then
	pass "race detector enabled in CI"
else
	warn "race detector NOT in CI (-race flag absent)"
fi

if find . -name 'coverage.out' -mmin -1440 -print -quit 2>/dev/null | grep -q .; then
	pass "fresh coverage profile exists"
else
	warn "no coverage.out under 24h"
fi

# ---------------------------------------------------------------------------
# 4. API security
# ---------------------------------------------------------------------------
section "4. API security"

API=internal/api/server.go
if [[ -f "$API" ]]; then
	if grep -q 'subtle.ConstantTimeCompare' "$API"; then
		pass "constant-time token compare"
	else
		fail "no constant-time token compare"
	fi
	if grep -q 'http.MaxBytesReader' "$API"; then
		pass "request body size limit"
	else
		fail "no MaxBytesReader on request bodies"
	fi
	if grep -qE 'ReadHeaderTimeout|ReadTimeout|WriteTimeout' "$API"; then
		pass "server timeouts configured"
	else
		fail "no ReadHeaderTimeout/ReadTimeout/WriteTimeout"
	fi
	if grep -qE 'rate[[:space:]]*\(|Limiter' internal/api/*.go 2>/dev/null; then
		pass "rate limiter present"
	else
		warn "no per-IP rate limiter on API"
	fi
	if grep -qE 'metricsHandler|authMiddleware\(promhttp' internal/api/*.go 2>/dev/null; then
		pass "/metrics is gated by auth/loopback policy"
	elif grep -q 'metrics' "$API"; then
		warn "/metrics endpoint exposed - verify it is bound to loopback or auth"
	else
		pass "/metrics not exposed"
	fi
else
	warn "$API not found — skipping API checks"
fi

# ---------------------------------------------------------------------------
# 5. Container runtime hardening
# ---------------------------------------------------------------------------
section "5. Runtime hardening"

if grep -qE 'DefaultSeccompProfile|seccomp\.Default' internal/container/*.go 2>/dev/null; then
	pass "default seccomp profile wired"
else
	fail "no default seccomp profile"
fi

if grep -rqE 'NoNewPrivileges|no_new_privs|PR_SET_NO_NEW_PRIVS' internal/container/ 2>/dev/null; then
	pass "no_new_privs handling present"
else
	fail "no no_new_privs handling"
fi

# Look for capability drop context, not just bare mentions.
if grep -rqE '(Drop|dropCapabilities|dropCaps)[^\n]*SYS_ADMIN|SYS_ADMIN[^\n]*(Drop|drop)' \
		internal/container/ 2>/dev/null; then
	pass "dangerous capabilities being dropped (SYS_ADMIN / SYS_MODULE in drop context)"
elif grep -rqE 'SYS_MODULE' internal/container/ 2>/dev/null; then
	warn "SYS_MODULE mentioned but not clearly in a drop list — manual review"
else
	fail "no SYS_ADMIN / SYS_MODULE drop"
fi

if grep -rqE 'IsProtectedHostPath|isProtectedHostPath|protectedHostPath' internal/container/ 2>/dev/null; then
	pass "bind-mount host-path protection present"
else
	fail "no bind-mount host-path protection"
fi

# ---------------------------------------------------------------------------
# 6. Install scripts (supply-chain at install time)
# ---------------------------------------------------------------------------
section "6. Installers"

for f in install.sh install.ps1 install-appimage.sh scripts/install-apt.sh; do
	[[ -f "$f" ]] || continue
	case "$f" in
		*.ps1)
			if grep -qE '\$ErrorActionPreference[[:space:]]*=[[:space:]]*[\"'\"']Stop[\"'\"']' "$f"; then
				pass "$f sets \$ErrorActionPreference = 'Stop'"
			else
				warn "$f missing \$ErrorActionPreference = 'Stop'"
			fi
			;;
		*)
			if grep -qE '^[[:space:]]*set[[:space:]]+-[a-zA-Z]*e' "$f"; then
				pass "$f has set -e / errexit"
			else
				warn "$f missing 'set -e' / 'errexit'"
			fi
			;;
	esac
done

# APT-repo signature — ignore comment lines and legacy references.
if grep -vE '^[[:space:]]*(#|.*Removing legacy|.*was using)' scripts/add-apt-repo.sh 2>/dev/null \
		| grep -qE 'trusted=yes'; then
	fail "APT repo configured with [trusted=yes] (no signature verification)"
elif grep -qE 'Signed-By' scripts/add-apt-repo.sh 2>/dev/null; then
	pass "APT repo uses signed-by keyring"
else
	warn "APT repo signature not detected (manual review needed)"
fi

# ---------------------------------------------------------------------------
# 7. CI hygiene
# ---------------------------------------------------------------------------
section "7. CI/CD"

if grep -q 'govulncheck' .github/workflows/*.yml 2>/dev/null; then
	pass "govulncheck in CI"
else
	fail "govulncheck missing from CI"
fi

if grep -q 'golangci-lint' .github/workflows/*.yml 2>/dev/null; then
	pass "golangci-lint in CI"
else
	fail "golangci-lint missing from CI"
fi

if [[ -f .github/dependabot.yml ]] || [[ -f .github/renovate.json ]] \
		|| [[ -f renovate.json ]] || [[ -f .renovaterc.json ]]; then
	pass "automated dependency updates configured"
else
	warn "no Dependabot / Renovate for Go modules and GitHub Actions"
fi

if grep -qE 'pull_request' .github/workflows/e2e.yml 2>/dev/null; then
	pass "E2E runs on PR"
else
	warn "E2E workflow is manual only (workflow_dispatch)"
fi

if grep -rqE 'permissions:' .github/workflows/ 2>/dev/null; then
	pass "GitHub Actions permissions declared"
else
	warn "GitHub Actions permissions not declared (consider OIDC / least-priv)"
fi

# ---------------------------------------------------------------------------
# 8. Manifest hygiene
# ---------------------------------------------------------------------------
section "8. Manifests & hygiene"

if grep -q 'errcheck' .golangci.yml 2>/dev/null; then
	pass "errcheck enabled"
else
	warn "errcheck not in .golangci.yml"
fi

if [[ -f LICENSE ]]; then
	pass "LICENSE file present"
else
	fail "missing LICENSE"
fi

if grep -rq 'spf13/cobra' cmd/ internal/ go.mod 2>/dev/null; then
	pass "cobra command framework wired (shell completion + global flags)"
else
	warn "no cobra/spf13 framework (hand-rolled CLI dispatch)"
fi

# CI architecture shape.
workflow_files=$(grep -lE '^jobs:' .github/workflows/*.yml 2>/dev/null | wc -l)
reusables=$(grep -l 'workflow_call' .github/workflows/*.yml 2>/dev/null | wc -l)
callers=$(grep -l 'uses: \./\.github/workflows/' .github/workflows/*.yml 2>/dev/null | wc -l)
if (( reusables == 0 )); then
	pass "single CI pipeline (workflow files=$workflow_files, no reusable layer)"
elif (( callers >= 2 && reusables >= 2 )); then
	pass "CI uses reusable workflows (reusables=$reusables callers=$callers)"
else
	warn "CI has partial reusable workflow usage (reusables=$reusables callers=$callers)"
fi

if grep -rq 'cancel-in-progress:' .github/workflows/ 2>/dev/null; then
	pass "cancel-in-progress configured"
else
	warn "no cancel-in-progress on PR runs (stale jobs will keep wasting runners)"
fi

# Command surface consistency.
declare -A COBRA_NAME_ALIAS=(
	[start]=StartCmd
	[console-serve]=ConsoleServe
	[version]=versionCommand
	[init]=initContainer
	[helper-mount]=HelperMount
)
if [[ -f cmd/cobra_commands.go ]]; then
	missing=0
	registered=$(grep -oE 'register\(commandSpec\{"[^"]+"' cmd/cobra_commands.go \
		| sed -E 's/.*"([^"]+)"/\1/')
	registered_count=0
	if [[ -n "$registered" ]]; then
		while IFS= read -r name; do
			[[ -z "$name" ]] && continue
			case "$name" in
				completion|help) continue ;;
			esac
			registered_count=$((registered_count+1))
			fname="${COBRA_NAME_ALIAS[$name]:-}"
			if [[ -z "$fname" ]]; then
				fname="$(printf '%s' "$name" | awk -F- '{for(i=1;i<=NF;i++) printf toupper(substr($i,1,1)) substr($i,2)}')"
			fi
			if ! grep -rqE "^func ${fname}\(" cmd/*.go; then
				echo "missing implementation: $name (looked for func $fname)"
				missing=$((missing+1))
			fi
		done <<< "$registered"
	fi
	if (( missing > 0 )); then
		fail "$missing registered cobra commands have no matching func X(...) implementation"
	else
		pass "every registered cobra command has a matching implementation (count=$registered_count)"
	fi
else
	warn "cmd/cobra_commands.go not found — skipping command-surface check"
fi

# go.mod tidy (skip when vendored).
TIDY_OUT="$TMPDIR_AUDIT/tidy.txt"
if have go && [[ -d vendor ]]; then
	pass "vendored dependencies present (tidy diff skipped)"
elif have go && go mod tidy -diff >"$TIDY_OUT" 2>&1; then
	pass "go.mod is tidy"
elif [[ -s "$TIDY_OUT" ]]; then
	warn "go.mod not tidy - review $TIDY_OUT"
else
	warn "go not installed — skipped go.mod tidy check"
fi

# Plaintext FTP server must stay out of the runtime.
if find internal -iname '*ftp*' -print -quit 2>/dev/null | grep -q .; then
	fail "internal/*ftp* present — see SECURITY.md 'File transfer' before re-introducing cleartext FTP"
else
	pass "no internal/ftp package (cleartext FTP server not shipped)"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
printf "WARN: %d,  FAIL: %d\n" "$WARN" "$FAIL"

if (( FAIL > 0 )); then
	exit 1
fi
if (( STRICT )) && (( WARN > 0 )); then
	exit 2
fi
exit 0
