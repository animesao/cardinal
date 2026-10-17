<!-- cardinal-version:start -->
**Documentation version:** `2.4.5`
**Project release:** `v2.4.5`
<!-- cardinal-version:end -->

<p align="center">
  <img src="img/cardinal.png" alt="cardinal logo" width="120">
</p>

# Security

## Security model

`cardinal` is a Linux container runtime, not a complete VM boundary. It uses Linux namespaces, OverlayFS, cgroups, capability dropping, `no_new_privs`, restricted sysctls, protected bind mounts, and archive path validation to reduce the impact of untrusted workloads.

Run `cardinal` as a dedicated non-root account where possible. Running a container runtime as `root` means a kernel, namespace, mount, or runtime vulnerability can affect the host. Do not treat `chroot`, namespaces, or `cardinal build` as a substitute for a VM when processing hostile code.

## Current hardening

- Container commands entered through `exec`, `cp`, `top`, and healthchecks use `nsenter -r` and verify persisted mount, PID, network, IPC, and UTS namespace identities.
- Container initialization keeps a Docker-compatible safe capability set by default (`MKNOD` excluded, unlike Docker — grant it explicitly with `--cap-add MKNOD` when the workload must create device nodes), drops dangerous capabilities such as `SYS_ADMIN` and `SYS_MODULE`, honors explicit `--cap-add`/`--cap-drop` settings, enables `no_new_privs`, and restricts container sysctls to network-namespace `net.*` names. Use `--cap-drop ALL` for a fully empty capability set.
- **Dangerous-capability gate**: `SYS_ADMIN|SYS_MODULE|SYS_RAWIO|SYS_PTRACE|SYS_BOOT|NET_ADMIN|NET_RAW|BPF|PERFMON|WAKE_ALARM` (and `ALL`) are refused unless acknowledged — CLI via `--allow-dangerous-caps`, API via `AllowDangerousCaps: true` in `HostConfig`. The check lives in the runtime layer, so the CLI and the API enforce the same policy. Docker's `Privileged: true` is refused outright by the API — request explicit caps instead.
- **Root-user gate**: `--user 0|root|0:N` (CLI `--allow-root`, API `AllowRoot: true`) is required for explicit root users. Note the default (no `--user`) also runs as uid 0 — there is no user namespace yet — so the gate catches explicit requests, not the default.
- **Seccomp/AppArmor status**: JSON profiles and a default profile ship in-tree and `--seccomp-profile` / `--apparmor-profile` overrides are accepted, but the filters are **not yet applied** to started containers. Do not rely on them until wired; current syscall confinement is `no_new_privs` + capability drops + read-only `/proc/sys` and `/sys`.
- **Device restrictions**: container `/dev` is pruned to pseudo-devices (`null`, `zero`, `full`, `random`, `urandom`, `tty`, `console`, `ptmx`, `pts`, `shm`, stdio links); `/dev/shm` is mounted with `noexec,nosuid,nodev` and a 64MB size limit; `/dev/mqueue` is mounted with `noexec,nosuid,nodev`; `/proc/sys` and `/sys` are bind-mounted read-only.
- **Network segmentation** (`--isolated`) blocks inter-container traffic via iptables, preventing lateral movement between containers. Egress to `127.0.0.0/8` and `169.254.0.0/16` (host loopback, cloud metadata) is dropped from the bridge by default; `route_localnet` stays off unless `CARDINAL_ROUTE_LOCALNET=1` is set.
- **Audit logging** records container lifecycle events (start, stop, exec, backup, etc.) with timestamps, user info, and success/failure status. Enable with `--audit-log`. Operations that escalate privileges (`--allow-dangerous-caps`) are automatically flagged in the audit log.
- Bind mounts and container targets are validated against traversal, protected host paths, and symlink escapes. Use named volumes or a dedicated `/data/...` directory for application data. Mounts into `~/.ssh`, `~/.aws`, `~/.kube`, `~/.docker`, `~/.gnupg`, `~/.netrc`, `/var/run/docker.sock`, `/var/run/podman`, or `/var/run/containerd` are blocked by `IsProtectedHostPath()`; whitelist extra paths via `CARDINAL_ALLOWED_HOST_PATHS=/path1:/path2`.
- Image layers reject traversal, unsafe links, special device entries (block/char devices), hardlinks, NUL byte paths, self-referencing symlinks, duplicate metadata, and excessive entry/total sizes (100k entries / 4G per entry / 16G total, same caps as `Import`). Registry token endpoints are pinned to the registry host (Docker Hub split-host allowlisted); pulls/pushes honor `CARDINAL_REGISTRY_STRICT` allowlist. `cmd.Update` verifies the SHA-256 of downloaded binaries; setting `CARDINAL_REQUIRE_SIGNATURE=1` also requires a cosign signature over the per-binary `.sha256` file.
- Dockerfile `COPY` stays inside the build context, rejects source links/special files and symlink destinations, and does not inherit arbitrary host environment variables into `RUN` or ARG substitution.
- The API refuses to start without a Bearer token on any bind (loopback included), limits request bodies, applies timeouts, only allows localhost CORS origins, can serve HTTPS with `--tls-cert`/`--tls-key`, applies a per-IP token-bucket rate limiter (loopback exempt), and gates `/metrics` behind the bearer token unless `CARDINAL_METRICS_REQUIRES_AUTH=0` is set. Cluster join/schedule RPCs fail closed without `CARDINAL_TOKEN`.
- **Registry allowlist**: `CARDINAL_REGISTRY_STRICT=1` plus `cardinal registry allowlist add <host>` (or `CARDINAL_REGISTRY_ALLOWLIST=` for one-off setups) refuses pulls / pushes from unapproved registries. `CARDINAL_ALLOW_INSECURE_REGISTRY=1` is required to talk to plain-HTTP registries.
- **Log-injection hardening**: every JSON log line is produced with `json.Marshal` (proper escaping of `\n`, `\r`, `\t`, and quote characters); the text-mode logger substitutes newlines with U+2028 to keep the human-readable stream unambiguous.
- `cardinal doctor` and `cardinal security check` provide read-only host/runtime diagnostics; use `--strict` in deployment checks to fail on warnings as well as errors.
- Backup archives have SHA-256 sidecar checksums and optional **AES-256-GCM encryption** (`--encrypt` or `-e` flag). Restore warns loudly when no checksum sidecar is present. Verify backups before restore and remember that host bind mounts are not included. Blueprint repositories must use `https://` (plain `http://` is refused at add and fetch time).
- Supply chain: every release publishes `SHA256SUMS.txt`, an SPDX-JSON SBOM, and (when `COSIGN_PRIVATE_KEY` is configured) detached cosign signatures over the binaries and the SHA256SUMS file. `install.sh`, `scripts/install-apt.sh` and the `AppImage` flow all verify the published digest before installing.
- `cardinal verify` checks local consistency only (config + layer hashes vs the stored manifest) — it does not prove registry provenance or tag immutability. There is no signature verification yet.

## Important limitations

There are still unavoidable kernel and race-condition limits around PID lookup, namespace entry, mount operations, and concurrent local filesystem mutation. A local privileged attacker can race filesystem checks or exploit the host kernel. For hostile multi-tenant builds or workloads, use a VM, a dedicated host, or an independently maintained sandbox such as a rootless OCI runtime with seccomp/AppArmor/SELinux policy.

The Linux namespace and mount paths must be tested on the target distribution and kernel. CI compile/lint checks do not prove that the host grants every required namespace, OverlayFS, cgroup, or networking capability.

## File transfer into and out of containers

cardinal deliberately does **not** ship an FTP server inside its runtime
binary. A previous code path (`internal/ftp/`) implemented a plain-TCP
FTP server with a single shared password, no TLS and no per-user
accounts; it was reachable from any import but lacked documentation,
tests and a CLI surface, so it has been removed outright rather than
patched.

For host↔container file transfer in production use one of the following
alternatives instead — these are explicitly listed because each protects
against the misconfigurations the removed FTP code enabled:

1. `cardinal cp <src> <container>:<dst>` — local-cp, no network exposure, no
   new listening port. Preferred for one-off transfers.
2. SSH/SFTP inside the container — run `openssh-server` (or a
   `scponly`/`chroot` setup) as the entrypoint, publish it via a regular
   `cardinal run -p 22:22`, and require key-based authentication. SFTP
   inherits SSH's KEX and channel encryption, plus host-key pinning.
3. `cardinal volume` mounts for steady-state data exchange.
4. `cardinal console <container>` for interactive read/write of files inside
   a running container (web terminal, plain-text — only over loopback
   or behind the same TLS reverse proxy that fronts the API).

We will **not** add FTP, TFTP or any other cleartext bulk-transfer
protocol back to the runtime, and we recommend operators reject
vendor-specific "FTP passthrough" features in their cluster stack.

## Safe deployment checklist

1. **Pin versions**: install via the SHA256-verified `install.sh` or via the
   signed APT repository (`deb822` `Signed-By=`). Verify checksums with
   `cardinal verify IMAGE[:TAG]`.
2. **Rootless**: prefer rootless mode with a dedicated service account and
   configure cgroup v2 quotas after creation.
3. **API surface**: the API refuses to start without a token on any bind —
   always set `--token`/`CARDINAL_TOKEN` (loopback included). Keep the API
   on `127.0.0.1` unless remote access is required, and pass `--tls-cert`
   / `--tls-key` whenever the bind is `0.0.0.0`.
4. **Filesystem discipline**: do not mount `/`, `/etc`, `/var/run`, `/proc`,
   `/sys`, `/dev`, sibling container runtimes' sockets, or credential
   directories into untrusted containers. The default blocklist already
   covers these; opt in to additional traffic through `CARDINAL_ALLOWED_HOST_PATHS`.
5. **Use `:ro`** for configuration and certificate mounts where write
   access is unnecessary.
6. **Resource limits**: set memory, CPU, disk, PID/ulimit, restart-loop, and
   backup-retention limits (`--memory`, `--cpus`, `--disk`,
   `--restart-max-attempts`, `--restart-window`).
7. **Image provenance**: maintain an explicit registry allowlist. Run the
   cluster with `CARDINAL_REGISTRY_STRICT=1` and `cardinal registry allowlist add …`.
8. **Dockerfile review**: review every `cardinal build` invocation; do not build
   untrusted Dockerfiles directly on a production host.
9. **Privileged containers**: avoid `cap-add=` with `SYS_ADMIN`, `SYS_MODULE`,
   `BPF`, `NET_ADMIN`, etc. If absolutely necessary, prefix your automation
   with `--allow-dangerous-caps` **and** log the rationale in the change
   ticket; the operation is recorded in the audit log. The same acknowledgement
   is required over the API (`AllowDangerousCaps: true`, `AllowRoot: true` in
   `HostConfig`); `Privileged: true` is refused outright.
10. **Updating**: prefer `cardinal update` together with `CARDINAL_REQUIRE_SIGNATURE=1`
    on production hosts so cosign verification is enforced.
11. **Secrets**: prefer compose `secrets:` file mounts (`/run/secrets/<name>`)
     or an external secret manager; do not commit `.env` files or API tokens.
12. **Disaster recovery**: test restoration on a separate stopped container
    at least once per quarter; rotate the encryption key alongside node
    upgrades.

## Compromised credentials in git history

> **Action required on any host that ran an old `update-vps.sh`.**

Commit `9030fd8` (v2.3.x, before v2.3.13) shipped a hardcoded `JWT_SECRET`
and a hardcoded `ADMIN_PASSWORD` straight into the systemd unit heredoc in
`update-vps.sh`, so every host that ran that script ended up with an
identical, publicly known pair of credentials.

The values are deliberately **not** reproduced here. This file is covered by
the gitleaks scan, and quoting them verbatim would either re-trip the detector
or force a blanket allowlist entry for the whole file — which would then hide
any *real* secret ever added below. Retrieve them yourself if you need to audit
a host:

```bash
git show 9030fd8 -- update-vps.sh | grep -E 'Environment=(JWT_SECRET|ADMIN_PASSWORD)'
```

These were never placeholders. They were literal defaults, identical on every
host that ran that script, and recoverable from the public git history even
though the script itself was fixed in v2.3.13.

Consequences for any VPS that ran that script:

- **`JWT_SECRET`** — anyone holding this value can mint a valid session token
  and authenticate as any user, including admin, against the panel API.
- **`ADMIN_PASSWORD`** — the initial admin login is publicly known.

Rotate both on each affected host:

```bash
UNIT=/etc/systemd/system/cardinal-panel.service

sudo sed -i \
  -e "s|^Environment=JWT_SECRET=.*|Environment=JWT_SECRET=$(openssl rand -hex 32)|" \
  -e "s|^Environment=ADMIN_PASSWORD=.*|Environment=ADMIN_PASSWORD=$(openssl rand -hex 16)|" \
  "$UNIT"

sudo systemctl daemon-reload && sudo systemctl restart cardinal-panel
```

Rotating `JWT_SECRET` invalidates every existing session token, so all users
sign in again. No data is lost.

Current releases are not affected: since v2.3.13 `update-vps.sh` reuses the
unit's existing values when present and otherwise generates them with
`openssl rand`, so no host ever shares a secret again.

The weekly `gitleaks` scan still sees this historical commit. `.gitleaks.toml`
suppresses it through an allowlist scoped to that single SHA. That allowlist
only silences the scan — it does not make the credentials secret again, and it
does not suppress findings in any other commit.


## Reporting a vulnerability

Please do not publish an exploitable proof of concept before a fix is available. Open a private GitHub security advisory for the repository when possible. Otherwise contact the maintainer through the GitHub profile linked in `CONTRIBUTING.md` and include:

- affected version/commit and Linux distribution/kernel;
- reproduction steps and required privileges;
- impact and realistic attack scenario;
- a minimal non-destructive proof of concept;
- any proposed mitigation.

We will acknowledge reports, coordinate a fix, and credit researchers who want attribution.
