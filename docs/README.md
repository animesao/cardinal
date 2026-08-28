<!-- cardinal-version:start -->
**Documentation version:** `2.0.6`
**Project release:** `v2.0.6`
<!-- cardinal-version:end -->

<p align="center">
  <img src="../img/cardinal.png" alt="cardinal logo" width="150">
</p>

> Version markers are generated from the root `VERSION` file. Run `make docs` to update all Markdown files, or `make docs-check` in CI to verify synchronization.

# cardinal Documentation

**cardinal** — ~5 MB static binary, zero daemon. Drop-in container runtime for Linux.

```
cardinal pull alpine               cardinal pull nginx:alpine
cardinal run -d -p 80:80 nginx     cardinal run -i -t --rm alpine sh
cardinal up                        cardinal cluster init
cardinal serve                     cardinal fn deploy --name hello myfunc
```

Version: see the root [`VERSION`](../VERSION) file — [GitHub](https://github.com/animesao/cardinal)

## Start here

- [Complete CLI command reference (English)](en/commands.md)
- [Полный справочник CLI (русский)](ru/commands.md)
- [Command examples (English)](en/examples.md)
- [Примеры команд (русский)](ru/examples.md)
- [Contributing guide](../CONTRIBUTING.md)
- [Security model and vulnerability reporting](../SECURITY.md)
- [Running guide (English)](en/running.md)
- [Руководство по запуску (русский)](ru/running.md)

The running guides cover installation, image/tag syntax, bind mounts, `.env`, Python bots, Java/Minecraft servers, logs, restart policies, automatic backups, updates, and troubleshooting. The build guide documents local checks, cross-compilation, CI matrix behavior, and release automation.

## CLI Command Reference

### Image Management
| Command | Description |
|---------|-------------|
| `cardinal pull <image>[:tag]` | Pull image from registry |
| `cardinal push <image>[:tag]` | Push image to registry |
| `cardinal images` | List local images |
| `cardinal search <term>` | Search Docker Hub |
| `cardinal rmi <image>[:tag]` | Remove image |
| `cardinal verify <image>[:tag]` | Verify image config and layer digests |
| `cardinal commit <container> <image>[:tag]` | Create image from container |
| `cardinal build -t name:tag .` | Build from Dockerfile |
| `cardinal export <image> -o file.tar.gz` | Save image to file |
| `cardinal import <file.tar.gz>` | Load image from file |
| `cardinal login <registry>` | Log in to registry |
| `cardinal logout <registry>` | Log out |

### Container Lifecycle
| Command | Description |
|---------|-------------|
| `cardinal run [opts] <image> [cmd]` | Create and run container |
| `cardinal start <container>` | Start stopped container |
| `cardinal stop <container>` | Stop running container |
| `cardinal restart <container>` | Restart container |
| `cardinal rm [-f] <container>` | Remove container |
| `cardinal rename <c> <new-name>` | Rename container |
| `cardinal set <c> [opts]` | Change container params |
| `cardinal ps [-a]` | List containers |
| `cardinal backup create/list/restore/enable/disable/status/verify` | Manual and scheduled container backups |
| `cardinal init` | Internal container init |

### Monitoring & Logs
| Command | Description |
|---------|-------------|
| `cardinal logs [-f] [--tail <n>] <c>` | Show/follow/tail logs |
| `cardinal stats [container] [--no-stream]` | CPU, RAM, IO stats (live or one-shot) |
| `cardinal top <container>` | Running processes |
| `cardinal info` | System-wide info |
| `cardinal doctor [--strict]` | Read-only host/runtime diagnostics |
| `cardinal security check [--strict]` | Security-focused diagnostics |
| `cardinal events` | Stream container events |

### Network
| Command | Description |
|---------|-------------|
| `cardinal port <container>` | Show port mappings |
| `cardinal port add <c> H:C[/p]` | Add port mapping (hot) |
| `cardinal port rm <c> H[/p]` | Remove port mapping (hot) |

### Filesystem
| Command | Description |
|---------|-------------|
| `cardinal fs ls <c> [path]` | List files |
| `cardinal fs cat <c> <path>` | Show file content |
| `cardinal fs tree <c> [path]` | Directory tree |
| `cardinal fs find [c] [path] [opts]` | Find files |
| `cardinal cp <src> <dst>` | Copy files host↔container |

### Execution
| Command | Description |
|---------|-------------|
| `cardinal exec <c> <cmd>` | Run command in container |
| `cardinal console <c>` | Web terminal |
| `cardinal console-serve` | Console server (internal) |
| `cardinal attach <c>` | Attach to main process |

### Compose
| Command | Description |
|---------|-------------|
| `cardinal up [-f config] [service]` | Start containers from config |
| `cardinal up --generate` | Generate config from existing containers |
| `cardinal down [-f config] [-a] [service]` | Stop/remove from config |

### Volumes
| Command | Description |
|---------|-------------|
| `cardinal volume create <name>` | Create named volume |
| `cardinal volume ls` | List volumes |
| `cardinal volume rm <name>` | Remove volume |
| `cardinal volume inspect <name>` | Inspect volume |
| `cardinal volume prune` | Remove unused volumes |

### Cluster
| Command | Description |
|---------|-------------|
| `cardinal cluster init [--serve]` | Initialize cluster |
| `cardinal cluster join <peer> [--serve]` | Join cluster |
| `cardinal cluster join-token` | Show peer address |
| `cardinal cluster leave` | Leave cluster |
| `cardinal cluster info` | Cluster overview |
| `cardinal cluster ls` | List cluster nodes |
| `cardinal cluster node ls` | List nodes with resources |
| `cardinal cluster node inspect <id>` | Node details |
| `cardinal cluster serve [-p 2375]` | API server for replicas |

### Services
| Command | Description |
|---------|-------------|
| `cardinal service create ...` | Create replicated service |
| `cardinal service ls` | List services |
| `cardinal service rm <name>` | Remove service |
| `cardinal service scale <name> N` | Scale service |
| `cardinal service update <name>` | Rolling update |

### FaaS (Functions)
| Command | Description |
|---------|-------------|
| `cardinal fn deploy [--name <n>] <file>` | Deploy serverless function |
| `cardinal fn ls` | List functions |
| `cardinal fn rm <name>` | Remove function |
| `cardinal fn call <name>` | Invoke function |

### Blueprints
| Command | Description |
|---------|-------------|
| `cardinal blueprint list` | List available blueprints |
| `cardinal blueprint info <name>` | Show blueprint details |
| `cardinal blueprint install <name>` | Install a blueprint |
| `cardinal blueprint repo add <url>` | Add repository |
| `cardinal blueprint repo list` | List repositories |
| `cardinal blueprint repo remove` | Remove repository |

### System
| Command | Description |
|---------|-------------|
| `cardinal serve [-p 2375] [-H host] [-d] [--token <key>]` | Start REST API server (localhost by default; external bind requires Bearer token) |
| `cardinal serve --tls-cert cert.pem --tls-key key.pem` | Serve the API over HTTPS |
| `cardinal system prune` | Clean up unused resources |
| `cardinal update [--check]` | Self-update |
| `cardinal bootstrap [--install\|--remove]` | Install/start or remove the systemd supervisor |
| `cardinal version / --version / -v` | Show version |
| `cardinal help / --help / -h` | Show help |

## Run Options

| Flag | Description |
|------|-------------|
| `-d` | Detach (background) |
| `-n <name>` | Container name |
| `-p H:C[/p]` | Port mapping |
| `--ports H:C` | Port mapping (alias) |
| `-v S:D` | Volume mount |
| `--volume / --vol S:D` | Volume mount (alias) |
| `-e K=V` | Environment variable |
| `--env-file <f>` | Env from file (one per line, `KEY=VAL` or `export KEY=VAL`) |
| `-i` | Interactive (keep stdin) |
| `-t` | Allocate TTY |
| `--rm` | Auto-remove on exit |
| `--restart <policy>` | `no`, `always`, `on-failure`, `unless-stopped`; detached boot supervision applies to `always`/`unless-stopped` |
| `--restart-max-attempts <n>` | Crash-loop budget before automatic restart is blocked |
| `--restart-window <duration>` | Window used for the crash-loop budget |
| `--restart-delay <duration>` | Wait before automatic restart, e.g. `10s`, `1m` |
| `--memory / --ram <lim>` | Memory limit (e.g. `1g`, `512m`) |
| `--cpus / --cpu <num>` | CPU limit (e.g. `1.5`, `4`) |
| `--disk <lim>` | Disk limit (e.g. `10G`) |
| `--workdir <dir>` | Working directory |
| `-h <name>` | Container hostname |
| `--entrypoint <cmd>` | Override entrypoint |
| `--image <img>` | Image (instead of positional) |
| `--cmd / --command <cmd>` | Command (instead of positional) |
| `--cap-add / --cap-drop` | Linux capabilities |
| `--user <uid>` | UID or UID:GID |
| `--readonly` | Read-only rootfs |
| `--no-new-privs` | Explicitly request no-new-privileges (cardinal also enforces this security default) |
| `--sysctl <k=v>` | Sysctl options |
| `--ulimit <opt>` | Ulimit options |
| `-l / --label <k=v>` | Container labels |
| `--dns <ip>` | DNS server |
| `--network <mode>` | `bridge`, `none`, `host` |
| `--startup <s>` | Startup script (inline or `@file`) |
| `--healthcheck-cmd <cmd>` | Health check command |
| `--healthcheck-interval <s>` | Interval (seconds) |
| `--healthcheck-retries <n>` | Retries |
| `--healthcheck-timeout <s>` | Timeout (seconds) |

## Docs Index

| English | Русский |
|---------|---------|
| [Running Guide](en/running.md) | [Руководство по запуску](ru/running.md) |
| [Complete CLI Reference](en/commands.md) | [Полный справочник CLI](ru/commands.md) |
| [Command Examples](en/examples.md) | [Примеры команд](ru/examples.md) |
| [Usage & Commands](en/usage.md) | [Команды и использование](ru/usage.md) |
| [Deploying Websites](en/websites.md) | [Развёртывание сайтов](ru/websites.md) |
| [Bots (Telegram, Discord)](en/bots.md) | [Боты (Telegram, Discord)](ru/bots.md) |
| [Compose / Deployment](en/compose.md) | [Compose / Развёртывание](ru/compose.md) |
| [Compose Examples (15 configs)](en/compose-examples.md) | [Примеры Compose (15 конфигураций)](ru/compose-examples.md) |
| [Cluster Orchestration](en/cluster.md) | [Кластерная оркестрация](ru/cluster.md) |
| [FaaS / Serverless](en/faas.md) | [FaaS / Serverless](ru/faas.md) |
| [Build, CI & Versioning](build.md) | [Сборка, CI и версионирование](build.md) |

## Architecture

```
Storage: `$CARDINAL_DATA_DIR` (default: `/root/.cardinal/`)

images/        OCI rootfs per tag
containers/    State JSON files
overlay/       upper/work/merged per container
logs/          Container stdout/stderr (fresh on each new start)
volumes/       Named volumes
cache/         Cached image layers
consoles/      Unix sockets for attach

cardinal run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc cardinal init <id>
  │   └─ cardinal init → pivot_root to overlay → setup /proc/lo/eth0 → exec CMD
  └─ cardinal console-serve <id>
      ├─ reads stdout pipe
      ├─ writes to log file
      ├─ listens on Unix socket
      └─ broadcasts to all attach clients
```

## Quick Reference

```bash
cardinal pull nginx:alpine            # Pull image
cardinal run -d -n web -p 80:80 nginx # Detached web server
cardinal run -i -t alpine sh          # Interactive shell
cardinal ps                           # List running containers
cardinal logs web                     # View logs
cardinal stop web                     # Stop container
cardinal rm web                       # Remove container
cardinal up                           # Start from cardinal.toml
cardinal cluster init                 # Init cluster
cardinal fn deploy --name hello fn.py # Deploy function
```
