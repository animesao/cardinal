<!-- dck-version:start -->
**Documentation version:** `1.24.12`
**Project release:** `v1.24.12`
<!-- dck-version:end -->

# FaaS / Serverless

dck can deploy container images as serverless functions with auto-scaling
and scale-to-zero.

## Quick start

```bash
# Deploy a function
dck fn deploy \
  --name hello \
  --port 8080 \
  --timeout 30 \
  --idle 300 \
  --warm 1 \
  -e GREETING=world \
  ghcr.io/myorg/hello-func:latest

# Invoke it
dck fn call hello --data '{"name": "dck"}'

# List functions
dck fn ls

# Remove
dck fn rm hello
```

## Commands

### `dck fn deploy`

Deploy a container image as a serverless function.

| Flag | Default | Description |
|---|---|---|
| `--name` | — | Function name (required) |
| `--port` | `8080` | Port the function listens on inside the container |
| `--handler` | `/handler` | Path to the handler binary/script inside the container |
| `--timeout` | `30` | Max execution time per invocation (seconds) |
| `--idle` | `300` | Seconds of inactivity before scale-to-zero |
| `--warm` | `0` | Number of warm replicas to keep always ready |
| `--memory` | — | Memory limit per invocation (`128m`, `512m`, `1g`) |
| `--cpus` | — | CPU limit per invocation |
| `-e, --env` | — | Environment variables (can repeat) |

```
dck fn deploy --name hello --port 8080 --timeout 30 --idle 300 myfunc
dck fn deploy --name worker --timeout 120 --memory 1g --cpus 2 worker-image
dck fn deploy --name api --port 3000 --warm 3 api-image
```

### `dck fn ls`

List all deployed functions.

```
NAME   IMAGE       PORT  TIMEOUT  IDLE   WARM  INVOKES
hello  myfunc:lat  8080  30s      300s   1     42
worker worker-img  0     120s     300s   0     7
```

### `dck fn rm <name> [<name>...]`

Remove a deployed function (scales down all active containers first).

```
dck fn rm hello
```

### `dck fn call <name> [--data <payload>]`

Invoke a function. Payload can be passed inline or via stdin.

```
dck fn call hello --data '{"key":"value"}'
echo '{"key":"value"}' | dck fn call hello
```

## Auto-scaling lifecycle

```
  idle          invoke           idle > timeout       idle
  ──────► [scale-up] ──────► [running] ──────────► [scale-to-zero]
             │                                       │
             └── warm replicas skip this◄─────────────┘
```

1. **Warm**: if `--warm N` is set, N containers stay running
2. **Cold start**: on first request, a container starts from scratch
3. **Scale-to-zero**: after `--idle` seconds with no requests, containers are stopped
4. **Re‑invocation**: triggers a new scale-up if no warm replicas are available

## Use cases

### HTTP API

```bash
dck fn deploy --name users-api --port 3000 --warm 3 myorg/users-api
```

Always 3 warm instances ready; scales up on traffic spikes,
scales down to 3 after idle period.

### Worker / Queue consumer

```bash
dck fn deploy --name email-worker --timeout 120 --memory 1g myorg/email-worker
```

Cold-start per invocation. Scales to zero between messages.

### Webhook handler

```bash
dck fn deploy --name github-webhook --port 9000 --timeout 15 --idle 60 myorg/webhook
```

Quick invocation (15s timeout), scales to zero after 1 minute of idle.

## Function runtime contract

The container should:
1. Listen on HTTP port (`--port`)
2. Accept POST requests
3. Read request body as invocation payload
4. Return response body as function result

Environment variables available at runtime:

| Variable | Description |
|---|---|
| `DCK_FN_NAME` | Function name |
| `DCK_FN_TIMEOUT` | Max execution time |
| `DCK_INVOCATION_ID` | Unique invocation ID |
