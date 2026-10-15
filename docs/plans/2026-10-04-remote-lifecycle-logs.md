<!-- cardinal-version:start -->
**Documentation version:** `2.3.11`
**Project release:** `v2.3.11`
<!-- cardinal-version:end -->

# Remote Lifecycle + Logs Follow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add remote `start`/`stop`/`restart` (incl. `stop --all`) and streaming `logs -f` to `cardinal --host/--token` mode.

**Architecture:** Lifecycle needs no server changes (endpoints exist; 204 + 304 both mean success). Logs follow needs a polling follow-loop in `handleContainerLogs` (ticker, no new deps), a `StreamLogs` client method on a timeout-less HTTP client (same pattern as `StreamEvents`), and a CLI branch replacing the loud `failf` on `-f`.

**Tech Stack:** Go 1.26, stdlib `net/http` + `httptest`, existing `DisableFlagParsing`/`extractRemoteFlags` wiring (no cobra changes — verify with grep).

---

## File map

| File | Role |
|---|---|
| `internal/client/client.go` | Add `Start`, `Stop`, `Restart`, `StreamLogs` |
| `internal/client/client_test.go` | Append lifecycle + chunked-stream cases |
| `cmd/start.go`, `cmd/stop.go`, `cmd/restart.go` | Add `*Remote` branches |
| `cmd/logs.go` | Replace remote `-f` failf with streaming |
| `cmd/remote_exec_test.go` | Append CLI remote cases |
| `internal/api/containers.go` | `handleContainerLogs`: follow-loop after tail |
| `internal/api/api_test.go` | Append follow test (temp `CARDINAL_DATA_DIR`) |
| `README.md` | Document lifecycle + `logs -f`, limits |

Non-goals: remote `kill`/`remove`, remote `--previous`/`--all` (need new rotated-log endpoints), server-side stats streaming, TTY.

---

### Task 1: client lifecycle (Start/Stop/Restart)

**Files:** Modify `internal/client/client.go`, `internal/client/client_test.go`

Server contract (already shipped, `internal/api/containers.go:586-647`):
`POST /containers/{id}/start|stop|restart` → `204` on change, `304` when
already in that state. Both are success (`doReq`/`doPost` only fail on
`>= 400`). `doPost(ctx, path, nil, nil)` sends a `null` JSON body which the
handlers ignore — no `doPost` changes needed.

Exact signatures (Task 2 depends on them):

```go
func (c *Client) Start(ctx context.Context, id string) error
func (c *Client) Stop(ctx context.Context, id string) error
func (c *Client) Restart(ctx context.Context, id string) error
```

Each is one line: `return c.doPost(ctx, "/containers/"+id+"/start", nil, nil)`
(resp. `/stop`, `/restart`).

- [ ] **Step 1: extend `testServer` mux** with
  `POST /containers/web/start|stop|restart` returning `204` (assert method
  is POST; return `304` for one extra id, e.g. `/containers/idle/stop`, to
  lock the no-op success path).
- [ ] **Step 2: add failing tests** `TestStartStopRestart` (204 paths +
  304 path returns nil error). Run `go test ./internal/client/ -v` —
  Expected: FAIL (undefined methods). This package is portable: must run
  on Windows/macOS.
- [ ] **Step 3: implement** the three methods in `client.go`.
- [ ] **Step 4: run** — same command. Expected: PASS, full package green.

### Task 2: CLI lifecycle branches

**Files:** Modify `cmd/start.go`, `cmd/stop.go`, `cmd/restart.go`; append `cmd/remote_exec_test.go`

- [ ] **Step 1: append failing CLI tests** to `cmd/remote_exec_test.go`
  (READ it first, reuse `setRemote`/`stubExit`/capture helpers; `//go:build
  linux` stays). Cases: `StartCmd(["web"])` prints short id (stub returns
  `204`); `Stop` on already-stopped id (stub returns `304`) succeeds;
  `Restart` succeeds; `Stop(["--all"])` stops each listed container (stub
  `/containers/json` + `POST .../stop`, assert two stop calls).
- [ ] **Step 2: run, watch fail** — `go test ./cmd/ -run TestRemote -v`
  (Linux CI; Windows: `GOOS=linux go vet ./cmd/`). Expected: FAIL.
- [ ] **Step 3: implement.** `cmd/start.go` — after the usage check, before
  `container.Load`:

```go
if host := remoteHostResolved(); host != "" {
    startRemote(host, args[0])
    return
}
```

```go
func startRemote(host, id string) {
    c := client.NewClient(host, remoteTokenResolved())
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    if err := c.Start(ctx, id); err != nil {
        failf("remote %s: %v", host, err)
        return
    }
    fmt.Println(shortID(id))
}
```

`stopRemote`/`restartRemote` are identical with `c.Stop`/`c.Restart`.
`cmd/stop.go` — inside the `--all` branch, when remote: list via
`c.ListContainers(ctx, false)`, stop each by name (TrimPrefix `/` like
`psRemote`), print short id per container, continue-on-error like local
(`Error stopping %s` to stderr). Single form mirrors `startRemote`.
`cmd/restart.go` mirrors `startRemote` with `c.Restart`. Add
`context`/`time`/`client` imports without duplicating.
- [ ] **Step 4: run** — PASS (CI). **Step 5: vet** — `GOOS=linux go vet
  ./cmd/` exit 0.

### Task 3: server logs follow

**Files:** Modify `internal/api/containers.go`; append `internal/api/api_test.go`

Current handler (`handleContainerLogs`, ~lines 792-834): reads the whole
file, applies `tail`, writes once, ignores `follow`. New behavior: after
the existing write, if `follow=1`, keep streaming appended bytes until the
client disconnects. Rotation-safe (a fresh `start` creates a new log):
track the sent offset against the on-disk size; `size < sent` means the
file was replaced → reset offset to 0. Open errors (deleted file between
restarts) reset the offset and keep waiting — never fail the stream.

- [ ] **Step 1: failing test** — append `TestHandleContainerLogsFollow` to
  `internal/api/api_test.go`: `t.Setenv("CARDINAL_DATA_DIR", t.TempDir())`,
  write initial bytes to `state.LogPath("w")`, call the handler in a
  goroutine with `follow=1` + cancellable context
  (`httptest.NewRequest` + `WithContext`), append more bytes mid-stream,
  cancel, assert the recorder body contains both chunks. Needs a real
  `&container.Container{ID: "w", Name: "w"}` (handler only reads `c.ID`
  for the path). Run `go test ./internal/api/ -run TestHandleContainerLogsFollow
  -v` (Linux CI; Windows: `GOOS=linux go vet`). Expected: FAIL (no follow).
- [ ] **Step 2: implement.** In `handleContainerLogs`, capture the full size
  before the existing tail cut (`sent := int64(len(data))` right after the
  file read), keep the snapshot write untouched, then append this loop
  (headers + `WriteHeader(200)` already sent — the loop only appends):

```go
if !follow {
    return
}
flusher, ok := w.(http.Flusher)
if !ok {
    return
}
ticker := time.NewTicker(500 * time.Millisecond)
defer ticker.Stop()
for {
    select {
    case <-r.Context().Done():
        return
    case <-ticker.C:
        fi, err := os.Stat(logPath)
        if err != nil {
            sent = 0
            continue
        }
        if fi.Size() < sent {
            sent = 0 // rotated: fresh log after `start`
        }
        if fi.Size() == sent {
            continue
        }
        f, err := os.Open(logPath)
        if err != nil {
            sent = 0
            continue
        }
        _, err = f.Seek(sent, io.SeekStart)
        if err == nil {
            var n int64
            n, err = io.Copy(w, f)
            sent += n
            flusher.Flush()
        }
        _ = f.Close()
        if err != nil {
            return
        }
    }
}
```

`sent` must be the FULL pre-tail size: capture `data` length before the
tail cut (restructure minimally: `full := data` then cut a copy). Check
imports (`os`, `io`, `time` — add what is missing, do not duplicate).
- [ ] **Step 3: run** — PASS (CI). **Step 4: vet** clean.

### Task 4: client StreamLogs

**Files:** Modify `internal/client/client.go`, `internal/client/client_test.go`

Exact signature (Task 5 depends on it):

```go
func (c *Client) StreamLogs(ctx context.Context, id string, tail int, w io.Writer) error
```

GET `/containers/{id}/logs?tail=N&follow=1` (omit `tail` when `<= 0`),
`io.Copy(w, resp.Body)` until EOF, with the request built on a dedicated
`http.Client{Timeout: 0}` (same reason as `StreamEvents` — the shared
15 s `hc` would cut the stream). Reuse the `doReq` error mapping: refactor
it minimally so both GET paths share the status-code handling, or duplicate
the 4-line mapping inline — either is fine, do not break existing callers.
`client.go` needs an `io` import (check first, do not duplicate).

- [ ] **Step 1: extend `testServer` mux** with a chunked endpoint: write
  `chunk1\n`, `Flush()`, sleep 100 ms, write `chunk2\n`, return (handler
  must cast to `http.Flusher`). Register it as a separate path the new
  test calls (e.g. reuse `/containers/web/logs` only if the existing tail
  assertion still holds — safer: new path `/containers/stream/logs` and
  have the test call `StreamLogs` against a fixed id via a test-only base;
  simplest correct: extend the existing logs handler to emit two chunks
  with flush when `follow=1`, keep the tail assertion for `follow != 1`).
- [ ] **Step 2: add failing test** `TestStreamLogs` asserting the copied
  output contains both chunks. Run `go test ./internal/client/ -v` —
  Expected: FAIL (undefined method). Portable: must run on Windows.
- [ ] **Step 3: implement** `StreamLogs` in `client.go`.
- [ ] **Step 4: run** — same command. Expected: PASS, full package green.

### Task 5: CLI logs -f remote

**Files:** Modify `cmd/logs.go`; append `cmd/remote_exec_test.go`

- [ ] **Step 1: append failing test** to `cmd/remote_exec_test.go` (READ it
  first, reuse helpers): stub `/containers/web/logs` emitting two flushed
  chunks then closing; set remote; call `Logs([]string{"-f", "web"})`;
  assert stdout contains both chunks and the call returns after close
  (no signals needed — EOF ends the stream).
- [ ] **Step 2: run, watch fail** — `go test ./cmd/ -run TestRemote -v`
  (Linux CI; Windows: `GOOS=linux go vet ./cmd/`). Expected: FAIL.
- [ ] **Step 3: implement.** In `logsRemote`, replace the `follow` failf
  with streaming (keep the `previous`/`all` loud fail untouched):

```go
if follow {
    logsFollowRemote(host, id, tail)
    return
}
```

```go
// logsFollowRemote streams `cardinal serve` logs until the server closes
// the stream or the user hits Ctrl+C.
func logsFollowRemote(host, id string, tail int) {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    c := client.NewClient(host, remoteTokenResolved())
    if err := c.StreamLogs(ctx, id, tail, os.Stdout); err != nil {
        if ctx.Err() != nil {
            return // user interrupt: exit quietly, like local -f
        }
        failf("remote %s: %v", host, err)
    }
}
```

Add `os/signal`/`syscall`/`context` imports (check first, do not
duplicate; `context`/`time` may already be there from the logsRemote
task). Update the `logsRemote` doc comment (follow is now supported).
- [ ] **Step 4: run** — PASS (CI). **Step 5: vet** clean.

### Task 6: docs (README lifecycle + follow)

**Files:** Modify `README.md`

- [ ] Extend the `## Remote mode` block (added in the previous plan):
  `start`/`stop`/`restart` examples incl. `stop --all` (client-side loop),
  `logs -f` example; update the limits list: remove the `logs -f`
  ban, keep no `-i`/`-t` exec, remote `stats` polling, Docker-schema
  `inspect`, client-side `events --since`; add: `--previous`/`--all`
  stay local-only, follow survives rotation (offset reset) but ends if
  the container is removed. No version strings/markers touched.
- [ ] Verify: `sh scripts/sync-docs-version.sh --check` exit 0
  (`--check` ONLY — bare run rewrites line endings repo-wide).
