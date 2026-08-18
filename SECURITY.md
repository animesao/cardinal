<!-- dck-version:start -->
**Documentation version:** `1.60.5`
**Project release:** `v1.60.5`
<!-- dck-version:end -->

<p align="center">
  <img src="img/dck.png" alt="dck logo" width="120">
</p>

# Security

## Security model

`dck` is a Linux container runtime, not a complete VM boundary. It uses Linux namespaces, OverlayFS, cgroups, capability dropping, `no_new_privs`, restricted sysctls, protected bind mounts, and archive path validation to reduce the impact of untrusted workloads.

Run `dck` as a dedicated non-root account where possible. Running a container runtime as `root` means a kernel, namespace, mount, or runtime vulnerability can affect the host. Do not treat `chroot`, namespaces, or `dck build` as a substitute for a VM when processing hostile code.

## Current hardening

- Container commands entered through `exec`, `cp`, `top`, and healthchecks use `nsenter -r` and verify persisted mount, PID, network, IPC, and UTS namespace identities.
- Container initialization keeps a Docker-compatible safe capability set by default, drops dangerous capabilities such as `SYS_ADMIN` and `SYS_MODULE`, honors explicit `--cap-add`/`--cap-drop` settings, enables `no_new_privs`, and restricts container sysctls to network-namespace `net.*` names. Use `--cap-drop ALL` for a fully empty capability set.
- **Dangerous-capability gate**: `dck run --cap-add SYS_ADMIN|SYS_MODULE|SYS_RAWIO|SYS_PTRACE|SYS_BOOT|NET_ADMIN|NET_RAW|BPF|PERFMON|WAKE_ALARM` is refused unless the user passes `--allow-dangerous-caps`. This protects against silent escalations in unattended scripts.
- **Root-user gate**: `--user 0|root|0:N` is refused unless `--allow-root` is supplied. Container processes should not run as UID 0 by default.
- **Seccomp filtering** blocks 30+ dangerous syscalls (mount, ptrace, reboot, kexec_load, bpf, etc.) by default. Use `--seccomp-profile` to provide a custom JSON profile.
- **AppArmor profile** (`dck-container`) restricts access to sensitive host paths (`/proc/sys`, `/sys/firmware`), denies device access (`/dev/mem`, `/dev/kmem`), and limits ptrace to within the container. Use `--apparmor-profile` to override.
- **Device restrictions**: `/dev/shm` is mounted with `noexec,nosuid,nodev` and a 64MB size limit; `/dev/mqueue` is mounted with `noexec,nosuid,nodev`; `/proc/sys` and `/sys` are bind-mounted read-only; sensitive devices (`/dev/mem`, `/dev/kmem`, `/dev/sda*`) are removed from the container.
- **Network segmentation** (`--isolated`) blocks inter-container traffic via iptables, preventing lateral movement between containers.
- **Audit logging** records container lifecycle events (start, stop, exec, backup, etc.) with timestamps, user info, and success/failure status. Enable with `--audit-log`. Operations that escalate privileges (`--allow-dangerous-caps`) are automatically flagged in the audit log.
- Bind mounts and container targets are validated against traversal, protected host paths, and symlink escapes. Use named volumes or a dedicated `/data/...` directory for application data. Mounts into `~/.ssh`, `~/.aws`, `~/.kube`, `~/.docker`, `~/.gnupg`, `~/.netrc`, `/var/run/docker.sock`, `/var/run/podman`, or `/var/run/containerd` are blocked by `IsProtectedHostPath()`; whitelist extra paths via `DCK_ALLOWED_HOST_PATHS=/path1:/path2`.
- Image layers and imported archives reject traversal, unsafe links, special device entries (block/char devices), hardlinks, NUL byte paths, self-referencing symlinks, duplicate metadata, and excessive entry/total sizes. `cmd.Update` verifies the SHA-256 of downloaded binaries; setting `DCK_REQUIRE_SIGNATURE=1` also requires a cosign signature over the per-binary `.sha256` file.
- Dockerfile `COPY` stays inside the build context, rejects source links/special files and symlink destinations, and does not inherit arbitrary host environment variables into `RUN` or ARG substitution.
- The API refuses external binds without a Bearer token, limits request bodies, applies timeouts, only allows localhost CORS origins, can serve HTTPS with `--tls-cert`/`--tls-key`, applies a per-IP token-bucket rate limiter (loopback exempt), and gates `/metrics` behind the bearer token unless `DCK_METRICS_REQUIRES_AUTH=0` is set.
- **Registry allowlist**: `DCK_REGISTRY_STRICT=1` plus `dck registry allowlist add <host>` (or `DCK_REGISTRY_ALLOWLIST=` for one-off setups) refuses pulls / pushes from unapproved registries. `DCK_ALLOW_INSECURE_REGISTRY=1` is required to talk to plain-HTTP registries.
- **Log-injection hardening**: every JSON log line is produced with `json.Marshal` (proper escaping of `\n`, `\r`, `\t`, and quote characters); the text-mode logger substitutes newlines with U+2028 to keep the human-readable stream unambiguous.
- `dck doctor` and `dck security check` provide read-only host/runtime diagnostics; use `--strict` in deployment checks to fail on warnings as well as errors.
- Backup archives have SHA-256 sidecar checksums and optional **AES-256-GCM encryption** (`--encrypt` or `-e` flag). Verify backups before restore and remember that host bind mounts are not included.
- Supply chain: every release publishes `SHA256SUMS.txt`, an SPDX-JSON SBOM, and (when `COSIGN_PRIVATE_KEY` is configured) detached cosign signatures over the binaries and the SHA256SUMS file. `install.sh`, `scripts/install-apt.sh` and the `AppImage` flow all verify the published digest before installing.

## Important limitations

There are still unavoidable kernel and race-condition limits around PID lookup, namespace entry, mount operations, and concurrent local filesystem mutation. A local privileged attacker can race filesystem checks or exploit the host kernel. For hostile multi-tenant builds or workloads, use a VM, a dedicated host, or an independently maintained sandbox such as a rootless OCI runtime with seccomp/AppArmor/SELinux policy.

The Linux namespace and mount paths must be tested on the target distribution and kernel. CI compile/lint checks do not prove that the host grants every required namespace, OverlayFS, cgroup, or networking capability.

## File transfer into and out of containers

dck deliberately does **not** ship an FTP server inside its runtime
binary. A previous code path (`internal/ftp/`) implemented a plain-TCP
FTP server with a single shared password, no TLS and no per-user
accounts; it was reachable from any import but lacked documentation,
tests and a CLI surface, so it has been removed outright rather than
patched.

For host↔container file transfer in production use one of the following
alternatives instead — these are explicitly listed because each protects
against the misconfigurations the removed FTP code enabled:

1. `dck cp <src> <container>:<dst>` — local-cp, no network exposure, no
   new listening port. Preferred for one-off transfers.
2. SSH/SFTP inside the container — run `openssh-server` (or a
   `scponly`/`chroot` setup) as the entrypoint, publish it via a regular
   `dck run -p 22:22`, and require key-based authentication. SFTP
   inherits SSH's KEX and channel encryption, plus host-key pinning.
3. `dck volume` mounts for steady-state data exchange.
4. `dck console <container>` for interactive read/write of files inside
   a running container (web terminal, plain-text — only over loopback
   or behind the same TLS reverse proxy that fronts the API).

We will **not** add FTP, TFTP or any other cleartext bulk-transfer
protocol back to the runtime, and we recommend operators reject
vendor-specific "FTP passthrough" features in their cluster stack.

## Safe deployment checklist

1. **Pin versions**: install via the SHA256-verified `install.sh` or via the
   signed APT repository (`deb822` `Signed-By=`). Verify checksums with
   `dck verify IMAGE[:TAG]`.
2. **Rootless**: prefer rootless mode with a dedicated service account and
   configure cgroup v2 quotas after creation.
3. **API surface**: keep the API on `127.0.0.1` unless remote access is
   required. Set `DCK_TOKEN` for any external bind, and pass `--tls-cert`
   / `--tls-key` whenever the bind is `0.0.0.0`.
4. **Filesystem discipline**: do not mount `/`, `/etc`, `/var/run`, `/proc`,
   `/sys`, `/dev`, sibling container runtimes' sockets, or credential
   directories into untrusted containers. The default blocklist already
   covers these; opt in to additional traffic through `DCK_ALLOWED_HOST_PATHS`.
5. **Use `:ro`** for configuration and certificate mounts where write
   access is unnecessary.
6. **Resource limits**: set memory, CPU, disk, PID/ulimit, restart-loop, and
   backup-retention limits (`--memory`, `--cpus`, `--disk`,
   `--restart-max-attempts`, `--restart-window`).
7. **Image provenance**: maintain an explicit registry allowlist. Run the
   cluster with `DCK_REGISTRY_STRICT=1` and `dck registry allowlist add …`.
8. **Dockerfile review**: review every `dck build` invocation; do not build
   untrusted Dockerfiles directly on a production host.
9. **Privileged containers**: avoid `cap-add=` with `SYS_ADMIN`, `SYS_MODULE`,
   `BPF`, `NET_ADMIN`, etc. If absolutely necessary, prefix your automation
   with `--allow-dangerous-caps` **and** log the rationale in the change
   ticket; the operation is recorded in the audit log.
10. **Updating**: prefer `dck update` together with `DCK_REQUIRE_SIGNATURE=1`
    on production hosts so cosign verification is enforced.
11. **Secrets**: prefer `--secret` mounts (`dck secret`) or an external
    secret manager; do not commit `.env` files or API tokens.
12. **Disaster recovery**: test restoration on a separate stopped container
    at least once per quarter; rotate the encryption key alongside node
    upgrades.


## Reporting a vulnerability

Please do not publish an exploitable proof of concept before a fix is available. Open a private GitHub security advisory for the repository when possible. Otherwise contact the maintainer through the GitHub profile linked in `CONTRIBUTING.md` and include:

- affected version/commit and Linux distribution/kernel;
- reproduction steps and required privileges;
- impact and realistic attack scenario;
- a minimal non-destructive proof of concept;
- any proposed mitigation.

We will acknowledge reports, coordinate a fix, and credit researchers who want attribution.
