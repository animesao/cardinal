# Architecture decisions

These records explain durable choices that shape the codebase. They are not a
history of every change and not a substitute for the architecture documents.
Each ADR states the decision, the evidence in the repository, the consequences,
and what was rejected.

| ADR | Decision | Status |
|---|---|---|
| [`ADR-0001`](ADR-0001-daemonless-static-binary.md) | One static binary, no mandatory daemon | Accepted |
| [`ADR-0002`](ADR-0002-linux-only-build-tags.md) | Linux-only runtime behind build tags | Accepted |
| [`ADR-0003`](ADR-0003-docker-api-subset.md) | Docker-compatible HTTP subset, not full Engine API | Accepted |
| [`ADR-0004`](ADR-0004-filesystem-state.md) | Filesystem state instead of a database | Accepted |
| [`ADR-0005`](ADR-0005-fail-closed-security-profiles.md) | Reject unsupported security configuration at start | Accepted |
| [`ADR-0006`](ADR-0006-dual-cli-layers.md) | Cobra registry over legacy command functions | Accepted, with debt |

## Conventions

- **Accepted** means the behavior is implemented and tested or exercised.
- **Accepted, with debt** means the decision holds but a migration or cleanup is
  incomplete.
- A new ADR is warranted only for a durable, cross-cutting decision. A local
  implementation choice belongs in code comments or an architecture document.
- Superseding an ADR means adding a new record and linking both directions; do
  not rewrite history.
