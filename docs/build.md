<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# Build, CI & Versioning

## Requirements

- Go 1.26+ (the module targets `go 1.26.0`; CI and release tooling pin `1.26.10`)
- Linux runtime checks: `cardinal doctor` and `cardinal security check`
- Linux for container execution features
- Optional: `git` for version injection and release metadata

## Quick build

Build a static Linux binary without glibc dependencies:

```bash
CGO_ENABLED=0 go build -tags netgo -ldflags="-s -w" -o cardinal .
```

The resulting binary is suitable for Linux hosts with the required namespace,
mount, networking, and OverlayFS support.

## Cross-compile

Cross-compilation verifies another architecture, but the resulting binary must
not be executed on the host unless the host has a compatible CPU or emulator.

```bash
# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -ldflags="-s -w" -o cardinal-linux-amd64 .

# Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags netgo -ldflags="-s -w" -o cardinal-linux-arm64 .

# Linux armv6
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -tags netgo -ldflags="-s -w" -o cardinal-linux-armv6 .
```

The Makefile builds the regular Linux artifacts (`amd64` and `arm64`):

```bash
make build
```

The release workflow builds the regular Linux `amd64` and `arm64` artifacts.
Goreleaser produces `.deb`, `.rpm`, `.pkg.tar.zst`, and `.apk` packages for
those two architectures, and the workflow builds an `amd64` AppImage. Every
release artifact has a SHA-256 checksum. Current workflows do not publish Snap
packages or ARMv6 release artifacts; ARMv6 remains a manual cross-build target.

## Tests and checks

Run the same checks locally that CI expects:

```bash
gofmt -w $(git ls-files '*.go')
go test ./... -count=1
go vet ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
git diff --check
cardinal doctor
cardinal security check --strict
```

Go tests execute compiled test binaries. Do not run `GOARCH=arm64 go test` on a
normal amd64 GitHub runner: it produces an arm64 test binary that cannot run on
that runner. CI therefore runs the full test suite on native amd64 and uses
arm64 cross-build only.

## Version injection

The root `VERSION` file is the single source of truth:

```bash
VERSION=$(tr -d ' \n' < VERSION)
go build -ldflags="-X cardinal/cmd.version=$VERSION" -o cardinal .
cardinal version
```

A build without `-X` reports the development fallback version. Edit only the
root `VERSION` file; do not create another version file under `cmd/`.

## GitHub Actions

The repository has four workflows. CI also runs `govulncheck` against the Go dependency graph; this catches known vulnerabilities in reachable Go code but does not replace OS/kernel/container-image scanning. Keep the workflow toolchain on the latest patched Go release; Go 1.26.3 was observed by the audit to contain GO-2026-5856, GO-2026-5039, and GO-2026-5037, fixed in later patch releases.



- **Build** (`.github/workflows/build.yml`) is the single main pipeline. It runs
  `tag-consistency`, lint (`golangci-lint`, `go vet`, `govulncheck`,
  `shellcheck`, docs version/link checks, `scripts/audit.sh`), tests (race,
  coverage, fuzz smoke), and builds Linux `amd64`/`arm64`. Tag pushes run
  goreleaser for the package matrix, build an amd64 AppImage, optionally sign
  checksums with cosign, attach an SBOM, and publish the GitHub release.
- **Linux E2E** (`.github/workflows/e2e.yml`) is a manual privileged Ubuntu smoke
  test. It pulls Alpine, runs a namespace-isolated command, exercises restart
  recovery, and creates/verifies a backup. It is manual because it needs host
  kernel, mount, namespace, and networking capabilities.
- **Scheduled security scan** (`.github/workflows/scheduled.yml`) runs
  `govulncheck` and `scripts/audit.sh` daily plus a full-history gitleaks scan
  weekly. It rewrites nothing; findings require a reviewed follow-up PR.
- **branch-guard** (`.github/workflows/branch-guard.yml`) is an advisory PR
  check for the branching model in `CONTRIBUTING.md`. It fails the check with an
  explanation but does not push, rewrite history, or replace branch protection.

Tag pushes require `VERSION` to match the tag (`tag-consistency`); the workflow
does not rewrite `VERSION` or create a version commit. The sync script updates
current version markers, the README release pointer, and an explicit
current-release pointer in `CHANGELOG.md` automatically; detailed release notes
remain manually maintained so historical entries are not overwritten.

## Goreleaser

Goreleaser uses the Git tag through `{{ .Version }}`. Keep the tag, root
`VERSION`, and injected `cardinal/cmd.version` aligned when publishing manually.

## Binary size

| Build mode | Approximate size |
|---|---:|
| Default static build | ~4.8 MB |
| `-ldflags="-s -w"` | ~3.9 MB |
| `-ldflags="-s -w"` + UPX | ~1.2 MB |

```bash
go build -ldflags="-s -w" -o cardinal .
upx --best cardinal
```

## Verify a build

```bash
./cardinal version
./cardinal info
```

For a Linux binary, perform the runtime test on a matching Linux host. For
cross-built arm64 artifacts, verify checksums and transfer them to the matching
target architecture before execution. The current workflow publishes an
`amd64` AppImage; use the raw binary or `.tar.gz` on other architectures.

## Update check

```bash
cardinal update --check
cardinal update
```
