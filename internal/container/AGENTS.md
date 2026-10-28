<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# internal/container — runtime core

Read [`docs/architecture/runtime.md`](../../docs/architecture/runtime.md) and
[`docs/architecture/security.md`](../../docs/architecture/security.md) before
editing this package. The root [`AGENTS.md`](../../AGENTS.md) lists the controls
that must not be weakened.

This package is Linux-only. Every runtime file carries `//go:build linux`.

## Entry points and files

| File | Responsibility |
|---|---|
| `types.go` | `Container` model, persisted JSON, `Save`/`Load`, validation helpers, `CreateOpts` |
| `start.go` | Start policy, filesystem setup, `unshare` launch, networking handshake, console/cleanup |
| `init.go` | In-namespace PID 1: mounts, `pivot_root`, capabilities, user, `no_new_privs`, readonly rootfs, seccomp, `execve` |
| `security_policy.go` | Dangerous-capability blocklist and explicit `--allow-*` policy shared by CLI and API |
| `seccomp.go` | cBPF compiler, default deny-list, `ApplyDefaultSeccompProfile` |
| `seccomp_arch_*.go` | Per-architecture syscall numbers; do not invent numbers, use `x/sys/unix` |
| `hostpath.go` | Bind-mount blocklist and operator allowlist |
| `namespace.go`, `overlay.go`, `cgroup.go` | Namespace setup, overlayfs, cgroup limits |
| `prune.go`, `volumes.go` | Prune behavior and named-volume lifecycle |
| `backup_encryption.go`, `backup_safe.go` | Backup crypto and safe archive handling |

`cmd/cardinal init` is implemented by `initContainer` and runs as the
in-namespace init process. It is not a user-facing command.

## Contracts that must not drift

- Persisted field names in `types.go` are storage contracts. Renaming one
  silently breaks existing containers and backups.
- `Save` writes `state.ContainerPath(c.ID)` via atomic JSON replacement. Do not
  replace it with a direct `os.WriteFile` on the live state file.
- Every start that allocates resources must have a matching cleanup path
  (`cleanupNetwork`, overlay unmount, cgroup removal, console kill). Tests may
  run in sequence on one host.
- `Start` takes `lifecycleMu`; `Save` takes `dataMu`. Do not hold one while
  calling code that takes the other unless the existing path already does.
- Container JSON has no cross-process lock. The supported model is a single
  writer; see `T-01` in [`known-issues.md`](../../docs/status/known-issues.md).

## Security invariants

- `startInternal` rejects `SeccompProfile` and `AppArmorProfile` instead of
  running them unenforced. Custom seccomp and AppArmor are `Planned`; the
  rejection is intentional. Do not turn a rejection into a warning.
- `ApplyDefaultSeccompProfile()` runs last in `InitContainer`, immediately
  before `execve`. It cannot move earlier: the readonly-rootfs remount is a
  `mount(2)` call and mount is in the deny-list.
- `CARDINAL_SECCOMP=0` is the documented escape hatch and logs a warning. Do not
  add more silent bypasses.
- `no_new_privs` is always enabled. The `NoNewPrivileges` field exists for API
  compatibility and cannot weaken the runtime policy.
- Capabilities start from the Docker-like default set. The bounding set is
  reduced too, so children cannot regain dropped capabilities. Dangerous caps
  require an explicit acknowledgement through `ValidateRuntimePolicy`.
- Explicit root users require `--allow-root`, but an unset user still runs as
  uid 0 (no user namespace by default). This is recorded as `T-03`; do not
  describe containers as rootless by default.
- Bind mounts pass through `IsProtectedHostPath` with symlink resolution.
  `CARDINAL_ALLOWED_HOST_PATHS` is the only operator override.
- Rootless mode rejects user-defined bridge networks. Preserve the fail-closed
  error unless rootless networking is actually implemented and tested.

## Verification

Unit tests on Linux:

```sh
gofmt -l internal/container
go test ./internal/container/... -count=1
go vet ./internal/container/...
```

The seccomp child test needs no root, but it does need a Linux kernel with
seccomp enabled. The lifecycle e2e test is skipped unless all of these hold:

```sh
sudo env CARDINAL_E2E_IMAGE=alpine:latest \
  go test ./internal/container -run TestContainerLifecycleE2E -count=1
```

It requires root, `unshare`, `mount`, and an already-pulled image. A skipped
test is not a pass; report the skip.

On Windows, cross-compile only:

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; go vet ./...
```

Cross-compilation proves type correctness, not namespace, cgroup, overlay, or
seccomp behavior. Never claim a runtime test passed from a Windows host.

## Good first tests when changing behavior

- `capabilities_test.go`, `seccomp_test.go`, `security_policy_test.go`,
  `hostpath_test.go` for hardening.
- `types_test.go`, `lifecycle_test.go` for persistence and process handling.
- `prune_volumes_test.go`, `volumes_test.go` for state cleanup.
- `backup_encryption_test.go` for crypto and key-file permissions.

If a change alters a default capability, the seccomp deny-list, the host-path
blocklist, or the start ordering, the burden is on the change to prove that the
control still holds.
