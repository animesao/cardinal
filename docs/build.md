# Build, CI & Versioning

## Requirements

- Go 1.25+
- Linux for container execution features
- Optional: `git` for version injection and release metadata

## Quick build

Build a static Linux binary without glibc dependencies:

```bash
CGO_ENABLED=0 go build -tags netgo -ldflags="-s -w" -o dck .
```

The resulting binary is suitable for Linux hosts with the required namespace,
mount, networking, and OverlayFS support.

## Cross-compile

Cross-compilation verifies another architecture, but the resulting binary must
not be executed on the host unless the host has a compatible CPU or emulator.

```bash
# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -ldflags="-s -w" -o dck-linux-amd64 .

# Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags netgo -ldflags="-s -w" -o dck-linux-arm64 .

# Linux armv6
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -tags netgo -ldflags="-s -w" -o dck-linux-armv6 .
```

The Makefile builds the regular Linux artifacts (`amd64` and `arm64`):

```bash
make build
```

The release workflow additionally builds an `armv6` artifact and publishes the
checksums and amd64 Debian package.

## Tests and checks

Run the same checks locally that CI expects:

```bash
gofmt -w $(git ls-files '*.go')
go test ./... -count=1
go vet ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
git diff --check
```

Go tests execute compiled test binaries. Do not run `GOARCH=arm64 go test` on a
normal amd64 GitHub runner: it produces an arm64 test binary that cannot run on
that runner. CI therefore runs the full test suite on native amd64 and uses
arm64 cross-build only.

## Version injection

The root `VERSION` file is the single source of truth:

```bash
VERSION=$(tr -d ' \n' < VERSION)
go build -ldflags="-X dck/cmd.version=$VERSION" -o dck .
dck version
```

A build without `-X` reports the development fallback version. Edit only the
root `VERSION` file; do not create another version file under `cmd/`.

## GitHub Actions

The repository has three workflows:

- **CI** (`.github/workflows/ci.yml`) runs golangci-lint and `go vet`, then builds
  Linux `amd64` and `arm64`. The full test suite runs on native amd64 only.
  The matrix has `fail-fast: false`, so an architecture failure does not cancel
  the other architecture job.
- **Build & Release** (`.github/workflows/build.yml`) runs on pushes to `main`
  and manual dispatch. It increments `VERSION`, creates a version tag, checks
  out that tag, builds Linux `amd64`, `arm64`, and `armv6`, creates checksums and
  an amd64 `.deb`, then publishes a GitHub release.
- **Release** (`.github/workflows/release.yml`) is a manual release workflow for
  major/minor/patch/automatic version bumps. Build/release workflows use the
  same serialized concurrency group so simultaneous runs do not race on
  `VERSION`, `main`, or tags.

The automated version commit uses `[skip ci]`; the release artifacts are built
from the version tag created by the same workflow.

## Goreleaser

Goreleaser uses the Git tag through `{{ .Version }}`. Keep the tag, root
`VERSION`, and injected `dck/cmd.version` aligned when publishing manually.

## Binary size

| Build mode | Approximate size |
|---|---:|
| Default static build | ~4.8 MB |
| `-ldflags="-s -w"` | ~3.9 MB |
| `-ldflags="-s -w"` + UPX | ~1.2 MB |

```bash
go build -ldflags="-s -w" -o dck .
upx --best dck
```

## Verify a build

```bash
./dck version
./dck info
```

For a Linux binary, perform the runtime test on a matching Linux host. For
cross-built arm64/armv6 artifacts, verify checksums and transfer them to the
matching target architecture before execution.

## Update check

```bash
dck update --check
dck update
```
