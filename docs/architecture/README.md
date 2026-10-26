# Architecture documents

These documents describe how `cardinal` works today, with file-level evidence.
They are not release notes and not a second README.

| Document | Subsystem |
|---|---|
| [`overview.md`](overview.md) | Component map, process models, data flow, boundaries |
| [`runtime.md`](runtime.md) | Images, container start sequence, namespaces, overlay, cgroups, networking, lifecycle, compose |
| [`security.md`](security.md) | Runtime hardening, seccomp, capabilities, host-path policy, API/cluster auth, supply chain |
| [`state-and-storage.md`](state-and-storage.md) | Data directory layout, atomic writes, prune, volumes, backups |
| [`api-and-orchestration.md`](api-and-orchestration.md) | HTTP API, Docker-compatible subset, remote client, clusters, services, FaaS, DNS |
| [`build-and-release.md`](build-and-release.md) | Toolchain, Makefile, CI workflows, release artifacts, version synchronization |

## Conventions

- "Current behavior" means the code at the commit being read, not the newest
  plan in `docs/plans/`.
- Every non-obvious claim should name a file, symbol, test, or command.
- Use the fact labels from the root `AGENTS.md`.
- When a document and the code disagree, the code wins. Record the drift in
  [`docs/status/known-issues.md`](../status/known-issues.md).

## Related decisions

Durable "why" answers live in [`docs/decisions/`](../decisions/):

- daemonless single binary;
- Linux-only runtime with build tags;
- Docker-compatible HTTP subset;
- filesystem state instead of a database;
- fail-closed security profiles;
- dual cobra/legacy CLI layers.
