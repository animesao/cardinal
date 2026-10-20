<!-- cardinal-version:start -->
**Documentation version:** `2.4.8`
**Project release:** `v2.4.8`
<!-- cardinal-version:end -->

# Remote Mode Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `cardinal --host/--token` remote mode from `ps`/`info`/`logs --tail` to `exec`, `stats`, `top`, `inspect` and `events` against `cardinal serve`.

**Architecture:** Server side: rework `POST /containers/{id}/exec` to capture output (non-interactive only) and add SSE `GET /events`; `stats?stream=0`, `top`, `inspect` endpoints already exist and are reused as-is. Client side: add `Exec`/`Top`/`Stats`/`Inspect`/`StreamEvents` to the portable `internal/client` package with `httptest` coverage. CLI side: thin `*Remote` branches in the existing handlers (no cobra changes — all target commands already go through `extractRemoteFlags`).

**Tech Stack:** Go 1.26, stdlib `net/http` (+ `httptest`), existing `container.ExecOptsIO`, existing cobra/`DisableFlagParsing` wiring.

---

## File map

| File | Role |
|---|---|
| `internal/api/containers.go` | Rework `handleContainerExec`: capture stdout/stderr + exit code, reject TTY/stdin with 400 |
| `internal/api/events.go` (create) | SSE `GET /events`: live stream of `container.SubscribeEvents` as `data: {...}` frames |
| `internal/api/server.go` | Register `/events` route |
| `internal/api/api_test.go` (create, `//go:build linux`) | Handler tests: exec 405/400 paths, SSE framing + auth |
| `internal/client/client.go` | Add `doPost`, `Exec`, `Top`, `Stats`, `Inspect`, `Event`, `StreamEvents` |
| `internal/client/client_test.go` | Extend `testServer` mux + cases per method |
| `cmd/exec.go`, `cmd/top.go`, `cmd/stats.go`, `cmd/inspect.go`, `cmd/events.go` | Add `*Remote` branches |
| `cmd/remote_exec_test.go` (create, `//go:build linux`) | CLI remote tests via `httptest` + stubbed `exitFunc` |
| `README.md` | Extend remote-mode docs: new commands, limits (`-i/-t`, `logs -f`) |

Non-goals (documented, not implemented): `logs -f` streaming, `stats` server-side stream over remote (CLI polls `?stream=0` instead), remote lifecycle (`start`/`stop`), interactive TTY exec (400 + hint), wings changes.

---

### Task 1: server exec captures output

**Files:**
- Modify: `internal/api/containers.go` (`handleContainerExec`, ~lines 932-959)
- Test: `internal/api/api_test.go` (create)

Today the handler runs `c.ExecOpts` (server stdio — output lost over HTTP)
and returns only `{"Id"}`. New contract, backward compatible (fields added):

- `405` wrong method (unchanged), `400` empty `Cmd` (unchanged)
- `400` when `Tty` or `AttachStdin` is true: `interactive exec requires a TTY; use local cardinal exec -it or the wings terminal`
- `200 {"Id","Output","Stderr","ExitCode"}` otherwise; non-zero container
  exit stays `200` with `ExitCode` set (CLI propagates it)

- [ ] **Step 1: write failing handler tests** — create
  `internal/api/api_test.go` (`//go:build linux`, package `api`).
  `handleContainerExec` needs `*container.Container` but the 400/405 paths
  never touch namespaces, so `&container.Container{ID: "abc", Name: "web"}`
  suffices. Assert: GET → 405; POST `{"Cmd":[]}` → 400;
  POST `{"Cmd":["echo","hi"],"Tty":true}` → 400 + `TTY` in body.

- [ ] **Step 2: run, watch fail** — `go test ./internal/api/ -run TestHandleContainerExec -v`
  (runs on Linux CI; on Windows dev boxes verify compile with
  `GOOS=linux go vet ./internal/api/`). Expected: FAIL, undefined handler
  behavior (old code returns 200 + `{"Id"}` for the TTY case).

- [ ] **Step 3: implement** — replace `handleContainerExec` body after the
  `Cmd`-empty check with:

```go
if req.AttachStdin || req.Tty {
    writeError(w, 400, "interactive exec requires a TTY; use local `cardinal exec -it` or the wings terminal")
    return
}
var stdout, stderr bytes.Buffer
runErr := c.ExecOptsIO(req.Cmd, false, false, nil, &stdout, &stderr)
writeJSON(w, 200, map[string]interface{}{
    "Id":       shortID(c.ID, 12) + "_exec",
    "Output":   stdout.String(),
    "Stderr":   stderr.String(),
    "ExitCode": execExitCode(runErr),
})
```

plus helper (same file, next to the handler):

```go
// execExitCode maps an ExecOptsIO result to a process exit code: nil → 0,
// *exec.ExitError → its status, anything else → -1 (start failure; the
// handler already returned 500 for those, so -1 is defensive).
func execExitCode(err error) int {
    if err == nil {
        return 0
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return exitErr.ExitCode()
    }
    return -1
}
```

`handleContainerExec` itself keeps returning 500 when `ExecOptsIO` fails
to start (distinguish: `if runErr != nil && execExitCode(runErr) == -1`
→ 500, else 200 with code). Add imports `bytes`, `errors`, `os/exec`
to `containers.go` (check existing import block first — do not duplicate).

- [ ] **Step 4: run tests** — same command. Expected: PASS.
- [ ] **Step 5: vet** — `GOOS=linux go vet ./internal/api/` exit 0.
  (Standing rule: no commits without explicit request.)

### Task 2: client methods (Exec/Top/Stats/Inspect/StreamEvents)

**Files:**
- Modify: `internal/client/client.go`
- Modify: `internal/client/client_test.go` (extend mux + cases)

Portable package (no build tag) — mirrors `Summary`/`Info` pattern, runs on
Windows/macOS. New types (JSON tags match the serve schema exactly):

```go
type ExecResult struct {
    ID       string `json:"Id"`
    Output   string `json:"Output"`
    Stderr   string `json:"Stderr"`
    ExitCode int    `json:"ExitCode"`
}
type TopResult struct {
    Titles    []string `json:"Titles"`
    Processes []string `json:"Processes"`
}
type Stats struct {
    ContainerID   string  `json:"container_id"`
    Name          string  `json:"name"`
    MemoryUsage   uint64  `json:"memory_usage_bytes"`
    MemoryLimit   uint64  `json:"memory_limit_bytes"`
    MemoryPercent float64 `json:"memory_percent"`
    CPUPercent    float64 `json:"cpu_percent"`
    CPUCount      float64 `json:"cpu_count"`
    PIDsCurrent   uint64  `json:"pids_current"`
    IOReadBytes   uint64  `json:"io_read_bytes"`
    IOWriteBytes  uint64  `json:"io_write_bytes"`
    DiskUsage     uint64  `json:"disk_usage_bytes"`
}
type Event struct {
    Type      string    `json:"type"`
    ActorID   string    `json:"actor_id"`
    ActorName string    `json:"actor_name"`
    ImageName string    `json:"image_name"`
    ImageTag  string    `json:"image_tag"`
    Status    string    `json:"status"`
    Time      time.Time `json:"time"`
}
```

Methods (exact signatures — Task 3/5 depend on them):

```go
func (c *Client) doPost(ctx context.Context, path string, body, out interface{}) error
func (c *Client) Exec(ctx context.Context, id string, cmd []string) (ExecResult, error)
func (c *Client) Top(ctx context.Context, id string) (TopResult, error)
func (c *Client) Stats(ctx context.Context, id string) (Stats, error)
func (c *Client) Inspect(ctx context.Context, id string) (string, error)
func (c *Client) StreamEvents(ctx context.Context) (<-chan Event, <-chan error)
```

`Exec` POSTs JSON `{"Cmd":cmd}` via `doPost` (same 403/body error mapping
as `doReq`); `Stats` uses `?stream=0`; `Inspect` returns the raw body
(64 MiB cap like `Logs`); `StreamEvents` GETs `/events` with
`Accept: text/event-stream`, scans `data:` lines in a goroutine,
`json.Unmarshal`s each into `Event`, closes both channels on EOF/error/ctx
cancel. SSE must not be cut by the 15 s `hc` timeout, so `StreamEvents`
builds its request with a dedicated `http.Client{Timeout: 0}`.

- [ ] **Step 1: extend `testServer` mux** in `client_test.go` with
  `/containers/web/exec` (assert method POST + `Cmd`, return fixture),
  `/containers/web/top`, `/containers/web/stats` (assert `stream=0`),
  `/containers/web/json`, `/events` (two `data: {...}` frames then close).
- [ ] **Step 2: add failing test funcs** `TestExec/TestTop/TestStats/
  TestInspect/TestStreamEvents` asserting decoded values. Run
  `go test ./internal/client/ -v` — Expected: FAIL (undefined methods).
- [ ] **Step 3: implement** methods in `client.go` as specified.
- [ ] **Step 4: run** — same command. Expected: PASS, full package green.
  `gofmt` clean on both files.

### Task 3: CLI remote branches (exec/top/stats/inspect)

**Files:**
- Modify: `cmd/exec.go`, `cmd/top.go`, `cmd/stats.go`, `cmd/inspect.go`
- Test: `cmd/remote_exec_test.go` (create, `//go:build linux`)

No cobra changes: every target command already runs with
`DisableFlagParsing`, so `extractRemoteFlags` (cobra_commands.go:101-104)
fills `remoteHost`/`remoteToken` before the handler sees args. Verify with
`grep -n "spec.use" cmd/cobra_commands.go` that none of
exec/top/stats/inspect/events takes the `ps` branch.

- [ ] **Step 1: write failing CLI tests** — create `cmd/remote_exec_test.go`
  (`//go:build linux`, package `cmd`). Follow `cmd/exit_test.go:20-21` to
  stub `exitFunc`, and `internal/client/client_test.go` for the `httptest`
  stub server. Set `remoteHost`/`remoteToken` package vars directly
  (save/restore via `t.Cleanup`), then call `Exec([]string{...})` etc.
  Cases: exec prints Output and calls `exitFunc(3)` on `ExitCode:3`;
  exec with `-i` fails loudly (`exitFunc(1)`, `TTY` in stderr);
  top prints titles; stats prints one snapshot; inspect prints raw JSON.
  Capture stdout/stderr by reassigning `os.Stdout`/`os.Stderr` to pipes
  (check `exit_test.go` helpers first — reuse if present, do not duplicate).
- [ ] **Step 2: run, watch fail** — `go test ./cmd/ -run TestRemote -v`
  (Linux CI; Windows dev: `GOOS=linux go vet ./cmd/`). Expected: FAIL,
  undefined `execRemote` etc.
- [ ] **Step 3: implement.** `cmd/exec.go` — after the usage check, before
  `container.Load`, insert:

```go
if host := remoteHostResolved(); host != "" {
    execRemote(host, remaining[0], remaining[1:], interactive, tty)
    return
}
```

```go
// execRemote runs a non-interactive command through `cardinal serve`.
// Interactive/TTY sessions need a PTY and fail loudly (as logsRemote does
// for -f); the server 400s them as well, this check saves a round trip.
func execRemote(host, id string, cmd []string, interactive, tty bool) {
    if interactive || tty {
        failf("remote %s: interactive exec (-i/-t) needs a TTY; use local `cardinal exec` or the wings terminal", host)
        return
    }
    c := client.NewClient(host, remoteTokenResolved())
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    res, err := c.Exec(ctx, id, cmd)
    if err != nil {
        failf("remote %s: %v", host, err)
        return
    }
    fmt.Print(res.Output)
    if res.Stderr != "" {
        fmt.Fprint(os.Stderr, res.Stderr)
    }
    if res.ExitCode != 0 {
        exitFunc(res.ExitCode)
    }
}
```

`cmd/top.go` — at the top of `Top`, before `container.Load`:

```go
if host := remoteHostResolved(); host != "" {
    topRemote(host, args[0])
    return
}
```

`topRemote` prints a tabwriter table (`TITLE...` header + one row per
process string, same 40-char truncation style as `psRemote`).

`cmd/stats.go` — remote polls the one-shot endpoint (server has no usable
remote stream): `statsRemote(host, names, noStream)` loops `c.Stats` per
container every 1 s (converts `client.Stats` → `container.ContainerStats`
field-by-field, then reuses `container.PrintContainerStats(s, prev, hdr)`),
prints once and returns when `noStream`, otherwise until SIGINT
(`signal.Notify`, same pattern as `cmd/events.go:32-33`). Container list
for the no-arg form comes from `c.ListContainers(ctx, false)` mapped to
names. On first-iteration error → `failf`.

`cmd/inspect.go` — remote branch per name: `c.Inspect` prints the body
as-is (serve returns Docker-style inspect JSON, which differs from the
local state dump — document in README, do not reshape). `--sensitive`
is accepted but noted as a no-op remotely (server schema carries no
cardinal secrets).

- [ ] **Step 4: run tests** — same command. Expected: PASS.
- [ ] **Step 5: vet** — `GOOS=linux go vet ./cmd/` exit 0.

### Task 4: server SSE /events

`container.SubscribeEvents`/`UnsubscribeEvents` already exist, but history
replay needs an exported accessor (`getEventsSince` is lowercase). Add one
in the same task:

```go
// EventsSince returns buffered history after since (exported for the API
// SSE handler; the in-memory ring is capped at 1000 entries).
func EventsSince(since time.Time) []Event {
    return getEventsSince(since)
}
```

**Files:**
- Create: `internal/api/events.go` (`//go:build linux`, package `api`)
- Modify: `internal/container/events.go` (append `EventsSince` wrapper)
- Modify: `internal/api/server.go` (one route line next to `/metrics`)
- Test: `internal/api/api_test.go` (append cases)

`container.SubscribeEvents`/`UnsubscribeEvents` already exist;
`getEventsSince` covers replay. Handler `GET /events?since=RFC3339`:

```go
func handleEvents(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, 405, "method not allowed")
        return
    }
    since := time.Time{}
    if s := r.URL.Query().Get("since"); s != "" {
        if t, err := time.Parse(time.RFC3339, s); err == nil {
            since = t
        }
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.WriteHeader(200)
    flusher, ok := w.(http.Flusher)
    if !ok {
        return
    }
    flusher.Flush()
    for _, evt := range container.EventsSince(since) { // replay
        if err := writeSSE(w, evt); err != nil { return }
    }
    flusher.Flush()
    ch := container.SubscribeEvents(100)
    defer container.UnsubscribeEvents(ch)
    for {
        select {
        case <-r.Context().Done():
            return
        case evt := <-ch:
            if err := writeSSE(w, evt); err != nil { return }
            flusher.Flush()
        }
    }
}
```

(`writeSSE`: `json.Marshal(evt)` → `w.Write("data: "+...+"\n\n")`.)
Note: `UnsubscribeEvents` closes the channel — never read after return.

- [ ] **Step 1: failing test** — `TestHandleEventsSSE`: build request
  `GET /events`, call handler in goroutine with `httptest.NewRecorder()`
  + `Flusher` (ResponseRecorder implements it), `container.EmitEvent`
  with `&container.Container{ID:"a",Name:"w"}` after setting
  `CARDINAL_DATA_DIR` to `t.TempDir()` (else `persistEvent` touches the
  real state dir — check `internal/state` for the env name first),
  cancel request context, assert body contains `data:` + `"type"`.
- [ ] **Step 2: run, watch fail** — undefined `handleEvents`. Expected FAIL.
- [ ] **Step 3: implement** handler + route
  `mux.HandleFunc("/events", handleEvents)` in `server.go` after `/metrics`.
- [ ] **Step 4: run** — PASS. **Step 5: vet** clean.

### Task 5: CLI remote events

**Files:**
- Modify: `cmd/events.go`
- Test: `cmd/remote_exec_test.go` (append cases)

- [ ] **Step 1: failing test** — stub SSE server emitting two `data:`
  frames; set `remoteHost`; call `Events([]string{"--since", ...})`;
  assert both JSON lines on stdout and return after server close.
- [ ] **Step 2: run, watch fail** — Expected FAIL (no remote branch).
- [ ] **Step 3: implement** — top of `Events`, after flag parse:

```go
if host := remoteHostResolved(); host != "" {
    eventsRemote(host, since)
    return
}
```

`eventsRemote` mirrors the local signal handling: `ctx, stop :=
signal.NotifyContext(context.Background(), syscall.SIGINT,
syscall.SIGTERM)`, `events, errs := client.StreamEvents...` — exact
client signature from Task 2 (if Task 2 chose
`StreamEvents(ctx) (<-chan Event, <-chan error)`, range over events,
apply the same `--since` filter + `json.NewEncoder(os.Stdout).Encode`,
print `Listening for events... (remote <host>)` to stderr, return on
`ctx.Done()`/channel close/errs (errs → `failf`). Keep the local `--since`
parse formats (`time.RFC3339`, `"2006-01-02 15:04:05"`).
- [ ] **Step 4: run** — PASS. **Step 5: vet** clean.

### Task 6: docs (README remote section)

**Files:**
- Modify: `README.md`

- [ ] Find the current remote-flags mention
  (`grep -rn CARDINAL_REMOTE_HOST README.md docs/en/ docs/ru/`).
- [ ] Extend it into a short `Remote mode` block: newly supported
  `exec`/`stats`/`top`/`inspect`/`events` with one example each;
  explicit limits list: no `-i/-t` exec, no `logs -f`, remote `stats`
  polls `?stream=0` 1/s, remote `inspect` is Docker-schema (differs from
  local dump), `events --since` filters client-side from subscribe time.
- [ ] Verify: `sh scripts/sync-docs-version.sh --check` exit 0 (markers
  untouched, version strings unchanged).
