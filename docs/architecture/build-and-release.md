<!-- cardinal-version:start -->
**Documentation version:** `2.4.12`
**Project release:** `v2.4.12`
<!-- cardinal-version:end -->

# Build and release

This document records the toolchain, build targets, CI gates, and release
artifacts that exist in the repository today. It deliberately names files and
commands so a future maintainer can re-run them.

## Toolchain

- `go.mod` declares `go 1.26.0`.
- `go.mod`/CI pin toolchain `1.26.10`. The comment in `go.mod` explains why:
  1.26.10 is the first release with the `crypto/x509` fix for GO-2026-6809, and
  1.26.9 carried the `net/http`, `net/textproto`, and `crypto/tls` fixes for
  GO-2026-6603/6605/6607/6608/6610/6611/6612/6613/6617.
- `VERSION` is the release source of truth. Current value: `2.4.11`.
- The module is `cardinal`; the entry point is `main.go` → `cmd.Execute()`.
- Runtime packages are Linux-only (`//go:build linux`). Windows and macOS
  checkouts are for editing and cross-compilation, not execution.

`docs/build.md` still says Go 1.25 / module `go 1.25.0` / toolchain `1.26.6`
and references `.github/workflows/ci.yml` and `release.yml`. Those files do not
exist. See [`known-issues.md`](../status/known-issues.md).

## Makefile targets

| Target | Action |
|---|---|
| `build` | Build `cardinal-linux-amd64` and `cardinal-linux-arm64` |
| `build-amd64` | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo` |
| `build-arm64` | Same for `GOARCH=arm64` |
| `verify-cobra` | `go vet ./cmd/...` and `go build ./cmd/...` |
| `completion` / `completion-*` | Print or generate shell completion |
| `test` | `go test ./... -count=1` |
| `test-race` | `go test -race ./... -count=1` |
| `test-cover` | Coverage profile plus total |
| `lint` | `go vet ./...` plus `golangci-lint@v1.64.8` |
| `audit` | `bash scripts/audit.sh` |
| `audit-strict` | `bash scripts/audit.sh --strict` |
| `fuzz` | 30 s fuzz runs in `internal/builder` |
| `docs` | `sh scripts/sync-docs-version.sh` |
| `docs-links` | `python3 scripts/check-docs-links.py` |
| `docs-check` | Version-marker check plus the relative-link check |
| `deb` | Build binaries, then `scripts/build-deb.sh` |
| `release` | `deb`, then report `dist/cardinal_2.4.11_amd64.deb` |
| `clean` | Remove `cardinal*`, `dist/`, `coverage.out` |

Build flags in the Makefile and goreleaser:

```
-trimpath -ldflags="-s -w -buildid= -X cardinal/cmd.version=$(VERSION)"
```

`-trimpath`, `-buildid=`, and empty VCS stamping make the binaries more
reproducible. `CGO_ENABLED=0` keeps them static and portable across glibc and
musl hosts.

## Continuous integration

### `.github/workflows/build.yml`

One pipeline owns lint, test, build, and release.

Triggers: push to `main`/`master`/`develop`, tags matching `v*`, pull requests
to those branches, and manual dispatch.

| Job | What it does |
|---|---|
| `tag-consistency` | On a `v*` tag, fails unless the tag equals `v$(cat VERSION)` |
| `lint` | golangci-lint v1.64.8, `go vet`, `govulncheck`, gitleaks, shellcheck, version-marker check, `scripts/audit.sh` |
| `test` | Per-package `go test -race`, coverage profile, fuzz smoke for two builder targets, coverage artifact |
| `build` | linux/amd64 and linux/arm64 static binaries with `.sha256` sidecars |
| `release` | Downloads artifacts, builds `SHA256SUMS.txt`, runs goreleaser on tags, builds the AppImage (amd64), signs with cosign only if `COSIGN_PRIVATE_KEY` is set, publishes the GitHub Release, generates an SPDX SBOM, uploads a full artifact bundle |

Notes:

- `go test -race` requires CGO; the workflow sets `CGO_ENABLED=1` explicitly.
- Fuzz smoke is advisory (`continue-on-error: true`) because shared runners
  occasionally hit coordinator timeouts.
- `scripts/audit.sh` runs without `--strict` so the tracked WARN-tier
  design-debt items do not mask real FAIL findings.
- Release `files:` uses explicit globs rather than `dist/*` to avoid uploading
  goreleaser bookkeeping directories.
- `continue-on-error: true` on the publish step means a GitHub API hiccup does
  not lose the run; the artifact bundle is the fallback.

### `.github/workflows/e2e.yml`

Privileged Linux smoke test on Ubuntu:

1. `cardinal doctor`;
2. `cardinal pull alpine:latest`;
3. `cardinal run --rm --network none` and assert the marker line;
4. duplicate host port is rejected with an "already mapped" hint;
5. occupied host port warns but proceeds;
6. bridge port forwarding (advisory, `continue-on-error: true`).

It installs `iproute2`, `iptables`, `util-linux`, and `mount`, builds the
binary, and uploads a diagnostic bundle on every outcome.

### `.github/workflows/scheduled.yml`

Daily `govulncheck`, weekly full-history gitleaks, and daily
`scripts/audit.sh`. Read-only; fixes arrive as reviewed PRs.

### `.github/workflows/branch-guard.yml`

Advisory branch-model check. A PR to `main` must come from `develop`,
`hotfix/*`, or `dependabot/*`. A PR from `main` to `develop` is rejected.
It does not push or rewrite history.

## Release artifacts

Goreleaser (`.goreleaser.yaml`, version 2) builds linux/amd64 and linux/arm64
and produces:

- raw binaries: `cardinal-linux-<arch>`;
- tar.gz archives;
- `.deb`, `.rpm`, `.pkg.tar.zst` (Arch), `.apk` (Alpine);
- `cardinal_<version>_checksums.txt`;
- an AppImage for amd64, built directly in `build.yml`;
- `SHA256SUMS.txt`, merging pipeline and goreleaser checksums;
- an SPDX-JSON SBOM, attached after the release exists;
- optional cosign signature and certificate over `SHA256SUMS.txt`.

The release job runs goreleaser with `--skip=publish --skip=announce` so
`softprops/action-gh-release` remains the single publisher.

## Version synchronization

`scripts/sync-docs-version.sh` scans every `*.md` outside `.git`, `vendor`, and
`node_modules`, excluding basenames `README.md` and `CHANGELOG.md`.

- Update mode: add or refresh the `cardinal-version` marker block at the top of
  each file.
- Check mode: fail if a marker is missing, duplicated, out of order, or stale.
- README: also checks `cardinal-version-badge` and `cardinal-release`.
- CHANGELOG: also checks `cardinal-current-release`.

The canonical block is the one at the top of this file. It is generated from
`VERSION`; it is not reproduced here as a code sample because the check counts
every literal marker occurrence in a file.

Do not hand-edit the text between the markers. Run
`sh scripts/sync-docs-version.sh` instead.

Because `README.md` is excluded from the generic scan, its top marker block is
not checked by `--check`; the badge and release block are checked by dedicated
functions. The current `README.md`, `docs/README.md`, and the three
`contrib/*/README.md` marker blocks match `2.4.11`, but they remain a manual
sync point. See `D-08` in [`known-issues.md`](../status/known-issues.md).

## Static audit

`scripts/audit.sh` walks nine areas: secrets, supply chain, tests, API security,
runtime hardening, installers, CI/CD, manifests, and documentation. Exit codes:

- `0` — no FAIL (WARN allowed);
- `1` — at least one FAIL;
- `2` — at least one WARN under `--strict`.

Current state at `2.4.11`: `FAIL: 0`, `WARN: 5` — docs password placeholder,
`install.ps1`, AppImage verification, stale coverage, and manual `SYS_MODULE`
review.

## Local build and verification

### Linux

```sh
gofmt -l $(git ls-files '*.go')
go test ./... -count=1
go vet ./...
golangci-lint run ./...
sh scripts/audit.sh
sh scripts/sync-docs-version.sh --check
python3 scripts/check-docs-links.py
```

### Windows cross-compilation

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build ./...
go vet ./...
```

Verified in this repository on Windows: both commands exit 0. This proves the
code compiles for Linux; it does **not** prove runtime behavior, seccomp,
namespaces, overlayfs, cgroups, or networking.

`bash` is not on `PATH` on this Windows host, so `make audit` fails at its
`bash scripts/audit.sh` recipe even though `make` itself is available. Run
`sh scripts/audit.sh` directly, or use a Linux host, WSL, or Git Bash with
`bash` available. `make docs` and `make docs-check` call `sh` and do work on
this host. `sh` was used for `scripts/sync-docs-version.sh --check` and
`scripts/audit.sh`.

## Before tagging a release

1. Bump `VERSION`.
2. Run `sh scripts/sync-docs-version.sh`.
3. Update the CHANGELOG narrative if the release needs it.
4. Run the Linux gates (`go test`, `go vet`, `golangci-lint`, `audit.sh`).
5. Tag `vX.Y.Z` so `tag-consistency` passes.
6. Confirm the release contains binaries, packages, `SHA256SUMS.txt`, and the
   SBOM; confirm cosign files exist if signing is configured.
