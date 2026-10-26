<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# internal/api — HTTP contract

Read [`docs/architecture/api-and-orchestration.md`](../../docs/architecture/api-and-orchestration.md)
and [`docs/architecture/security.md`](../../docs/architecture/security.md)
before changing a route or a response. The root [`AGENTS.md`](../../AGENTS.md)
lists the contracts that must not be broken.

This package is Linux-only (`//go:build linux`).

## Shape

- `server.go` starts the listener, owns the token gate, middleware order,
  routes, and shared response helpers.
- `types.go` defines the Docker-compatible response types. These JSON names are
  a public contract.
- `containers.go`, `images.go`, `system.go`, `events.go`, and `cluster.go`
  implement handlers.
- `internal/client` is the in-repo consumer. Its tests are the cheapest way to
  detect a response-shape break.
- `cmd/*.go` calls the client for remote mode.

## Routes

The paths are **unversioned** even though `/version` reports
`DockerAPIVersion = "1.44"`. Do not add a `/v1.44/...` prefix without checking
the client, the panel/wings integration, and both language docs.

| Path | Methods | Handler |
|---|---|---|
| `/_ping` | GET | `handlePing` |
| `/version` | GET | `handleVersion` |
| `/info` | GET | `handleInfo` |
| `/containers/json` | GET | `handleContainersList` |
| `/containers/create` | POST | `handleContainersCreate` |
| `/containers/{id}` | DELETE | remove |
| `/containers/{id}/json` | GET | inspect |
| `/containers/{id}/{start,stop,restart,kill,rename,update,exec,wait}` | POST | lifecycle/action handlers |
| `/containers/{id}/{logs,top,stats,export,changes}` | GET | read handlers |
| `/images/json` | GET | `handleImagesList` |
| `/images/{name}/json` | GET | inspect |
| `/images/{name}/{remove,push,tag,pull}` | POST/DELETE | image actions |
| `/images/{name}/{history,get}` | GET | image read actions |
| `/system/prune` | POST | `handleSystemPrune` |
| `/cluster/replicas`, `/cluster/replicas/{id}`, `/cluster/health` | GET/POST/DELETE | cluster router |
| `/cluster/containers` | GET | `handleListContainersOnNode` |
| `/metrics` | GET | Prometheus handler, wrapped in auth |
| `/events` | GET | SSE stream |
| `/` | GET | API index |

Container lookup falls back to ID prefix and exact name. Preserve that behavior:
the panel and `cardinal` commands rely on short IDs and names.

## Contracts and status semantics

- Do not rename JSON fields in `types.go` or change their types. Wings/panel
  clients parse them.
- Preserve `204` for successful lifecycle actions and `304` where a handler
  already returns it. Do not "improve" status codes without checking the client.
- Use `writeJSON` and `writeError`; do not hand-roll a response body.
- Method mismatches return `405`. Missing objects return `404`. Validation
  failures return `400`. Duplicate name/ID conflicts return `409`.
- `DELETE /containers/{id}` is Docker's remove convention. Keep it.
- `exec` rejects TTY requests with `400`; there is no PTY support over this API.
- `/events` is Server-Sent Events. Do not change the frame format without
  updating `internal/client` and `cmd/events.go`.

## Security invariants

- The server refuses to start without a bearer token on **every** bind,
  including loopback. This is not "external hosts only".
- `authMiddleware` compares the full `Authorization: Bearer ...` value with
  `subtle.ConstantTimeCompare`. Query-string tokens are rejected.
- Middleware order is
  `hostGuard -> CORS -> rateLimiter -> auth -> jsonContentType -> mux`.
  Reordering can create a bypass; change it only with a security test.
- `hostGuardMiddleware` blocks DNS-rebinding on loopback binds.
- CORS allows only configured localhost/loopback origins by default and rejects
  a foreign `Origin` instead of merely omitting headers.
- Request bodies are capped by `http.MaxBytesReader` at 8 MiB.
- `/metrics` is wrapped in `authMiddleware`; do not expose it unauthenticated.
- Do not add a tokenless localhost path, a query-token fallback, or a
  permissive CORS wildcard to make a client work.

## Verification

```sh
gofmt -l internal/api internal/client
go test ./internal/api/... ./internal/client/... ./cmd/... -count=1
go vet ./internal/api/... ./internal/client/...
```

These tests use `httptest` and do not need root. A real server smoke test needs
Linux plus a token:

```sh
cardinal serve -p 2375 --token "$CARDINAL_TOKEN" &
curl -H "Authorization: Bearer $CARDINAL_TOKEN" http://127.0.0.1:2375/version
```

On Windows, cross-compile only:

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; go vet ./...
```

Useful tests: `TestStartServerRefusesTokenlessBind`,
`TestAuthMiddlewareRequiresBearerHeader`, `TestIsExternalHost`,
`TestAllowedCORSOrigin`, `TestHandleContainerExec`, `TestHandleEventsSSE`, and
`internal/client/client_test.go`.

If you add a route, add a handler test and update
`docs/architecture/api-and-orchestration.md`. If the route is remote-capable,
also add a `cmd` remote test and update the remote-capable list in
[`cmd/AGENTS.md`](../../cmd/AGENTS.md).
