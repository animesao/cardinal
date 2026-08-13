<!-- dck-version:start -->
**Documentation version:** `1.23.27`
**Project release:** `v1.23.27`
<!-- dck-version:end -->

# Security

## Security model

`dck` is a Linux container runtime, not a complete VM boundary. It uses Linux namespaces, OverlayFS, cgroups, capability dropping, `no_new_privs`, restricted sysctls, protected bind mounts, and archive path validation to reduce the impact of untrusted workloads.

Run `dck` as a dedicated non-root account where possible. Running a container runtime as `root` means a kernel, namespace, mount, or runtime vulnerability can affect the host. Do not treat `chroot`, namespaces, or `dck build` as a substitute for a VM when processing hostile code.

## Current hardening

- Container commands entered through `exec`, `cp`, `top`, and healthchecks use `nsenter -r` and verify persisted mount, PID, network, IPC, and UTS namespace identities.
- Container initialization drops capabilities by default, honors explicit capability allowlists, enables `no_new_privs`, and restricts container sysctls to network-namespace `net.*` names.
- Bind mounts and container targets are validated against traversal, protected host paths, and symlink escapes. Use named volumes or a dedicated `/data/...` directory for application data.
- Image layers and imported archives reject traversal, unsafe links, special device entries, duplicate metadata, and excessive entry/total sizes.
- Dockerfile `COPY` stays inside the build context, rejects source links/special files and symlink destinations, and does not inherit arbitrary host environment variables into `RUN` or ARG substitution.
- The API refuses external binds without a Bearer token, limits request bodies, applies timeouts, only allows localhost CORS origins, and can serve HTTPS with `--tls-cert`/`--tls-key`.
- `dck doctor` and `dck security check` provide read-only host/runtime diagnostics; use `--strict` in deployment checks to fail on warnings as well as errors.
- Backup archives have SHA-256 sidecar checksums. Verify backups before restore and remember that host bind mounts are not included.

## Important limitations

There are still unavoidable kernel and race-condition limits around PID lookup, namespace entry, mount operations, and concurrent local filesystem mutation. A local privileged attacker can race filesystem checks or exploit the host kernel. For hostile multi-tenant builds or workloads, use a VM, a dedicated host, or an independently maintained sandbox such as a rootless OCI runtime with seccomp/AppArmor/SELinux policy.

The Linux namespace and mount paths must be tested on the target distribution and kernel. CI compile/lint checks do not prove that the host grants every required namespace, OverlayFS, cgroup, or networking capability.

## Safe deployment checklist

1. Keep `dck` and the Linux kernel updated.
2. Prefer rootless mode and a dedicated service account.
3. Keep the API on `127.0.0.1` unless remote access is required; configure `DCK_TOKEN` for any external bind.
4. Do not mount `/`, `/etc`, `/var/run`, `/proc`, `/sys`, `/dev`, or Docker/dck state directories into untrusted containers.
5. Use `:ro` for configuration and certificate mounts where write access is unnecessary.
6. Set memory, CPU, disk, PID/ulimit, restart-loop, and backup retention limits for production containers.
7. Review Dockerfiles before `dck build`; do not build untrusted Dockerfiles directly on a production host.
8. Verify image digests and backup checksums when supply-chain integrity matters.
9. Keep secrets in dedicated secret/config mounts or an external secret manager; never commit `.env` files or tokens.
10. Test disaster recovery by restoring a backup to a separate stopped container.

## Reporting a vulnerability

Please do not publish an exploitable proof of concept before a fix is available. Open a private GitHub security advisory for the repository when possible. Otherwise contact the maintainer through the GitHub profile linked in `CONTRIBUTING.md` and include:

- affected version/commit and Linux distribution/kernel;
- reproduction steps and required privileges;
- impact and realistic attack scenario;
- a minimal non-destructive proof of concept;
- any proposed mitigation.

We will acknowledge reports, coordinate a fix, and credit researchers who want attribution.
