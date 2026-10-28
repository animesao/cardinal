<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# ADR-0003: Docker-compatible HTTP subset

- **Status:** Accepted
- **Date:** 2026-10-26
- **Scope:** `internal/api`, `internal/client`, external integrations

## Context

Panels and control planes (Pterodactyl Wings, Portainer, custom tooling) speak
the Docker Engine API. Implementing the full Engine API is a large, evolving
surface that would dominate the project. Implementing a proprietary API would
force every integration to write a custom client.

## Decision

Serve a deliberately selected subset of Docker-shaped routes and response
fields. Reuse Docker names and shapes where they are useful, and document where
the implementation stops. Do not claim full Engine API compatibility.

## Evidence

- `internal/api/server.go` advertises `DockerAPIVersion = "1.44"` and registers
  a fixed route table.
- Routes are unversioned: there is no `/v1.44/...` handler.
- `internal/api/types.go` defines the Docker-shaped request/response structs.
- Supported routes cover container lifecycle, inspect, logs, top, stats, exec,
  wait, export, changes, image list/pull/push/tag/remove/history/get, prune,
  events, metrics, and a small cluster surface.
- `internal/client` parses the same shapes for remote CLI mode.
- `internal/api/containers.go` deliberately accepts `DELETE /containers/{id}`
  because Wings and panel clients rely on it.

## Consequences

Positive:

- existing Docker clients and panels can drive the common lifecycle;
- one HTTP surface serves the CLI, the panel, and external tools;
- the contract is small enough to test and document.

Negative:

- clients that require full Engine semantics, versioned paths, or unimplemented
  endpoints will fail;
- Docker field names become a compatibility contract even when cardinal's
  internal model differs;
- changing a response field can break Wings/panel integrations silently;
- the advertised `1.44` version can mislead a client into expecting endpoints
  that do not exist.

## Rules

1. Additive changes are preferred; renaming or removing a field is a breaking
   change.
2. Before changing a field, status code, or 204/304 response, check
   `internal/api/types.go`, the handler, `internal/client`, Wings/panel usage,
   and both language docs.
3. New routes should follow Docker naming where a Docker equivalent exists.
4. Do not add a version prefix without updating every client and integration.
5. Document unsupported endpoints as unsupported; do not fake success.

## Alternatives rejected

- **Full Docker Engine API.** Rejected as disproportionate and a permanent
  compatibility burden.
- **Proprietary API only.** Rejected because it breaks existing panel and
  tooling integrations.
- **Versioned URL prefix.** Rejected for now because no client requires it and
  it would add a routing layer without a compatibility benefit.

## Related

- [`../architecture/api-and-orchestration.md`](../architecture/api-and-orchestration.md)
- [`../status/known-issues.md`](../status/known-issues.md)
