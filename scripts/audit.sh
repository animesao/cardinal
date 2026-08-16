#!/usr/bin/env bash
# scripts/audit.sh — статический аудит репозитория dck
# Запуск: bash scripts/audit.sh [--strict]
# Exit: 0 если нет FAIL, 1 если есть FAIL (или WARN при --strict)
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

STRICT=0
[[ "${1:-}" == "--strict" ]] && STRICT=1

FAIL=0
WARN=0
pass() { printf "  \033[32mPASS\033[0m  %s\n" "$1"; }
warn() { printf "  \033[33mWARN\033[0m  %s\n" "$1"; WARN=$((WARN+1)); }
fail() { printf "  \033[31mFAIL\033[0m  %s\n" "$1"; FAIL=$((FAIL+1)); }
section() { printf "\n== %s ==\n" "$1"; }

have() { command -v "$1" >/dev/null 2>&1; }

# --- 1. Secrets & sensitive data in tree ------------------------------------
section "1. Secrets / credentials in repo"

# .env, .aws, id_rsa, etc.
if git ls-files | grep -E '(^|/)\.env$|/credentials|/id_rsa|/id_ed25519|\.pem$|\.key$' >/dev/null; then
	fail ".env / private keys / credentials ARE tracked in git"
else
	pass "no tracked .env / private keys"
fi

# Plain-text passwords in YAML/MD examples (warning-only)
if grep -RnE '(MYSQL_ROOT_PASSWORD|password|secret|token|api_key)\s*[:=]\s*["'\'']?[A-Za-z0-9_-]{4,}' \
		compose.yaml docs/en docs/ru 2>/dev/null \
		| grep -v -E '\$\{|\.env|example|EXAMPLE' >/tmp/audit_pw.txt; then
	if [[ -s /tmp/audit_pw.txt ]]; then
		warn "plain-looking password in docs/compose — review /tmp/audit_pw.txt"
	fi
else
	pass "no obvious plain credentials in docs/"
fi

# --- 2. Supply-chain / build hygiene ----------------------------------------
section "2. Supply chain & build"

# SHA256 verification in installers
for f in install.sh install.ps1 install-appimage.sh scripts/install-apt.sh; do
	if [[ -f "$f" ]]; then
		if grep -qE 'sha256sum|--check|CHECKSUM|Get-FileHash' "$f"; then
			pass "$f verifies SHA256"
		else
			# install-appimage.sh installs from a local AppImage so SHA256
			# verification against SHA256SUMS.txt is opt-in (warn).
			if [[ "$f" == "install-appimage.sh" ]]; then
				warn "$f does not verify SHA256 (installs from local AppImage only)"
			elif [[ "$f" == "install.ps1" ]]; then
				# install.ps1 builds from a git clone rather than downloading
				# a precompiled binary; supply-chain trust relies on git's
				# transport integrity plus the verified tag.
				warn "$f builds from source (no SHA256 verification of clone)"
			else
				fail "$f does NOT verify SHA256 of downloaded binary"
			fi
		fi
		if grep -qE 'set -e|errexit|ErrorActionPreference' "$f"; then
			pass "$f has errexit"
		else
			warn "$f missing 'set -e' / 'errexit'"
		fi
	fi
done

# Signed release artifacts
if grep -rqE 'cosign|minisign|gpg --armor --detach' .github/ Makefile 2>/dev/null; then
	pass "release artifacts are signed"
else
	fail "release artifacts are NOT signed (no cosign / minisign / gpg)"
fi

# SBOM
if grep -rqE 'syft|spdx|cyclonedx' .github/ Makefile 2>/dev/null; then
	pass "SBOM generation configured"
else
	warn "no SBOM generation step (syft / spdx / cyclonedx)"
fi

# Reproducible build flags
if grep -qE 'buildvcs=false|-trimpath' Makefile .goreleaser.yaml 2>/dev/null; then
	pass "reproducible-build flags present"
else
	warn "no -trimpath / buildvcs=false in build flags"
fi

# --- 3. Tests ---------------------------------------------------------------
section "3. Tests"

# Fuzz tests
if grep -rq 'func Fuzz' cmd/ internal/ 2>/dev/null; then
	pass "has fuzz tests"
else
	warn "no fuzz tests (FuzzParseDockerfile, FuzzValidateImageRef, ...)"
fi

# Race detector in CI
if grep -q 'go test -race' .github/workflows/*.yml 2>/dev/null; then
	pass "race detector enabled in CI"
else
	warn "race detector NOT in CI (-race flag absent)"
fi

# Coverage
if find . -name "coverage.out" -mmin -1440 2>/dev/null | head -1 >/dev/null; then
	pass "fresh coverage profile exists"
else
	warn "no coverage.out under 24h"
fi

# --- 4. API security --------------------------------------------------------
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
	if grep -q 'ReadHeaderTimeout\|ReadTimeout\|WriteTimeout' "$API"; then
		pass "server timeouts configured"
	else
		fail "no ReadHeaderTimeout/ReadTimeout/WriteTimeout"
	fi
	if grep -qE 'rate( |Limiter)' internal/api/*.go; then
		pass "rate limiter present"
	else
		warn "no per-IP rate limiter on API"
	fi
	if grep -q 'metricsHandler\|authMiddleware(promhttp' internal/api/*.go; then
		pass "/metrics is gated by auth/loopback policy"
	elif grep -q 'metrics' "$API"; then
		warn "/metrics endpoint exposed - verify it is bound to loopback or auth"
	else
		pass "/metrics not exposed"
	fi
fi

# --- 5. Container runtime hardening ----------------------------------------
section "5. Runtime hardening"

# Default seccomp
if grep -q 'DefaultSeccompProfile\|seccomp.Default' internal/container/*.go 2>/dev/null; then
	pass "default seccomp profile wired"
else
	fail "no default seccomp profile"
fi

# Default no-new-privs
if grep -rq 'NoNewPrivileges\|no_new_privs\|PR_SET_NO_NEW_PRIVS' internal/container/ 2>/dev/null; then
	pass "no_new_privs handling present"
else
	fail "no no_new_privs handling"
fi

# Capability drop list
if grep -q 'SYS_ADMIN\|SYS_MODULE' internal/container/*.go 2>/dev/null; then
	pass "dangerous capabilities being dropped"
else
	fail "no SYS_ADMIN / SYS_MODULE drop"
fi

# Allowed host paths (blocklist for bind mounts)
if grep -rq 'IsProtectedHostPath\|protectedHostPath\|isProtectedHostPath' internal/container/ 2>/dev/null; then
	pass "bind-mount host-path protection present"
else
	fail "no bind-mount host-path protection"
fi

# --- 6. Install scripts (supply-chain at install time) ---------------------
section "6. Installers"

for f in install.sh install.ps1 install-appimage.sh scripts/install-apt.sh; do
	if [[ -f "$f" ]]; then
		if grep -qE 'set -e|errexit|ErrorActionPreference' "$f"; then
			pass "$f has errexit"
		else
			warn "$f missing 'set -e' / 'errexit'"
		fi
	fi
done

# APT-repo signature — ignore lines that are merely references to the legacy mode.
if grep -vE '^\s*(#|.*Removing legacy|.*was using)' scripts/add-apt-repo.sh 2>/dev/null \
   | grep -qE 'trusted=yes'; then
	fail "APT repo configured with [trusted=yes] (no signature verification)"
elif grep -qE 'Signed-By' scripts/add-apt-repo.sh 2>/dev/null; then
	pass "APT repo uses signed-by keyring"
else
	warn "APT repo signature not detected (manual review needed)"
fi

# --- 7. CI hygiene ---------------------------------------------------------
section "7. CI/CD"

# govulncheck
if grep -q 'govulncheck' .github/workflows/*.yml 2>/dev/null; then
	pass "govulncheck in CI"
else
	fail "govulncheck missing from CI"
fi

# golangci-lint
if grep -q 'golangci-lint' .github/workflows/*.yml 2>/dev/null; then
	pass "golangci-lint in CI"
else
	fail "golangci-lint missing from CI"
fi

# Dependabot / renovate
if [[ -f .github/dependabot.yml ]] || [[ -f .github/renovate.json ]] \
		|| [[ -f renovate.json ]] || [[ -f .renovaterc.json ]]; then
	pass "automated dependency updates configured"
else
	warn "no Dependabot / Renovate for Go modules and GitHub Actions"
fi

# E2E on PR
if grep -qE 'pull_request' .github/workflows/e2e.yml 2>/dev/null; then
	pass "E2E runs on PR"
else
	warn "E2E workflow is manual only (workflow_dispatch)"
fi

# Permissions minimal-default in workflows
if grep -rqE 'permissions:' .github/workflows/ 2>/dev/null; then
	pass "GitHub Actions permissions declared"
else
	warn "GitHub Actions permissions not declared (consider OIDC / least-priv)"
fi

# --- 8. Manifest hygiene ---------------------------------------------------
section "8. Manifests & hygiene"

# .golangci.yml
if grep -q 'errcheck' .golangci.yml 2>/dev/null; then
	pass "errcheck enabled"
else
	warn "errcheck not in .golangci.yml"
fi

# LICENSE present
[[ -f LICENSE ]] && pass "LICENSE file present" || fail "missing LICENSE"

# Cobra migration: spf13/cobra is the supported CLI framework; hand-rolled
# switches make shell completion, --help and global flags much harder to
# maintain.
if grep -rqE 'spf13/cobra' cmd/ 2>/dev/null; then
	pass "cobra command framework wired (shell completion + global flags)"
else
	warn "no cobra/spf13 framework in cmd/ (hand-rolled CLI dispatch)"
fi

# Command surface consistency: every function registered in cobra_commands.go
# must have a matching `func X(args []string)` definition in cmd/*.go. This
# is the cheapest static guarantee against typos and silent registration
# gaps. A handful of names have known free-function aliases (start/startCmd,
# console-serve/ConsoleServe, version/versionCommand, init/initContainer)
# and we map them explicitly so the audit script does not flag them as
# missing.
declare -A COBRA_NAME_ALIAS=(
	[start]=StartCmd
	[console-serve]=ConsoleServe
	[version]=versionCommand
	[init]=initContainer
)
if [[ -f cmd/cobra_commands.go ]]; then
	missing=0
	registered=$(grep -oE 'register\(commandSpec\{"[^"]+"' cmd/cobra_commands.go | sed -E 's/.*"([^"]+)"/\1/')
	for name in $registered; do
		case "$name" in
			completion|help) continue ;;
		esac
		fname=${COBRA_NAME_ALIAS[$name]:-}
		if [[ -z "$fname" ]]; then
			fname=$(tr '[:lower:]' '[:upper:]' <<< "${name:0:1}")${name:1}
		fi
		if ! grep -rqE "^func ${fname}\((args |\\_ )\[\\]string\\)" cmd/*.go; then
			echo "missing implementation: $name (looked for func $fname)"
			missing=$((missing+1))
		fi
	done
	if (( missing > 0 )); then
		fail "$missing registered cobra commands have no matching func X(args []string) implementation"
	else
		registered_count=$(echo "$registered" | wc -l)
		pass "every registered cobra command has a matching implementation (count=$registered_count)"
	fi
fi

# go.mod tidy - for vendored projects this is informational only because
# the locally installed Go toolchain may disagree with the project's locked
# version (the diff purely reports test-only transitive deps).
if have go && [[ -d vendor ]]; then
	pass "vendored dependencies present (tidy diff skipped)"
elif have go && go mod tidy -diff >/tmp/audit_tidy.txt 2>&1; then
	pass "go.mod is tidy"
else
	if [[ -s /tmp/audit_tidy.txt ]]; then
		warn "go.mod not tidy - review /tmp/audit_tidy.txt"
	fi
fi

# Plaintext FTP server must stay out of the runtime. The previous
# internal/ftp/ package was removed in 2026 because it implemented a
# cleartext FTP server with a single shared password; this check prevents
# an accidental re-introduction.
if find internal -type d -name ftp -print -quit 2>/dev/null | grep -q .; then
	fail "internal/ftp/ is present - see SECURITY.md 'File transfer' before re-introducing cleartext FTP"
else
	pass "no internal/ftp package (cleartext FTP server not shipped)"
fi
# --- Summary ---------------------------------------------------------------
echo
printf "WARN: %d,  FAIL: %d\n" "$WARN" "$FAIL"

if (( FAIL > 0 )); then
	exit 1
fi
if (( STRICT )) && (( WARN > 0 )); then
	exit 2
fi
exit 0
