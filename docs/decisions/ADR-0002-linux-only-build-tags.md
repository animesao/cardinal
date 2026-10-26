<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# ADR-0002: Linux-only runtime behind build tags

- **Status:** Accepted
- **Date:** 2026-10-26
- **Scope:** Portability, build matrix, developer workflow

## Context

Namespaces, `pivot_root`, overlayfs, cgroups, seccomp, and capability syscalls
are Linux-specific. Pretending to support another OS at runtime would mean
stubbing the core of the product and shipping a binary that cannot do its job.
At the same time, editing and cross-compiling on Windows or macOS is useful.

## Decision

Keep the runtime Linux-only and make that explicit at the file level with
`//go:build linux`. A non-Linux checkout is for editing and cross-compilation,
not container execution. The release matrix builds `GOOS=linux` for `amd64` and
`arm64` with `CGO_ENABLED=0`.

## Evidence

- Most runtime files start with `//go:build linux`, including
  `internal/container/start.go`, `init.go`, `seccomp.go`, `security_policy.go`,
  `hostpath.go`, and `internal/api/server.go`.
- `Makefile` only targets `GOOS=linux`.
- `.goreleaser.yaml` sets `goos: [linux]` and `goarch: [amd64, arm64]`.
- `main.go` and `cmd/cobra.go` also carry the Linux build tag, so a non-Linux
  `go build` of the module has no entry point.

## Consequences

Positive:

- no misleading "best effort" support on platforms that cannot run containers;
- the compiler enforces the boundary instead of runtime checks;
- Linux-only syscalls and structs can be used without portability shims;
- cross-compilation from Windows/macOS still works.

Negative:

- `go test ./...` on Windows does not exercise the runtime;
- a Windows/macOS contributor cannot run e2e locally without a Linux VM/WSL;
- adding a portable helper requires a separate untagged package or a stub with
  a matching signature.

## Verification rule

On a non-Linux host, the following is a compile check only:

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build ./...
go vet ./...
```

It must never be reported as runtime verification. Runtime claims require a
Linux kernel and, for privileged behavior, root.

## Alternatives rejected

- **Runtime OS checks with stubs.** Rejected because it hides missing behavior
  and produces binaries that appear to work.
- **Darwin/Windows container backends.** Rejected as out of scope for a
  namespace/overlay/cgroup runtime.
- **Build tags on only a few files.** Rejected because partial tagging produces
  confusing compile errors and half-supported packages.

## Related

- [`../architecture/build-and-release.md`](../architecture/build-and-release.md)
- [`../architecture/runtime.md`](../architecture/runtime.md)
