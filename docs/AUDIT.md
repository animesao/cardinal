<!-- dck-version:start -->
**Documentation version:** `1.60.3`
**Project release:** `v1.60.3`
<!-- dck-version:end -->

# dck — Continuous audit

This document describes how the repository is audited automatically and what
every check enforces.

## `scripts/audit.sh`

A static analyser that runs on local developer machines (via `make audit`)
and on every push / pull request (`.github/workflows/audit.yml`). It exits
with:

* `0` — no failures (warnings allowed);
* `1` — at least one `FAIL` was recorded;
* `2` — `WARN` only when `--strict` was passed.

The script is intentionally side-effect free: it only reads files and, in
one optional step (`go mod tidy -diff`), produces a diff in `/tmp` that
operators can ignore for vendored projects.

### Sections

| # | Title | Purpose |
|---|-------|---------|
| 1 | Secrets / credentials in repo | Warns about `.env`/`.pem`/credentials tracked in git and obvious plain-text passwords in documentation. |
| 2 | Supply chain & build | SHA256 verification of installers, signed release artifacts, SBOM generation, reproducible-build flags. |
| 3 | Tests | Fuzz tests, race detector usage in CI, coverage profile freshness. |
| 4 | API security | Constant-time token compare, request body size cap, server timeouts, per-IP rate limiter, `/metrics` gating. |
| 5 | Runtime hardening | Seccomp default, `no_new_privs`, dangerous-cap drop, host-path blocklist. |
| 6 | Installers | `errexit` for every installer, APT repository signature. |
| 7 | CI/CD | `govulncheck`, `golangci-lint`, Dependabot or Renovate, E2E on PR, permissions minimal-default. |
| 8 | Manifests & hygiene | Linter config, LICENSE, vendored vs tidy `go.mod`. |

### Adding new checks

1. Add a labelled `section "..."` block.
2. Use the helpers `pass`, `warn`, `fail`. Each call increments one of the
   global counters (`WARN`, `FAIL`).
3. The block must not modify the working tree — the audit is part of every
   pre-commit gate.

## Current state — output snapshot

The full output is reproduced in the CI workflow logs; abbreviated:

```
== 1. Secrets / credentials in repo ==
  PASS  no tracked .env / private keys
  WARN  plain-looking password in docs/compose
== 2. Supply chain & build ==
  PASS  install.sh verifies SHA256
  PASS  install.ps1 has errexit
  PASS  install-appimage.sh has errexit
  PASS  scripts/install-apt.sh verifies SHA256
  PASS  release artifacts are signed
  WARN  no SBOM generation step
  PASS  reproducible-build flags present
== 3. Tests ==
  WARN  no fuzz tests
  PASS  race detector enabled in CI
  PASS  fresh coverage profile exists
== 4. API security ==
  PASS  constant-time token compare
  PASS  request body size limit
  PASS  server timeouts configured
  PASS  rate limiter present
  PASS  /metrics is gated by auth/loopback policy
== 5. Runtime hardening ==
  PASS  default seccomp profile wired
  PASS  no_new_privs handling present
  PASS  dangerous capabilities being dropped
  PASS  bind-mount host-path protection present
== 6. Installers ==
  PASS  install.sh has errexit
  PASS  install-appimage.sh has errexit
  PASS  scripts/install-apt.sh has errexit
  PASS  APT repo uses signed-by keyring
== 7. CI/CD ==
  PASS  govulncheck in CI
  PASS  golangci-lint in CI
  PASS  automated dependency updates configured
  PASS  E2E runs on PR
  PASS  GitHub Actions permissions declared
== 8. Manifests & hygiene ==
  PASS  errcheck enabled
  PASS  LICENSE file present
  PASS  vendored dependencies present (tidy diff skipped)
```

The remaining warnings are kept in scope on purpose:

* **docs/compose.** has placeholder passwords (`rootpass`, `botpass`) — the
  maintainer-driven cleanup is tracked as a documentation task; do not
  silently substitute them with `${...}` because users legitimately
  copy-paste examples verbatim.
* **install.ps1** builds from `main` HEAD via `git clone`. SHA256
  verification is meaningless without a tag pin; tracked in
  `TODO install.ps1 commit pinning`.
* **install-appimage.sh** installs from a local AppImage rather than
  network; SHA256 verification against the SHA256SUMS.txt file is opt-in
  via the `--verify` AppRun argument.

## Removed: `internal/ftp/`

The package contained a 398-line plain-TCP FTP server with a single
shared password, no TLS, no per-user accounts, no audit log, and no CLI
surface. It had been unreachable (no caller, no docs entry) since the
project first imported cobra-ready commands. Security review concluded
that patching it was riskier than deleting it: any future commit that
imported the package would have exposed the runtime host to anonymous
FTP probing on a privileged port because `cmd/` already passes
through to syscall-making helpers.

Removal recorded under commit message: `chore(security): drop
unmaintained internal/ftp; recommend sftp/dck cp in SECURITY.md`. The
rationale and the recommended replacement stack are documented in
`SECURITY.md` § *File transfer into and out of containers*.

## Continuous process

* Audit triggers on every PR (`make audit-strict`).
* Daily `govulncheck` via `.github/workflows/scheduled-vuln-scan.yml`.
* Dependabot weekly (`/.github/dependabot.yml`).
* Secret scan weekly (`gitleaks/gitleaks-action@v2` with the
  `/gitleaks.toml` allowlist for documented placeholders).
