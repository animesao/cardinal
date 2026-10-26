<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# API and orchestration

`cardinal serve` exposes an HTTP API shaped like a subset of the Docker Engine
API. The same process can also act as a cluster peer, host service replicas, and
run FaaS functions. This document separates the HTTP contract from the
orchestration features that sit behind it.

## Server startup

`api.StartServerWithTLS(port, host, certFile, keyFile)`:

1. rejects a half-configured TLS pair (one file without the other);
2. rejects an external bind without a token;
3. rejects **every** bind without a token, including loopback;
4. registers routes;
5. listens on `host:port`;
6. wraps the mux in middleware and serves HTTP or HTTPS.

The token comes from `--token` or `CARDINAL_TOKEN`. `cmd/serve.go` still has a
comment saying the token can be disabled; the server does not allow that. See
[`known-issues.md`](../status/known-issues.md).

## Middleware order

The handler chain is:

```
hostGuard → CORS → rateLimiter → auth → jsonContentType → mux
```

| Layer | Behavior |
|---|---|
| Host guard | Active only on loopback binds; rejects Host values other than localhost/127.0.0.1/::1 (DNS-rebinding defense) |
| CORS | Allows localhost origins plus `CARDINAL_CORS_ORIGINS`; rejects a present, disallowed Origin; answers preflight with 204 |
| Rate limiter | Per-IP token bucket, 25 req/s burst of 50; loopback exempt |
| Auth | Requires exactly `Authorization: Bearer <token>`; constant-time compare; query/raw tokens rejected |
| JSON content type | Sets `Content-Type: application/json`; caps non-GET bodies at 8 MiB |

Server timeouts: 10 s header read, 60 s read, 60 s write, 120 s idle,
1 MiB max header. `authMiddleware` has an `authToken == ""` pass-through branch,
but the startup gate makes it unreachable for a running server.

## Routes

All routes are **unversioned**. There is no `/v1.44/...` handler; clients must
call the paths below even though `/version` advertises `ApiVersion: 1.44`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/_ping` | Liveness, `OK` |
| GET | `/version` | Docker-shaped version object |
| GET | `/info` | Host and runtime summary |
| GET | `/containers/json` | List containers |
| POST | `/containers/create` | Create and start a container |
| GET | `/containers/{id}/json` | Inspect |
| POST | `/containers/{id}/start` | Start |
| POST | `/containers/{id}/stop` | Stop |
| POST | `/containers/{id}/restart` | Restart |
| POST | `/containers/{id}/kill` | Kill |
| DELETE | `/containers/{id}` | Remove |
| GET | `/containers/{id}/logs` | Logs |
| GET | `/containers/{id}/top` | Processes |
| GET | `/containers/{id}/stats` | Resource stats |
| POST | `/containers/{id}/rename` | Rename |
| POST | `/containers/{id}/update` | Update limits |
| POST | `/containers/{id}/exec` | Exec |
| POST | `/containers/{id}/wait` | Wait for exit |
| GET | `/containers/{id}/export` | Export filesystem |
| GET | `/containers/{id}/changes` | Filesystem changes |
| GET | `/images/json` | List images |
| POST | `/images/{name}/pull` | Pull |
| POST | `/images/{name}/push` | Push |
| POST | `/images/{name}/tag` | Tag |
| DELETE | `/images/{name}` | Remove |
| GET | `/images/{name}/history` | History |
| GET | `/images/{name}/get` | Export |
| POST | `/system/prune` | Prune |
| GET | `/cluster/replicas` | Cluster replica listing |
| GET | `/cluster/health` | Cluster health |
| POST/DELETE | `/cluster/replicas/{id}` | Schedule/remove a replica |
| GET | `/cluster/containers` | Containers on a node |
| GET | `/events` | Server-sent events |
| GET | `/metrics` | Prometheus metrics |
| * | `/` | Root/help response |

Container lookup accepts an exact ID, an ID prefix, or a name. The `create`
handler starts the container immediately and removes the saved JSON if the
start fails.

## Response-shape contract

The API mirrors Docker fields, not the full Docker API. Compatibility matters
most for Pterodactyl Wings and panel integrations. Before changing a JSON field
name, a status code, or a 204/304 response:

1. check `internal/api/types.go` and the handler;
2. check `internal/client` for the field the CLI parses;
3. check both language docs;
4. assume external consumers exist.

Known shape choices:

- `CreateContainerResponse` returns `201` with `Id`.
- Stop/kill/remove return `204` on success.
- Container inspect wraps the runtime struct in a Docker-like envelope.
- `/events` uses `text/event-stream`; each event is `data: <json>` followed by
  a blank line.
- `/metrics` is behind auth unless `CARDINAL_METRICS_REQUIRES_AUTH=0`.

## Remote CLI mode

`--host`/`-H` or `CARDINAL_REMOTE_HOST` selects a remote server;
`--token`/`CARDINAL_TOKEN` supplies the Bearer token. Remote support exists for:

`ps`, `info`, `inspect`, `logs` (including `-f`), `events`, `exec`, `start`,
`stop`, `restart`, `stats`, and `top`.

Everything else mutates local state only. The root help still says "read-only
remote mode"; that is stale. The CLI is a thin wrapper over
`internal/client`:

- `get` for JSON reads;
- `doPost` for mutations;
- `StreamLogs` and `StreamEvents` for streaming responses;
- errors are decoded from the Docker-style `{"message": ...}` body.

## Clusters

Cluster state lives in `cluster/cluster.json`. Commands:

```
cardinal cluster init
cardinal cluster join <peer>
cardinal cluster join-token
cardinal cluster leave
cardinal cluster info
cardinal cluster ls
cardinal cluster node ls|inspect
cardinal cluster serve
```

- `init` and `join` wire the token into both `api.SetAuthToken` and
  `orchestrator.SetAPIToken`.
- Peer RPCs use `Authorization: Bearer <CARDINAL_TOKEN>` and fail closed when
  no token is configured.
- `cluster join-token` is a connection hint, not a secret; the shared token is
  still required.
- The cluster file is written atomically and has a `.bak` copy.

**Partial:** cluster membership, replica scheduling, and health are file-backed
and best effort. There is no consensus protocol, no leader election beyond the
recorded role, and no split-brain protection.

## Services

`internal/orchestrator` stores service definitions in `services/services.json`
and one JSON per replica under `services/<name>/`.

- `CreateService` validates the name and image, records desired replicas, and
  schedules them.
- `ReconcileServices` compares desired and actual replicas and scales up or
  down.
- `AutoHealServices` (started by `StartAutoHealer`) recreates missing replicas.
- `RollingUpdateService`/`UpdateService` replace replicas one at a time.
- `ScaleService` changes the desired count.
- Replica placement chooses a local start or a remote node through
  `internal/orchestrator/scheduler.go`.
- `RemoveService` stops and removes replicas, then deletes its records.

**Partial:** reconciliation is periodic, not transactional. A crash between
starting a replica and persisting its record can leave an untracked container.

## FaaS

FaaS definitions live in `functions/functions.json`.

- `DeployFunction` creates a function from an image and port, with optional
  environment, memory, CPU, and idle settings.
- `InvokeFunction` forwards a payload to a running replica, scaling up on a
  cold start and rolling back containers if the scale-up fails.
- `StartFunctionGC` and `CleanIdleFunctions` scale down idle replicas.
- `RemoveFunction` removes replicas and the definition.

The invocation transport is HTTP forwarding to a replica, not an in-process
runtime. A function image must therefore expose an HTTP endpoint.

## DNS

`internal/orchestrator/dns.go` provides a small UDP DNS server:

- `StartDNSServer(addr)` listens for UDP queries;
- `handleDNSQuery` extracts the service name and answers with A records;
- `ResolveService` prefers the in-memory DNS record map and falls back to
  discovering container IPs;
- `AddDNSRecord`/`RemoveDNSRecord` are called when containers start and stop.

**Partial:** this is a minimal A-record responder. It has no DNSSEC, no
recursion, no caching semantics, and no TCP fallback.

## Metrics and events

- `/metrics` exposes Prometheus collectors from `internal/orchestrator/metrics.go`.
- `/events` subscribes to the container event bus with a 100-event buffer and
  streams SSE until the client disconnects.
- `cardinal events` can consume the same stream locally or remotely.

## Compatibility and failure rules

- Do not add a version prefix to routes without updating every client and
  integration.
- Do not change a persisted JSON field or Docker response field without checking
  `internal/client`, Wings/panel usage, and both language docs.
- A request that cannot apply its security policy must fail; the API must not
  fall back to a weaker default.
- The server must not start without a token, even on loopback.

## Verification

| Check | Command |
|---|---|
| API handlers and middleware | `go test ./internal/api/ -count=1` on Linux |
| Remote client | `go test ./internal/client/ -count=1` on Linux |
| CLI remote paths | `go test ./cmd/ -run Remote -count=1` on Linux |
| Cluster/scheduler/service/FaaS | `go test ./internal/orchestrator/ -count=1` on Linux |
| Manual ping | `curl -H "Authorization: Bearer $CARDINAL_TOKEN" http://127.0.0.1:2375/_ping` |

Cross-compiling the server proves it builds, not that it serves or enforces auth
on a Linux host.
