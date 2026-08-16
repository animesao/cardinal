<!-- dck-version:start -->
**Documentation version:** `1.24.15`
**Project release:** `v1.24.15`
<!-- dck-version:end -->

<p align="center">
  <img src="img/dck.png" alt="dck logo" width="100">
</p>

# Changelog

<!-- dck-current-release:start -->
> Current release: **v1.24.15**. Detailed release notes below are maintained manually.
<!-- dck-current-release:end -->

## 1.24.15 (2026-08-16)

### CI/CD

- Patch release: same code body as v1.24.14 with a refreshed tag so consumers can pin a green build of the goreleaser v2 release pipeline.

## 1.24.14 (2026-08-16)

### CLI

- Migrated the top-level dispatch from a hand-rolled switch / flag-parser in `cmd/root.go` to `spf13/cobra v1.8.1` with `spf13/pflag v1.0.5`. Every previously-working invocation (52 commands + 7 backup sub-commands + `security check`) is preserved as a thin cobra wrapper around the legacy free function, so existing tests and call sites stay green.
- New global flags on every sub-command: `--log-level debug|info|warn|error`, `--json`, `--quiet`.
- `dck completion` now generates shell scripts for **bash / zsh / fish / powershell** out of the box; installation instructions added to `docs/en/commands.md` and `docs/ru/commands.md`.
- Discovered and fixed a regression where `dck run --rm --network none alpine:latest …` exited silently with empty stderr. The legacy flag-parsing path inside `Run()` is reachable again via `DisableFlagParsing: true` on the `run` cobra command. A regression test (`TestRunCommand_DisablesFlagParsing`) guards against future removal of that knob.
- Surfaced cobra's runtime errors to stderr in `cmd/cobra.go` (`Execute()`), guaranteeing that any future unknown-flag / wrong-arity error is at least actionable instead of leaving a single newline and exit code 1.

### CI/CD

- Rebuilt the GitHub Actions pipeline around a single `build.yml` orchestrator that runs the full `lint → test → build matrix → SBOM → cosign → GitHub Release` flow. Replaces nine reusable workflows plus five orchestrators with three files: `build.yml`, `e2e.yml`, `scheduled.yml`.
- Pinned action versions to known-good majors (`actions/checkout@v4`, `actions/setup-go@v5`, `actions/upload-artifact@v4`, `anchore/sbom-action@v0`, `softprops/action-gh-release@v2`); dependabot now ignores semver-major bumps in `.github/dependabot.yml` so the pipeline does not silently flip to a breaking major.
- `build.yml` runs `golangci-lint`, `go vet`, `govulncheck`, `shellcheck`, `scripts/sync-docs-version.sh --check`, and `bash scripts/audit.sh --strict` on every push and PR; the build matrix produces reproducible `linux/amd64` and `linux/arm64` artefacts with `-trimpath -buildid=`, sha256 sidecars, SPDX-JSON SBOM, optional `cosign` signing, and a GitHub Release on tag pushes.
- `e2e.yml` (privileged) now always uploads a diagnostic bundle (`dck-e2e-diag-<run-id>`) with the captured stdout/stderr buffer, preflight dump of id / env / mount / cgroup / overlay / data-dir / df, and `if: always()` artifact upload so a future maintainer can post-mortem a failed run without re-executing it.
- `scheduled.yml` runs daily govulncheck + gitleaks + `scripts/audit.sh`; dependabot.yml groups minor+patch bumps and ignores major version updates.

### Documentation

- Added `docs/COMMANDS.md` mapping every cobra-registered command to its runtime requirements (kernel feature, network, state) and verification path (`cobra`, `unit-test`, `e2e-linux`, `audit`, `audit-by-design`).
- Expanded `SECURITY.md` with a deployment-checklist and a hardening section including: host-path blocklist (`internal/container/hostpath.go`), log-injection protection for line-delimiter / control-character payloads, registry allow-list, image authentication, and dangerous-capability gating (`--allow-dangerous-caps`, `--allow-root`).
- Added `docs/AUDIT.md` summarising every change above with a per-feature rationale and the known design debt.
- Added regression tests: Dockerfile parser fuzz (`internal/builder/fuzz_test.go`), `internal/image/allowlist_test.go`, `internal/log/logger_test.go`, `cmd/security_test.go`, OCI-layer traversal and hardlink/symlink-loop extraction.

### Removed

- Removed the unreachable `internal/ftp/` package (398 lines of plain-TCP FTP server with a single shared password). `SECURITY.md` records the decision; `scripts/audit.sh` carries a prevention check so future regressions are caught at audit time.

## 1.23.28 (2026-08-13)

### Runtime fixes

- Fixed `dck run` options placed after the image, including `--workdir`, being passed to the container command.
- Preserved the safe default Linux capability set so standard images such as `nginx:alpine` can perform required startup filesystem operations while dangerous capabilities remain dropped.
- Cleaned up container resources when the init process exits before namespace registration.

## 1.23.17 (2026-08-12)

### Documentation and release automation

- Added a single-source documentation version marker synchronized from `VERSION` across every Markdown file.
- Added `scripts/sync-docs-version.sh --check` and CI validation so stale documentation versions fail before release.
- Updated installation, AppImage, update, backup, verification, restart, and multi-architecture guidance in English and Russian.
- Removed unreliable third-party download mirror instructions and replaced unsafe manual binary replacement examples with `install -D`.
- Fixed duplicate help/bootstrap documentation entries.

## 1.23.0 (2026-08-11)

### Highlights since 1.22.0

- Persistent restart supervisor: detached `always` and `unless-stopped` containers are recovered after reboot or crash via systemd, with configurable `--restart-delay` and crash-loop protection (`--restart-max-attempts`, `--restart-window`, `restart_blocked`).
- Per-container scheduled backups with `dck backup enable/disable/status`, configurable intervals and retention, safe destinations, and checksum verification with `dck backup verify`.
- Offline image verification with `dck verify IMAGE[:TAG]` — config and layer digests are checked against the locally stored manifest.
- Reliable `dck update`: five-minute download timeout, per-method error reporting, bounded curl/wget fallbacks.
- Runtime hardening: zombie-exit detection, `dck rm` tombstones against supervisor restart races, safe OCI layer extraction (path traversal, absolute and symlink targets), protected bind sources, and volume mount modes (`:ro`/`:rw`, propagation, tmpfs, NFS).
- Instant startup for `--network none`/`host` containers (no eth0 wait).
- Docker-compatible REST API with optional HTTPS and bearer-token auth, cluster orchestration, FaaS, services, blueprints, and Compose support.
- Complete bilingual (EN/RU) documentation: command references, usage guides, practical examples, and per-application guides, including the pull → verify → run workflow and a fully synchronized Russian websites guide.

## 1.22.40 (2026-08-11)

### CLI

- `dck --help` no longer duplicates the backup, inspect, doctor, and security check entries in the System section (they remain listed under Container).

### Documentation

- Documented the previously undocumented `dck verify IMAGE[:TAG]` (offline config and layer digest verification) and `dck backup verify FILE.tar.gz` (checksum verification) commands, plus `-v SRC:DST` mount modes (`:ro`/`:rw`, propagation flags, tmpfs and NFS specs).

## 1.22.39 (2026-08-11)

### Documentation

- The README and running guides document crash-loop protection with `--restart-max-attempts` and `--restart-window` and the `restart_blocked` state.

## 1.22.38 (2026-08-11)

### Update reliability

- `dck update` now allows up to five minutes for the release download instead of timing out after ten seconds, which was too short for multi-megabyte binaries on slow links.
- The opaque `all methods failed` message is gone: each download method (Go HTTP client, curl, wget) now reports its own error so a failure can be diagnosed.
- The curl/wget fallbacks get explicit connect and max-time limits so the updater can never hang indefinitely.

## 1.22.37 (2026-08-11)

### Runtime fixes

- Container startup no longer spends up to 20 seconds polling for an `eth0` address. The wait is skipped entirely for `--network none` (no interface exists) and `--network host` (the host interface is already up), and capped at five seconds for bridge mode. Crash-loop restart cycles now run on schedule, and simple containers such as `sh -c sleep 5` start immediately after `dck run -d` returns.

## 1.22.36 (2026-08-11)

### Runtime fixes

- Detached container processes that become zombies (defunct, reparented to systemd after the CLI exits) are now detected as dead: process liveness reads `/proc/<pid>/stat` and treats `Z`/`X` states as exited. A plain `/proc/<pid>` existence check counted zombies as alive, stalling exit detection — containers stuck on `running`, cgroup/network cleanup not running, and crash-loop restarts almost never firing.
- `dck rm` writes a tombstone marker as its first action, so a supervisor automatic restart racing a slow removal can no longer resurrect a container mid-delete.
- The supervisor re-loads fresh container state before an automatic restart and skips containers being removed; `dck start` aborts cleanly (killing spawned processes and releasing the DNS record) if the state file vanished or a removal is in progress.

## 1.22.35 (2026-08-11)

### Documentation

- Added complete English and Russian CLI command references with syntax prefixes, positional arguments, aliases, every user-facing option, and internal-command notes.
- Added separate English and Russian practical command examples for applications, databases, bots, Minecraft, restart recovery, volumes, backups, Compose, registries, clusters, services, and functions.
- Linked the new references and examples from the README and documentation index.

### CI and release workflow

- CI runs Go tests only on the native amd64 runner; arm64 is cross-compiled without executing an incompatible test binary.
- The architecture matrix uses `fail-fast: false`, so one architecture cannot cancel the other job.
- Build & Release checks out the newly created version tag before building amd64, arm64, and armv6 artifacts.
- Build and manual release workflows are serialized to avoid concurrent version and tag updates.

### Added

- Added configurable automatic restart delays with `--restart-delay` / `dck set --restart-delay` (for example, `10s` or `1m`).
- Added a persistent systemd supervisor for detached `always` and `unless-stopped` containers; `on-failure` remains a foreground-process policy and is not adopted after the detached CLI exits.
- Added per-container scheduled backups with `dck backup enable/disable/status`, configurable intervals, retention, safe destinations, and supervisor-based recovery after reboot.

## 1.22.30 (2026-08-11)

### Documentation

- Added complete bilingual CLI references and command examples.
- Updated documentation navigation and current release references.

## 1.22.28 (2026-08-11)

### Documentation

- Updated English and Russian guides with the current version, automatic backups, supervisor behavior, restart delays, protected bind mounts, log rotation, and CI/build workflow.

## 1.22.26 (2026-08-11)

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
