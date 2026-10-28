<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# Security architecture

This document describes the controls that exist in the code today, where they
are enforced, and what remains a residual risk. It is a map of the current
implementation, not a claim that `cardinal` is a security boundary equivalent
to a virtual machine.

## Threat model in one paragraph

A container is assumed to run untrusted code. The runtime tries to prevent that
code from reaching host files, host devices, kernel interfaces, sibling
containers, or the control API without an explicit operator decision.
`cardinal` usually runs as root, so a kernel or runtime bug can still be a host
compromise. The design reduces reachable attack surface; it does not eliminate
the shared-kernel risk.

## Enforcement points

| Layer | Enforced in | Main effect |
|---|---|---|
| CLI/API input policy | `internal/container/security_policy.go` | Dangerous capabilities and explicit root users need acknowledgement |
| Start-time validation | `internal/container/start.go` | Unsupported security profiles fail closed |
| Mount/path policy | `internal/container/hostpath.go`, `volumes.go` | Protected host paths are refused |
| Namespace init | `internal/container/init.go` | Capability, sysctl, mount, `no_new_privs`, seccomp ordering |
| Seccomp | `internal/container/seccomp.go` | cBPF deny-list installed before `execve` |
| Image/archive intake | `internal/image`, `cmd/backup.go` | Traversal, symlink, device, size limits |
| API gate | `internal/api/server.go` | Bearer token required on every bind |
| Cluster RPC | `internal/orchestrator/cluster.go` | Peer calls require `CARDINAL_TOKEN` |
| Registry egress | `internal/image/allowlist.go` | Strict allowlist and internal-address refusal |

## Runtime policy shared by CLI and API

`ValidateRuntimePolicy` is called by both surfaces so the API cannot bypass a
CLI-only check.

- **Dangerous capabilities.** The blocklist is `SYS_ADMIN`, `SYS_MODULE`,
  `SYS_RAWIO`, `SYS_PTRACE`, `SYS_BOOT`, `NET_ADMIN`, `NET_RAW`, `BPF`,
  `PERFMON`, `WAKE_ALARM`, plus the literal `ALL`. A request is rejected unless
  `--allow-dangerous-caps` (CLI) or `AllowDangerousCaps` (API) is set.
- **Explicit root user.** `--user 0`, `--user root`, and `--user 0:N` are
  rejected unless `--allow-root`/`AllowRoot` is set.
  **Partial:** an unset `--user` also runs as uid 0 because there is no user
  namespace by default. The gate catches explicit requests, not the default.
- **Privileged mode.** Docker's `Privileged: true` is refused by the API;
  callers must request explicit capabilities.

## Capabilities

`internal/container/init.go` starts from a Docker-like default set:

`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `FSETID`, `KILL`, `SETGID`, `SETUID`,
`SETPCAP`, `NET_BIND_SERVICE`, `NET_RAW`, `SYS_CHROOT`, `AUDIT_WRITE`,
`SETFCAP`.

- `MKNOD` is intentionally absent. Grant it explicitly when an image must create
  device nodes.
- `--cap-add ALL` expands to every known capability, but the pre-start
  dangerous-capability gate rejects `ALL` unless acknowledged.
- `--cap-drop ALL` produces an empty set.
- Effective and permitted sets are applied with `capset`; every capability not
  in the keep set is also removed from the bounding set with
  `PR_CAPBSET_DROP`, so descendants cannot regain it.
- `SETUID`/`SETGID` are kept automatically when `--user` is set.

**Residual risk:** uid 0 inside the container is uid 0 on the host unless a
user namespace is used. Capability dropping limits what uid 0 can do, but it is
not a substitute for rootless mode.

## `no_new_privs`

`PR_SET_NO_NEW_PRIVS` is always set in `InitContainer`. The persisted/API flag
is accepted for compatibility but cannot turn the protection off. Seccomp
application also sets it before `PR_SET_SECCOMP`, as the kernel requires.

## Seccomp

**Implemented:** a default seccomp profile is applied to every container start.

- `DefaultSeccompProfile()` is allow-by-default. Only the deny-list in
  `deniedSyscalls` returns `EPERM`.
- The deny-list covers mount/namespace escape primitives, kernel module and
  reboot calls, cross-process memory/trace calls, keyring calls, BPF/perf/
  userfaultfd, clock/hostname changes, swap/log/raw I/O, and
  `open_by_handle_at`/`name_to_handle_at`.
- `buildSeccompProgram` resolves names per architecture, rejects a native-arch
  mismatch, and blocks the x32 syscall range on amd64.
- A deny-by-default profile that resolves zero rules is refused rather than
  installed.
- The filter is installed last, immediately before `execve`, because the
  read-only rootfs remount before it needs `mount(2)`.
- Failure to install the filter aborts container start.
- `CARDINAL_SECCOMP=0` is the explicit escape hatch. It logs a warning and
  starts the container without syscall confinement.

**Not supported:** custom `--seccomp-profile` files. `startInternal` refuses
them with an "not enforced yet" error instead of silently ignoring the request.
Shipping and applying arbitrary profiles remains **planned**.

**Residual risk:** the default profile is a deny-list, not a complete
allow-list. It reduces known escape primitives but does not restrict every
syscall a compromised workload can reach.

## AppArmor and SELinux

**Planned / not applied.** `--apparmor-profile` is rejected at start for the
same fail-closed reason as custom seccomp profiles. An AppArmor template exists
in `internal/container/apparmor.go`, but no profile is loaded or attached.
SELinux labeling is not implemented.

## Mount and host-path policy

`validateBindSource` requires an absolute host path, resolves symlinks, and
calls `IsProtectedHostPath`. The default blocklist includes:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/media`,
  `/mnt`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/sys`, `/usr`, `/var`;
- `/var/lib/cardinal`, `/var/run/cardinal`;
- Docker, Podman, and containerd runtime sockets;
- credential directories such as `/.ssh`, `/.aws`, `/.kube`, `/.docker`,
  `/.gnupg`, `/.netrc`.

Container mount targets are cleaned and rejected if they are `/`, `.`, or
escape the container root.

`CARDINAL_ALLOWED_HOST_PATHS` is a colon-separated operator override. It is an
escape hatch, not a security improvement: every path added there becomes
mountable by any container the operator can start.

## Namespace init hardening

`InitContainer` performs, in order:

1. hostname, chdir to the merged rootfs, exec-helper preparation;
2. `pivot_root`, unmount and delete `.old_root`;
3. mount `proc`, `devtmpfs`, `sysfs`, `devpts`;
4. `PruneDevtmpfs` to a pseudo-device allowlist;
5. reject sysctls outside the `net.*` namespace-scoped allowlist;
6. remount `/proc/sys` and `/sys` read-only;
7. `ApplyDeviceRestrictions` (`/dev/shm` 64 MiB `noexec,nosuid,nodev`,
   `/dev/mqueue` `noexec,nosuid,nodev`);
8. apply capabilities, switch user, set `no_new_privs`;
9. optional read-only rootfs, ulimits, environment injection;
10. apply seccomp, then `execve`.

Device restrictions are **best effort**: failures are logged and do not abort
the start. Treat them as hardening, not a guarantee.

## Filesystem and image intake

- Image extraction rejects path traversal, unsafe links, block/char devices,
  hardlinks, NUL bytes, self-referencing symlinks, duplicate metadata, and
  exceeds entry/size caps (100k entries, 4 GiB per entry, 16 GiB total).
- Dockerfile `COPY` stays inside the build context and rejects source links,
  special files, and symlink destinations.
- Backup restore uses `SafeBackupPath` and `RejectBackupSymlinkAncestors` to
  keep archive entries inside the staging directory and to reject symlinked
  ancestors.
- Backup archives get a `.sha256` sidecar. `--encrypt` produces AES-256-GCM
  output using `CARDINAL_BACKUP_KEY`. Restore warns when the sidecar is
  missing.

## API security

`StartServerWithTLS` refuses to serve without a Bearer token on **every** bind,
including loopback. The middleware order is:

`host guard → CORS → per-IP rate limiter → Bearer auth → JSON content type → routes`

- Tokens are compared with `crypto/subtle.ConstantTimeCompare`.
- `Authorization` must be exactly `Bearer <token>`; a query parameter or raw
  token is rejected.
- CORS allows localhost origins plus `CARDINAL_CORS_ORIGINS`.
- Request bodies are capped at 8 MiB; the server sets read, write, idle, and
  header timeouts.
- `/metrics` is behind auth unless `CARDINAL_METRICS_REQUIRES_AUTH=0`.
- TLS is opt-in with `--tls-cert`/`--tls-key`.

**Residual risk:** plain HTTP is the default; a token sent over plain HTTP is
visible to anyone who can observe the network.

## Cluster security

Cluster peer RPCs read the token from `SetAPIToken` or `CARDINAL_TOKEN`.
`requireClusterToken` fails closed when neither is set. `cluster init` and
`cluster join` also wire the token into the API and orchestrator.

## Registry and egress policy

- The registry allowlist is **strict by default**. The built-in list is Docker
  Hub, `ghcr.io`, `quay.io`, and `gcr.io`; operators extend it through
  `CARDINAL_REGISTRY_ALLOWLIST`, the on-disk allowlist, or
  `CARDINAL_REGISTRY_BLOCKLIST`.
- `CARDINAL_REGISTRY_STRICT=0` disables the gate and is a deliberate downgrade.
- `checkRegistryHostNotInternal` refuses loopback, private, link-local,
  unspecified, and `.localhost` registry hosts unless
  `CARDINAL_ALLOW_INSECURE_REGISTRY=1`.
- Plain-HTTP registries require the same opt-in.
- Bridge networks drop egress to `169.254.0.0/16` in FORWARD and to
  `127.0.0.0/8` in INPUT.
- Host `127.0.0.1:<published-port>` connections are source-NATed to the bridge
  gateway in POSTROUTING, and `route_localnet` is enabled only on cardinal
  bridge interfaces so the de-NATed reply is delivered. Two INPUT guards keep
  the relaxation safe: bridge traffic with a loopback source is dropped, and
  bridge traffic to `127.0.0.0/8` is dropped unless conntrack reports
  `ESTABLISHED,RELATED`. Both exempt tracked flows because reverse NAT restores
  a published-port reply to `127.0.0.1 -> 127.0.0.1`. If a guard cannot be
  installed, `route_localnet` stays off for that bridge.
- `CARDINAL_ROUTE_LOCALNET=1` is a broader opt-in that sets
  `net.ipv4.conf.all.route_localnet=1`. It is not needed for published ports
  and widens loopback routing outside the guarded bridge path.

## Auditing and diagnostics

- `--audit-log` initializes a JSON-lines audit log under
  `<data-dir>/audit/audit-YYYY-MM-DD.log`.
- `cardinal doctor` and `cardinal security check` are read-only diagnostics;
  `--strict` turns warnings into a non-zero exit.

## What is not protected

- A kernel, namespace, overlay, cgroup, or runtime bug can still compromise the
  host.
- No user namespace by default; default containers run as host uid 0.
- No device cgroup allowlist, hence the deliberate `MKNOD` exclusion.
- No custom seccomp profiles, AppArmor enforcement, or SELinux labeling.
- `CARDINAL_SECCOMP=0`, `CARDINAL_REGISTRY_STRICT=0`, and
  `CARDINAL_ALLOWED_HOST_PATHS` are operator-controlled downgrades.
- Secret management is environment-based; there is no vault integration.

## Verification

| Check | Command |
|---|---|
| Capability/policy unit tests | `go test ./internal/container/ -run 'Policy|Dangerous|Cap' -count=1` on Linux |
| Seccomp unit tests | `go test ./internal/container/ -run Seccomp -count=1` on Linux |
| API auth/TLS tests | `go test ./internal/api/ -count=1` on Linux |
| Host-path tests | `go test ./internal/container/ -run HostPath -count=1` on Linux |
| Static audit | `sh scripts/audit.sh` |

Cross-compilation on Windows proves the code builds; it does not prove these
controls work on a Linux kernel.
