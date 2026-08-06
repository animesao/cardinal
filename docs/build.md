# Build & Versioning

## Requirements

- Go 1.25+
- Linux (for container execution features)
- Optional: `git` (for version injection)

## Quick build (static binary, no glibc)

```bash
CGO_ENABLED=0 go build -tags netgo -ldflags="-s -w" -o dck .
```

Produces a fully static binary — works on any Linux regardless of glibc version.

## Cross-compile from any OS

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -ldflags="-s -w" -o dck .
```

## Version injection

Version is injected via `ldflags` from the root `VERSION` file:

```bash
VERSION=$(cat VERSION)
go build -ldflags="-X dck/cmd.version=$VERSION" -o dck .
dck version   # → dck version 1.2.3
```

The single source of truth is `VERSION` — edit only this file.

### Using Makefile

```bash
make build   # reads VERSION, injects via -X, produces dck-linux-amd64
```

### CI / goreleaser

Both CI workflows (`build.yml`, `release.yml`) read `VERSION`, bump it, and pass via `-X dck/cmd.version=<ver>`. Goreleaser uses git tag via `{{ .Version }}`.

### Dev build (without ldflags)

```go
var version = "dev"   // shown when built without -X
```

## Binary size

| Build mode | Size |
|---|---|
| Default (no cgo) | ~4.8 MB |
| `-ldflags="-s -w"` | ~3.9 MB |
| `-ldflags="-s -w"` + UPX | ~1.2 MB |

```bash
# Strip and pack
go build -ldflags="-s -w" -o dck .
upx --best dck
```

## Verify

```bash
./dck version
./dck info
```

## Update check

```bash
dck update --check   # Check latest release
dck update           # Self-update
```
