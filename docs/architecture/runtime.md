<!-- cardinal-version:start -->
**Documentation version:** `2.4.10`
**Project release:** `v2.4.10`
<!-- cardinal-version:end -->

# Runtime architecture

This document covers image handling, container creation and start, namespaces,
storage, cgroups, networking, lifecycle, rootless mode, and compose-style
`up`/`down`. Security policy is summarized here and detailed in
[`security.md`](security.md).

## Entry points

- `main.go` calls `cmd.Execute()`; both are Linux-only.
- `cmd.NewRoot()` builds the cobra tree and registers commands from
  `cmd/cobra_commands.go`.
- Most commands forward raw arguments to a legacy `func Command(args []string)`
  handler that parses flags with the standard library `flag` package.
- `cardinal init <id> <merged>` is the in-namespace init process. It is an
  implementation command, not a user interface.

## Images

### Pull and store

`internal/image/pull.go` implements an OCI/Docker Registry v2 client:

1. Parse `name[:tag]` (`parseRef` defaults the tag to `latest`).
2. Run the registry allowlist gate and the internal-host/SSRF check before any
   network contact.
3. Obtain a Bearer token. `allowedTokenHosts` restricts which `realm` hosts may
   issue tokens for a given registry.
4. Resolve a manifest or manifest list for the requested platform
   (`linux/amd64` by default, `--platform` supported).
5. Download the config blob and each layer, verifying the digest of every blob.
6. Cache layers under the state directory and extract them into the image
   rootfs.

Images are considered present when `image.json` exists in
`<data-dir>/images/<name>/<tag>/`. `internal/image/store.go` also reads legacy
`manifest.json` metadata for older installations. Removing an image removes its
directory tree.

`internal/image/verify.go` re-checks stored image metadata and layer digests for
`cardinal verify`. `push.go` performs the upload path and also enforces the
registry allowlist.

### Build

`internal/builder` parses a Dockerfile and applies supported instructions. The
parser handles continuations, shell-word splitting, exec-form JSON arrays,
`ARG`, `ENV`, `COPY`, `RUN`, `WORKDIR`, `USER`, `EXPOSE`, `VOLUME`, `LABEL`,
`HEALTHCHECK`, `ENTRYPOINT`, `CMD`, `SHELL`, `STOPSIGNAL`, and multi-stage
`FROM`. Build sources and destinations pass through
`internal/builder/security.go`, which rejects symlink escapes and paths outside
the build context/rootfs. Build `RUN` steps execute in the build environment,
not in the host environment (`buildEnvironment`).

## Container create and start

The authoritative path is `internal/container/start.go`
(`Start` → `startInternal`) plus `internal/container/init.go`
(`InitContainer`).

### Host-side sequence

1. Serialize lifecycle operations with `lifecycleMu`.
2. Reject a container that is already running.
3. Reject custom `SeccompProfile` and `AppArmorProfile` values. They are not
   enforced yet, so accepting them would be a silent security downgrade.
4. Release a stale IP allocation left by a host crash.
5. Ensure the state directory layout exists.
6. Reject user-defined bridge networks in rootless mode.
7. Prepare the filesystem:
   - image rootfs is the lower layer;
   - `<data-dir>/overlay/<id>/upper` and `work` are the writable layers;
   - `<data-dir>/overlay/<id>/merged` is the mount point;
   - if `--disk` is set, a loop-backed ext4 image is created/mounted at
     `<data-dir>/overlay/<id>/data` and the overlay upper/work live inside it;
   - bind mounts, named volumes, tmpfs, and NFS specs are mounted into the
     merged tree.
8. Build the `unshare` command:
   `unshare --fork --pid --mount --uts --ipc --kill-child [--net] <binary> init <id> <merged>`.
   `--net` is omitted for `--network host`.
9. Set up stdio/log pipes and, for bridge networking, a readiness handshake so
   the workload cannot run before the veth is moved into its namespace.
10. Start the init process and record namespace inode identities.
11. Create the cgroup, move the init PID into it, apply memory/CPU limits, and
    save the cgroup path.
12. Attach the container to a bridge, allocate an IP, move the veth, and install
    port forwarding.
13. Persist the container JSON and start monitoring.

### In-namespace sequence (`InitContainer`)

1. Load container state and wait for the network handshake.
2. Read the image config if present.
3. Set the hostname, `chdir` into the merged root, provision the exec helper,
   and `pivot_root` into the merged tree.
4. Mount `proc`, `devtmpfs`, `sysfs`, and `devpts`; create `/dev`, `/sys`,
   `/dev/pts`, and `/tmp` with expected modes.
5. Prune `devtmpfs` to pseudo-devices so a container does not inherit host
   disks, KVM, or other device nodes.
6. Apply only allowlisted namespaced sysctls (`allowedContainerSysctl`).
7. Remount `/proc/sys` and `/sys` read-only.
8. Apply device restrictions (best effort, logged on failure):
   `/dev/shm` tmpfs (`size=64M,noexec,nosuid,nodev` when supported),
   `/dev/mqueue` (`noexec,nosuid,nodev`), read-only `/proc/sys` and `/sys`.
9. Apply the capability set and bounding-set drops.
10. Switch UID/GID when `--user` is set.
11. Set `no_new_privs` unconditionally.
12. Remount `/` read-only when requested.
13. Apply ulimits, inject `CARDINAL_*` environment variables, and write
    `/cardinal/{info,env,help}` helper scripts.
14. Apply the default seccomp filter immediately before `exec` (fail-closed).
15. Exec the startup script if supplied; otherwise exec the image entrypoint/CMD.

Ordering matters: seccomp is last because the read-only remount and `/proc`
setup themselves need syscalls that the default profile denies.

## Namespaces and cgroups

- Namespaces: PID, mount, UTS, IPC, and (unless `--network host`) network.
  User namespaces are used by the rootless path, not by the default root path.
- Namespace inode numbers are recorded in container JSON for later `exec`,
  `attach`, and validation.
- cgroups use v2 when `/sys/fs/cgroup/cgroup.controllers` exists:
  - memory: `memory.max`
  - CPU: `cpu.max` with a 100 ms period
  - PID and I/O controllers are enabled for accounting
  - the init PID is written to `cgroup.procs`
- Failures in enabling controllers or writing limits are warnings, not fatal;
  the container may run without limits on hosts that do not delegate cgroups
  (common on LXC/OpenVZ VPS hosts).
- Cleanup kills remaining cgroup processes before removing the cgroup.

## Networking

Default bridge:

- bridge `cardinal0`, CIDR `10.0.2.0/24`, gateway `10.0.2.1`;
- `EnsureBridge` creates the bridge, the container-egress MASQUERADE rule, the
  loopback MASQUERADE (`127.0.0.0/8` → bridge subnet), FORWARD accepts, and the
  link-local egress drop for `169.254.0.0/16`;
- a host `127.0.0.1:<published-port>` connection is source-NATed to the
  gateway, so the container replies to the gateway instead of routing its
  SYN-ACK to its own loopback;
- `route_localnet` is enabled per bridge interface
  (`net.ipv4.conf.<bridge>.route_localnet`), not globally, so the de-NATed
  reply whose destination is `127.0.0.1` is delivered. Two guards are
  installed first: bridge traffic with a loopback source is dropped (spoof
  protection), and bridge traffic to `127.0.0.0/8` is dropped unless conntrack
  reports `ESTABLISHED,RELATED`. Both exempt tracked flows because reverse NAT
  restores a published-port reply to `127.0.0.1 -> 127.0.0.1`. If a guard
  cannot be installed, `route_localnet` stays off for that bridge;
- IP allocations are tracked in `<data-dir>/networks/ips.json`;
- each container gets a veth pair (`ve<id>` / `vc<id>`), moved into its netns;
- published ports use iptables DNAT, with a `socat` fallback when the firewall
  path is unavailable.

User-defined networks (`cardinal network create`) get their own bridge, subnet,
allocation state, and firewall rules. `--isolated` applies a default-deny
policy between containers using dedicated iptables chains. Rootless mode
supports bridge/host/none but rejects user-defined bridge networks at start.

DNS records for container/service names are maintained in
`internal/orchestrator/dns.go`; a lightweight UDP DNS server answers service
lookups. Normal container `/etc/resolv.conf` generation is handled in
`internal/container/dns.go`.

## Lifecycle, restart, health, logs

- `start.go` owns start, monitoring, restart policy (`no`, `always`,
  `on-failure`, `unless-stopped`), restart delay, max attempts, and crash-loop
  window.
- `finalizeStopped` releases DNS, network, console, and cgroup resources exactly
  once, persists `Stopped`, and decides whether to restart.
- Healthchecks run as goroutines and use `exec` inside the container's
  namespaces; failures affect container status and service dependency checks.
- Logs are files under `<data-dir>/logs/<id>.log`; `logs -f` streams them.
- `exec`, `attach`, `console`, `cp`, and `fs` operations enter the recorded
  namespaces instead of opening a daemon socket.
- The optional `supervisor` re-attaches to containers after a host reboot or
  CLI exit, using persisted state.

## Rootless mode

`internal/container/rootless.go` detects non-root execution and checks:

- `fuse-overlayfs` for rootless overlay (`MountRootlessOverlay`);
- `newuidmap`/`newgidmap` and `/etc/subuid`/`/etc/subgid` mappings;
- `slirp4netns` (wired) or `pasta` (detected; exec wiring is a follow-up).

Rootless mode is `Partial`: host/none networking works, bridge networking needs
a userspace backend, and user-defined bridge networks are rejected.

## Compose and config

`internal/config` loads either `cardinal.toml` or a compose YAML file
(`LoadConfigOrCompose`). It supports:

- `.env` loading and `${VAR}` interpolation;
- `env_file` as a scalar or list;
- ports, volumes (short and long syntax), networks, healthchecks;
- `depends_on` with `service_started`, `service_healthy`, and
  `service_completed_successfully`;
- deploy resources, restart policy, update config, placement;
- top-level secrets/configs, materialized into container secret/config mounts.

`cmd/up.go` topologically sorts services by `depends_on`, pulls images, creates
or reuses containers, waits for dependencies, and starts them. `cmd/down.go`
stops and removes the selected containers and networks.

## Key source files

| Concern | File |
|---|---|
| Start orchestration | `internal/container/start.go` |
| In-namespace init | `internal/container/init.go` |
| Capabilities and ulimits | `internal/container/init.go` |
| Seccomp | `internal/container/seccomp.go` |
| Overlay and disk limits | `internal/container/overlay.go` |
| cgroups | `internal/container/cgroup.go` |
| Namespace validation | `internal/container/namespace.go` |
| Host-path blocklist | `internal/container/hostpath.go` |
| Networking | `internal/network/*.go` |
| Image pull/store | `internal/image/*.go` |
| Builder | `internal/builder/*.go` |
| Compose/config | `internal/config/*.go` |
| Lifecycle tests | `internal/container/lifecycle_test.go`, `lifecycle_e2e_test.go` |

## Verification

Linux-only:

```sh
go test ./internal/container/... ./internal/network/... ./internal/image/... ./internal/builder/... -count=1
```

Privileged behavior (namespaces, overlay, cgroups, iptables) is covered by
`.github/workflows/e2e.yml`; a compile-only check is not runtime verification.
