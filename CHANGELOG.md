# Changelog

## Unreleased

### Added

- Added configurable automatic restart delays with `--restart-delay` / `dck set --restart-delay` (for example, `10s` or `1m`).

## 1.22.19 (2026-08-11)

### Documentation

- Added complete English and Russian running guides covering installation, image/tag syntax, protected bind mounts, `.env` files, Python bots, Java/Minecraft servers, logs, restart policies, updates, and troubleshooting.
- Updated the README, documentation index, command reference, storage notes, and version references.

### Runtime fixes documented

- Container dck stdout/stderr logs start fresh on every new container run instead of accumulating across restarts.
- OCI extraction accepts forward symlink targets and standard root entries while preserving traversal protection.
- Bind mounts reject protected host paths and require absolute container targets.

## 1.22.7 (2026-08-06)

- Isolated tests with `DCK_DATA_DIR` and made JSON state writes atomic.
- Secured API defaults, Bearer authentication, and image metadata storage.
- Standardized Go 1.25 across CI and release tooling.

## 1.22.4 (2026-07-14)

- CI auto-bump after install script fix
- Single `VERSION` source of truth (removed `cmd/VERSION`)
- Removed `//go:embed VERSION`, injected via `-X dck/cmd.version` only

## 1.22.3 (2026-07-14)

- CI auto-bump after release.yml fix

## 1.22.2 (2026-07-14)

- CI auto-bump

## 1.22.1 (2026-07-14)

- CI auto-bump after first 1.22.0 release

## 1.22.0 (2026-07-14)

### Features
- **Cluster orchestration**: `dck cluster init/join/leave/info/ls/node` — multi-node container orchestration
- **Services**: `dck service create/ls/rm/scale/update` — replicated services with rolling updates
- **FaaS / Serverless**: `dck fn deploy/ls/rm/call` — auto-scaling serverless functions with scale-to-zero
- **Blueprints**: `dck blueprint list/info/install` + `blueprint repo add/remove/list` — pre-configured templates
- **Docker-compatible REST API**: `dck serve [-d] [--token]` — works with Portainer, VS Code Dev Containers
- **Compose secrets & configs**: Docker-style secret/config injection via `dck.toml` / `compose.yaml`
- **Container events**: `dck events [--since <time>]` — real-time lifecycle event streaming
- **Dynamic port management**: `dck port add/rm` — hot-add/remove port mappings without restart
- **Container FS browser**: `dck fs ls/cat/tree/find` — browse stopped/running container filesystems
- **Healthchecks**: `--healthcheck-*` flags — auto-restart on failure
- **Startup scripts**: `--startup` flag with `@file` support, `DCK_*` env vars injection
- **Named volumes**: `dck volume create/ls/rm/inspect/prune`
- **Container export/import**: `dck export/import` — save and load container images
- **Registry auth**: `dck login/logout` — authenticated registry access + `dck push`

### Improvements
- Rootless mode support (`internal/container/rootless.go`)
- DNS service discovery for cluster (UDP 5353)
- Systemd bootstrap auto-install on `dck run --restart always`
- `dck set` now supports `--memory`, `--cpus`, `--disk`, `--restart`, `--workdir`, `-e`, `--entrypoint`, `--user`, `--readonly`, `--no-new-privs`, `-h`, `--network`
- `dck up --generate` — generate dck.toml from existing containers
- Multi-arch image resolution (`--platform`)
- cgroups v2 resource limits for CPU, memory, disk

## 1.21.0 (2026-07-01)

### Features
- Container commit: `dck commit <container> <image>:<tag>`
- `dck build` — Dockerfile builder with `--no-cache`, `--build-arg`, multi-stage support
- `dck system prune` — cleanup unused containers and images
- `dck stop --all` — stop all running containers
- `dck exec -i/-t` flags properly handled
- `dck console` — auto-detect shell inside container
- Improved attach with Unix socket (Ctrl+C safe)

## 1.20.0 (2026-06-20)

### Features
- Dynamic port management: `dck port add/rm` — hot-add/remove iptables DNAT rules
- Russian (ru) documentation mirror
- `--ulimit` support in run flags

### Bug Fixes
- Fixed overlay mount ordering for disk limits
- Fixed `dck exec` TTY handling

## 1.19.0 (2026-06-17)

### Features
- Container FS browser: `dck fs ls/cat/tree/find`
- `--healthcheck-*` flags with auto-restart on failure
- `--startup` flag with `@file` inline script support
- `DCK_*` environment variables injected for startup scripts

## 1.18.0 (2026-06-15)

### Features
- `dck events` — real-time container event streaming
- `dck volume create/ls/rm/inspect/prune` — named volume management
- `dck export/import` — save/load images as tar.gz
- `dck login/logout` — authenticated registry access
- Multi-arch image resolution with `--platform` flag

### Security
- Rootless mode support (experimental)
- No-new-privs flag support

## 1.17.0 (2026-06-24)

### Code Quality
- **Dead code removed**: `internal/container/rcon.go` — неиспользуемый RCON протокол
- **State tests**: 12 unit-тестов для `internal/state` (пути, JSON, FileExists)

### Bug Fixes
- **dck-wings**: Исправлен баг валидации container ID — `/` блокировал все action-запросы (start/stop/restart)

## 1.15.0 (2026-06-13)

### Security
- **pivot_root** вместо chroot — исправлен container escape vector
- Build tags (`//go:build linux`) добавлены на Linux-only файлы

### Bug Fixes
- **Disk limit**: исправлен path mismatch — overlay монтируется в правильную merged директорию
- **Race condition StoppedByUser**: `sync.Map` для атомарного флага между Stop() и monitor goroutine
- **Error swallowing**: логируются ошибки cgroup, mount, network, kill, overlay
- **Context/Timeout**: volume и network команды теперь с 30s timeout через `exec.CommandContext`
- **RCON**: мёртвый код помечен комментарием

### Features
- **`dck stop --all`**: остановка всех запущенных контейнеров
- **`dck exec -i/-t`**: флаги парсятся и применяются корректно
- **`dck console`**: TTY handling через ExecOpts

### Maintenance
- Debian control version синхронизирована с VERSION (1.15.0)

## 1.14.0 (Previous)
- DiskLimit support + loop device quota enforcement
- dck run --disk human-readable format
- Fix overlay mount ordering
- Multi-arch image resolution
