<!-- cardinal-version:start -->
**Documentation version:** `2.4.10`
**Project release:** `v2.4.10`
<!-- cardinal-version:end -->

# ADR-0004: Filesystem state instead of a database

- **Status:** Accepted
- **Date:** 2026-10-26
- **Scope:** `internal/state`, persisted JSON, recovery

## Context

`cardinal` must work without a daemon and on small hosts. A database would add
a process, a schema, migrations, a binary file format, and a recovery tool. The
data model is mostly independent records: one image, one container, one volume,
one service. Cross-record transactions are rare and not required for basic
operation.

## Decision

Store state as JSON and directories under a single data directory. Use atomic
file replacement for writes. Treat persisted field names as a public contract.
Do not introduce a database for core state.

## Evidence

- `internal/state/state.go` defines the directory layout and
  `WriteFileAtomic`.
- `WriteJSON` writes mode `0600` via `WriteFileAtomic`.
- `EnsureDirs` creates the tree with `0700`.
- `internal/container/types.go` persists a container via
  `state.WriteJSON(state.ContainerPath(c.ID), c)`.
- Image metadata is `image.json` / `config.json`; volumes are directories with
  metadata; orchestrator state is `cluster.json`, `services.json`, and
  `functions.json`.
- `internal/image/store.go` falls back to legacy `manifest.json` for older
  images.

## Consequences

Positive:

- no database process, driver, or schema migration;
- state is inspectable with `cat`, `jq`, `ls`, and ordinary backups;
- recovery is possible offline;
- a single static binary remains possible.

Negative:

- no multi-file transaction; a crash between two writes can leave inconsistent
  records;
- no cross-process lock for container JSON, so two concurrent CLI processes can
  lose an update;
- listing and reconciliation scan directories;
- every persisted field name is a compatibility contract;
- large states are slower than indexed queries.

## Rules

1. Every write goes through `state.WriteFileAtomic` (or `WriteJSON`).
2. Do not change a persisted JSON field name without checking `internal/client`,
   panel/Wings usage, and both language docs.
3. Validate any user-controlled string before it becomes a path component; use
   `state.safePart` and the validated image/volume helpers.
4. New state files get `0600`; directories get `0700`.
5. If a cross-record invariant becomes necessary, add an explicit recovery or
   reconciliation step rather than assuming atomicity.

## Alternatives rejected

- **SQLite.** Rejected for binary state, driver/CGO considerations, migration
  burden, and reduced offline inspectability.
- **Embedded key-value store.** Rejected for opaque encoding and recovery
  complexity.
- **Daemon-owned in-memory state with periodic snapshots.** Rejected because it
  reintroduces a mandatory daemon and loses state on crash.

## Related

- [`../architecture/state-and-storage.md`](../architecture/state-and-storage.md)
- [`ADR-0001`](ADR-0001-daemonless-static-binary.md)
