<!-- cardinal-version:start -->
**Documentation version:** `2.4.6`
**Project release:** `v2.4.6`
<!-- cardinal-version:end -->

# cardinal — Command verification map

This is the per-command audit map. For every top-level command, we list
the runtime requirements (kernel features, registry network access,
state directory), the verification path that proved the command works
in the audited revision, and any caveat operators should know about.

The "verification" column is one of:

| Code | Meaning |
|---|---|
| `cobra` | Only structural — the cobra tree registers the command and renders --help. |
| `unit-test` | A unit test in `*_test.go` exercises the parser / policy / helpers. |
| `e2e-linux` | Verified on a Linux runner (CI: `.github/workflows/e2e.yml`). |
| `audit` | Confirmed by `scripts/audit.sh` static checks. |
| `audit-by-design` | Deliberately not exercised — see notes in `docs/AUDIT.md` (e.g. composer-ASCII examples, force-on-purpose warnings). |

If you want to verify a command locally on Linux you only need the kernel
modules listed under **Requires**. Everything else is plain Go that
invokes the kernel through `syscall`/`exec` like any container engine.

## Top-level commands

| Command | Requires | Verification | Notes |
|---|---|---|---|
| `pull` | registry network, overlayfs mount | `cobra`, `audit` | Allowlist via `CARDINAL_REGISTRY_STRICT=1`. |
| `push` | registry network, overlayfs | `cobra`, `audit` | Blocked by same allowlist. |
| `run` | namespaces, cgroup v2, overlayfs, iptables | `cobra`, `unit-test` (cap-root gate, run-flag reorder) | Dangerous-cap/root gated by `--allow-*` flags. |
| `exec` | nsenter, ps/pgrep | `cobra`, `e2e-linux` (smoke in `e2e.yml`) | Verifies namespace IDs. |
| `start` / `stop` / `restart` | supervisor, cgroup | `cobra`, `e2e-linux` | restart guard prevents crash loops. |
| `rm` | supervisor (auto-clears tombstone) | `cobra`, `e2e-linux` | New restart-tombstone guard in `start.go`. |
| `rename` | state dir + supervisor reload | `cobra` | |
| `set` | supervisor, cgroup | `cobra` | |
| `ps` | state dir | `cobra`, `e2e-linux` | |
| `inspect` | state dir | `cobra`, `e2e-linux` | `--sensitive` reveals mount sources. |
| `logs` | container runtime dir + nsenter | `cobra`, `e2e-linux` | |
| `stats` | cgroup v2 stat files + fs tree walk | `cobra` | cgroup reads plus disk tree-walks cached 60s; live ANSI redraw 1/s; `--no-stream` one-shot (CPU always 0.0%). |
| `top` | nsenter+ps | `cobra` | |
| `info` | `/proc` | `cobra`, `e2e-linux` | |
| `images` | state dir | `cobra`, `e2e-linux` | |
| `verify` | state dir | `cobra` | Offline digest check. |
| `rmi` | state dir | `cobra` | |
| `commit` | container runtime | `cobra` | Creates OCI image from running container. |
| `build` | overlayfs, namespaces, registry on pull-step | `cobra`, `unit-test` (path-traversal, fuzz-parser) | Dockerfile COPY bounds the build context. |
| `export` | state dir | `cobra` | Produces tar.gz with valid OCI layout. |
| `import` | state dir | `cobra`, `unit-test` (hardlink, NUL byte, symlink-loop, deep-traversal) | Rejects dangerous tar entries. |
| `search` | registry hub | `cobra` | Read-only. |
| `login` / `logout` | state dir + optional registry | `cobra`, `unit-test` (host-credential exact-match fix) | Credentials stored 0600. |
| `events` | state dir | `cobra` | |
| `port` | iptables | `cobra` | Add/remove/list mappings. |
| `network` | iptables, bridge | `cobra` | User-defined networks. |
| `volume` | mount utilities | `cobra`, `unit-test` (`IsProtectedHostPath`) | Bind-mount host-path blocklist. |
| `fs` | nsenter + tar | `cobra` | ls/cat/tree/find inside container. |
| `cp` | nsenter + tar | `cobra`, `e2e-linux` | Used in e2e smoke. |
| `backup` | tar, optional AES-256-GCM | `cobra`, `unit-test` (round-trip, scheduler, encryption) | Nine verbs: create/list/ls/restore/enable/disable/status/verify/remove (+rm/delete aliases)/generate-key; restore supports --rebind. |
| `cluster` | cluster state dir + multicast | `cobra` | Pure-Go; runs locally without external daemons. |
| `fn` | FaaS state dir | `cobra`, `unit-test` (FaaS rules) | Function-as-a-Service. |
| `service` | supervisor + healthcheck | `cobra`, `unit-test` | Long-running services. |
| `blueprint` | registry (blueprints repo) | `cobra` | Canned application recipes. |
| `up` | state dir + overlayfs + cgroup | `cobra`, `unit-test` (compose parser) | Docker-Compose parse. |
| `down` | supervisor | `cobra` | Reverse of up. |
| `serve` | HTTP listener | `cobra`, `unit-test` (rate limit, metrics gate, isExternalHost) | Bearer-token required off-loopback. |
| `console` / `console-serve` | interactive exec | `cobra` | `console` opens a local interactive shell; `console-serve` (hidden) serves attach connections. |
| `attach` | nsenter+exec | `cobra`, `e2e-linux` | Attach stdin/stdout to running process. |
| `supervisor` | systemd-timer or background loop | `cobra`, `e2e-linux` | Persistent restart policy enforcement. |
| `bootstrap` | systemd-only Linux | `cobra` | Install/uninstall systemd unit. |
| `doctor` | `/proc` | `cobra`, `e2e-linux` | Read-only host check. |
| `security` | `/proc` + config | `cobra` | `security check` runs the security-focused subset of doctor. |
| `system` | state dir | `cobra` | `prune` removes stopped containers/unused images/orphan overlays+logs (never volumes); `df` reports Images/Containers/Overlay/Volumes/Logs/Backups/Cache. |
| `update` | network + GitHub Releases | `cobra`, `unit-test` (cosign verification path) | SHA256 mandatory; cosign opt-in via `CARDINAL_REQUIRE_SIGNATURE=1`. |
| `init` (alias `init-container`) | namespaces | `cobra` | Internal helper used by `run`. (`helper-mount` is hidden entirely.) |
| `registry` | state dir | `cobra`, `unit-test` (allowlist default/perm/strict/insecure) | Allowlist + credential management. |
| `version` | nothing | `cobra`, `e2e-linux` | Plain text version. |

## Cobra scaffold (auto-included)

| Command | Requires | Verification | Notes |
|---|---|---|---|
| `completion bash\|zsh\|fish\|powershell` | nothing | `cobra`, `unit-test` | Generated by cobra; ships with the binary. |
| `--log-level`, `--json`, `--quiet` | nothing | `cobra`, `unit-test` | Global persistent flags set in `applyLogOptions`. |

## Sub-command surface

`cardinal backup` advertises: create / list / ls / restore / enable / disable /
status / verify / remove / generate-key (plus `rm`/`delete` aliases and
`restore --rebind`). Each sub-command is also a `register(commandSpec{...})`
entry — they appear in `cardinal backup --help` in alphabetical order.

`cardinal security` advertises: check.

`cardinal completion` advertises: bash / zsh / fish / powershell.

`cardinal registry` advertises: allowlist (list/add/remove) and forwards
`login` / `logout` to the existing pre-cobra implementations.

## How to verify on your own host

1. `make audit` — runs `scripts/audit.sh` (PASS / WARN / FAIL).
2. `go test ./... -count=1` on Linux — runs unit + cobra smoke tests.
3. `make test-race` — runs the same suite with `-race`.
4. `.github/workflows/e2e.yml` on a Linux runner with sudo — runs the e2e smoke workflow.
5. `make fuzz` for 30 seconds — exercises the Dockerfile parser.

If a command is missing from this matrix, please open an issue or update
this map after adding new functionality.
