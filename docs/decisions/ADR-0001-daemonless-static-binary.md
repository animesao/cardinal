<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# ADR-0001: One static binary, no mandatory daemon

- **Status:** Accepted
- **Date:** 2026-10-26
- **Scope:** Process model, packaging, state coordination

## Context

Container runtimes commonly run a long-lived daemon that owns state and exposes
a socket. That model centralizes state but adds an always-on privileged process,
an IPC protocol, service management, and a daemon lifecycle to every operation.
`cardinal` targets small hosts and panel-style deployments where a single
binary and a data directory are easier to install and reason about.

## Decision

Ship one static Linux binary. `cardinal <command>` performs the operation
directly against the data directory. Long-running components are optional
subcommands of the same binary:

- `cardinal serve` — HTTP API;
- `cardinal supervisor` — restart and scheduled work;
- `cardinal console-serve` — console socket;
- `cardinal init` — PID 1 inside a container.

No component is required for basic `pull`, `run`, `stop`, and `rm`.

## Evidence

- `main.go` calls `cmd.Execute()`; there is no daemon bootstrap.
- `state.DataDir()` is the source of truth; there is no database and no
  daemon-owned in-memory state.
- `Makefile` builds `CGO_ENABLED=0` linux/amd64 and linux/arm64 binaries.
- `docs/architecture/overview.md` lists the process models.
- `cmd/serve.go`, `cmd/supervisor.go`, and `cmd/consoleserve.go` are optional
  subcommands.

## Consequences

Positive:

- installation is a file copy plus an optional systemd unit;
- state can be inspected and repaired with ordinary file tools;
- no socket protocol version skew between CLI and daemon;
- the same binary serves as CLI, API server, and container init.

Negative:

- concurrent CLI processes coordinate through files, not transactions;
- there is no central scheduler or in-memory cache;
- privilege is per-invocation; every state mutation runs with the caller's
  rights;
- long-running behavior (restart policy, service reconciliation) needs the
  optional supervisor.

## Alternatives rejected

- **Always-on daemon with a Unix socket.** Rejected because it adds an
  always-running privileged surface and a mandatory service.
- **Embedded database (SQLite/Bolt).** Rejected because it adds a binary format,
  migration burden, and recovery difficulty for a small host tool.
- **Separate binaries per role.** Rejected because it multiplies packaging and
  version-skew surface.

## Related

- [`ADR-0004`](ADR-0004-filesystem-state.md) — the storage model that makes this
  work.
- [`../architecture/overview.md`](../architecture/overview.md) — process models.
