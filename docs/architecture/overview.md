<!-- cardinal-version:start -->
**Documentation version:** `2.4.10`
**Project release:** `v2.4.10`
<!-- cardinal-version:end -->

# Architecture overview

`cardinal` is a daemonless Linux container runtime written in Go. The normal
entry point is a single CLI binary that reads and writes a local state
directory. Long-running components exist, but none is required for basic
container operation.

## Component map

```
main.go
  └─ cmd (cobra tree)
      ├─ container   namespaces, overlayfs, cgroups, lifecycle, backups
      ├─ image       OCI pull/push/store/export/import, digest verification
      ├─ builder     Dockerfile parser and image builder
      ├─ network     bridge, veth, iptables, user networks, isolation, DNS records
      ├─ config      cardinal.toml / compose YAML loading and interpolation
      ├─ state       data-directory layout, path safety, atomic JSON
      ├─ api         Docker-compatible HTTP subset
      ├─ client      HTTP client used by remote CLI mode
      └─ orchestrator clusters, services, scheduler, FaaS, DNS, metrics
```

Dependency direction is toward the lower-level packages: `cmd` → `container`,
`image`, `config`, `api`, `client`, `orchestrator` → `state`, `network`,
`overlayutil`, `log`. Avoid importing `cmd` from runtime packages.

## Process models

| Process | Lifetime | Role |
|---|---|---|
| `cardinal <command>` | One shot | Parses CLI args, mutates state, starts/stops containers |
| `cardinal init` | Container lifetime | PID 1 inside the new namespaces; sets up rootfs and execs the workload |
| `cardinal serve` | Optional daemon | HTTP API on a TCP port; systemd mode available |
| `cardinal supervisor` | Optional daemon | Restarts containers and runs scheduled/background work |
| `cardinal console-serve` | Container lifetime | Serves the web console socket |
| `unshare` helper | Container lifetime | Owns the PID/mount/UTS/IPC(/net) namespace tree |

There is no mandatory long-running daemon and no central socket. Two CLI
invocations coordinate through files and OS locks in the state directory, not
through an RPC service. This is the core trade-off described in
[`ADR-0001`](../decisions/ADR-0001-daemonless-static-binary.md).

## Primary data flows

### Local container start

1. CLI parses flags and applies the shared runtime policy
   (`internal/container/security_policy.go`).
2. `Container.Start` → `startInternal` validates the request, prepares the
   overlay and optional disk image, and launches `unshare`.
3. The child runs `cardinal init`, pivots into the merged rootfs, applies
   kernel/security settings, and execs the workload (`internal/container/init.go`).
4. The parent records namespaces, cgroup path, IP, port forwards, and persists
   the container JSON atomically.

Detail: [`runtime.md`](runtime.md).

### Image pull

1. Registry allowlist and SSRF checks run before any network call
   (`internal/image/allowlist.go`, `pull.go`).
2. The manifest/config/layers are fetched over HTTPS; each blob digest is
   verified.
3. Layers are cached under the state directory and extracted into the image
   rootfs. Metadata is stored as JSON.

Detail: [`runtime.md`](runtime.md) and [`state-and-storage.md`](state-and-storage.md).

### API request

1. `StartServerWithTLS` refuses to bind without a Bearer token.
2. Middleware order is host guard → CORS → rate limiter → auth → JSON content
   type → routes.
3. Handlers translate the Docker-shaped request into the same internal
   container operations used by the CLI.

Detail: [`api-and-orchestration.md`](api-and-orchestration.md).

### Compose/config start

1. `internal/config` loads `cardinal.toml` or a compose YAML file, applies
   `.env` interpolation, secrets/config references, and dependency ordering.
2. `cmd/up.go` creates the network and containers, then starts them in order.
   `cmd/down.go` stops and removes them.

Detail: [`runtime.md`](runtime.md).

## Architectural boundaries

- **Linux only.** Runtime files carry `//go:build linux`. A Windows checkout is
  for editing and cross-compilation, not execution.
- **Filesystem state.** There is no database and no daemon-owned in-memory
  source of truth. Persisted JSON is authoritative.
- **Docker-compatible subset.** The HTTP layer mirrors selected Docker Engine
  fields and routes, not the full Engine API. It is unversioned at the URL
  level even though it advertises `ApiVersion: 1.44`.
- **Fail-closed security.** Unknown or unimplemented security configuration is
  rejected rather than silently ignored.
- **One binary.** CLI, container init, API server, supervisor, and console
  helper are subcommands of the same executable.

## Invariants to preserve

- Default seccomp, `no_new_privs`, capability dropping, host-path blocklists,
  API token enforcement, cluster token checks, and archive path checks are not
  optional.
- Persisted JSON field names and Docker response field names are contracts.
- Version markers in Markdown are generated; do not hand-edit the block.
- A container start that cannot apply a requested security control must fail.

## Where to go next

- Runtime behavior: [`runtime.md`](runtime.md)
- Threat model and controls: [`security.md`](security.md)
- On-disk layout and recovery: [`state-and-storage.md`](state-and-storage.md)
- HTTP and orchestration contracts: [`api-and-orchestration.md`](api-and-orchestration.md)
- Build and release: [`build-and-release.md`](build-and-release.md)
