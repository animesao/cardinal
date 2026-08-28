<!-- cardinal-version:start -->
**Documentation version:** `2.0.11`
**Project release:** `v2.0.11`
<!-- cardinal-version:end -->

# cardinal CLI Command Reference

This is the complete command reference for the Linux `cardinal` binary. It describes the command tree, positional arguments, short and long flags, aliases, defaults, and safety rules.

> **Platform:** cardinal runs on Linux and requires namespaces, OverlayFS, cgroups v2, `unshare`, `nsenter`, `mount`, `ip`, `iptables`, and `pgrep`.

## 1. Syntax and prefixes

```text
cardinal COMMAND [SUBCOMMAND] [OPTIONS] [POSITIONAL-ARGUMENTS]
```

| Form | Meaning | Example |
|---|---|---|
| `COMMAND` | Top-level command | `run`, `logs`, `volume` |
| `SUBCOMMAND` | Operation below a command | `volume create`, `backup enable` |
| `<value>` | Required positional value | `<container>`, `<image>` |
| `[value]` | Optional positional value | `[path]`, `[service]` |
| `-x` | Short option | `-d`, `-p 8080:80` |
| `--option` | Long option | `--restart unless-stopped` |
| `--option=value` | Long option with an inline value | `--memory=2g` |
| `--` | End options; following values are command arguments | `cardinal run alpine sh -c -- 'echo -n hi'` |

Short boolean options can be written separately (`-i -t`) or as a combined shorthand (`-it`, `-dit`) — cardinal normalizes combined boolean shorthands such as `-it`/`-dit` (for `run`) and `-it` (for `exec`) before parsing. Long options are generally not interchangeable unless this reference lists an alias. Quote values containing spaces, `$`, `*`, `:`, or shell syntax.

### Common aliases

- `ls` and `list` are aliases for list subcommands where noted.
- `rm` and `remove` are aliases for removal subcommands where noted.
- `-p` / `--ports` — port mapping in `run`.
- `-v` / `--volume` / `--vol` — volume mapping in `run`.
- `--memory` / `--ram` and `--cpus` / `--cpu` — resource aliases in `run` and `set`.
- `--cmd` / `--command` — command aliases in `run`.
- `-l` / `--label` — label aliases.
- `-v`, `--version`, and `version` — version display aliases; `-h`, `--help`, and `help` — help aliases.

## 2. Image and registry commands

### `cardinal pull [--platform os/arch] IMAGE[:TAG]`

Pull an image from Docker Hub or a configured registry. If no tag is given, the registry's `latest` tag is used. `--platform` selects a manifest platform such as `linux/amd64` or `linux/arm64`.

```bash
cardinal pull alpine
cardinal pull alpine:3.20
cardinal pull --platform linux/arm64 eclipse-temurin:21
```

### `cardinal push IMAGE[:TAG] [-u USER] [-p PASSWORD]`

Push a local image. The short credentials flags are `-u` and `-p`; prefer `cardinal login` or environment variables for automation.

```bash
cardinal push myapp:v1
cardinal push -u registry-user -p 'secret' registry.example.com/team/myapp:v1
```

### `cardinal images`

List locally stored images.

```bash
cardinal images
```

### `cardinal search TERM`

Search Docker Hub. A tag filter can be supplied after a colon.

```bash
cardinal search nginx
cardinal search python:3.12
```

### `cardinal rmi IMAGE[:TAG]`

Remove a local image. The default tag is `latest`.

```bash
cardinal rmi alpine:3.20
```

### `cardinal verify IMAGE[:TAG]`

Verify a local image's config and layer digests against its stored manifests.

```bash
cardinal verify alpine:3.20
```

### `cardinal commit CONTAINER IMAGE[:TAG]`

Create an image from a container's current writable state.

```bash
cardinal commit web myregistry/web:snapshot
```

### `cardinal build -t NAME[:TAG] [OPTIONS] [CONTEXT]`

Build an image from a Dockerfile. The context defaults to `.` and the tag is required.

| Option | Description |
|---|---|
| `-t NAME[:TAG]` | Required image name and tag |
| `-f FILE` | Dockerfile path; defaults to `<context>/Dockerfile` |
| `--no-cache` | Accepted for compatibility; builds currently do not reuse instruction results |
| `--build-arg KEY=VALUE` | Repeatable build-time variable |
| `--quiet` | Suppress build output |
| `--cpu N` | CPU limit for build work |
| `--memory BYTES` | Build memory limit in bytes |

```bash
cardinal build -t myapp:dev .
cardinal build -t myapp:prod -f Dockerfile.prod --build-arg VERSION=1.0 ./src
```

### `cardinal export IMAGE[:TAG] [-o FILE.tar.gz]`

Export an image to an archive. `-o` selects the output path.

```bash
cardinal export myapp:v1 -o /data/images/myapp-v1.tar.gz
```

### `cardinal import FILE.tar.gz [FILE.tar.gz ...]`

Import one or more image archives.

```bash
cardinal import /data/images/myapp-v1.tar.gz
```

### `cardinal login REGISTRY [-u USER] [-p PASSWORD] [--password-stdin]`

Save registry credentials. Without credentials, cardinal prompts interactively. `--password-stdin` reads the password from standard input.

```bash
cardinal login registry.example.com
echo "$REGISTRY_PASSWORD" | cardinal login registry.example.com -u "$REGISTRY_USER" --password-stdin
```

### `cardinal logout REGISTRY [REGISTRY ...]`

Remove saved credentials.

```bash
cardinal logout registry.example.com
```

## 3. Container lifecycle

### `cardinal run [OPTIONS] IMAGE [COMMAND ...]`

Create and start a container. The image may instead be supplied with `--image`, and the command may instead be supplied with `--cmd` or `--command`. `--cmd` is parsed as shell-like words; positional command arguments are kept as arguments.

| Option | Description |
|---|---|
| `-d` | Run detached in the background |
| `-n NAME` / `--name NAME` | Container name |
| `-i` | Keep stdin interactive |
| `-t` | Allocate a TTY |
| `--rm` | Remove the container when its process exits |
| `-h HOSTNAME` | Container hostname |
| `--restart POLICY` | `no`, `always`, `on-failure`, or `unless-stopped` |
| `--restart-delay DURATION` | Crash-restart delay, e.g. `10s` or `1m` |
| `--restart-max-attempts N` | Crash-loop budget: automatic restart is blocked after N failures within the window (default 5) |
| `--restart-window DURATION` | Window for the crash-loop budget, e.g. `10m`, `1h` |
| `-e KEY=VALUE` | Repeatable environment variable |
| `--env-file FILE` | Load `KEY=VALUE` or `export KEY=VALUE` lines |
| `-p HOST:CONTAINER[/PROTO]` | Port mapping; comma-separated mappings are accepted |
| `--ports` | Alias for `-p` |
| `-v SRC:DST[:MODE]` | Volume/bind mapping; modes `:ro`/`:rw`, propagation `:shared`/`:rslave`, `nocopy`, plus `tmpfs:` and `nfs://` specs |
| `--volume`, `--vol` | Aliases for `-v` |
| `--memory`, `--ram LIMIT` | Memory limit, e.g. `512m`, `1g` |
| `--cpus`, `--cpu N` | CPU limit, e.g. `0.5`, `2` |
| `--disk LIMIT` | Disk limit, e.g. `1G`, `2T` |
| `--workdir DIR` | Working directory in the container |
| `--image IMAGE` | Image instead of the positional image |
| `--cmd`, `--command COMMAND` | Command instead of positional command arguments |
| `--entrypoint COMMAND` | Override image entrypoint |
| `--network MODE` | `bridge` (default), `none`, `host`, or a user-defined network name |
| `-l`, `--label KEY=VALUE` | Repeatable container label |
| `--cap-add CAP` | Repeatable Linux capability to add |
| `--cap-drop CAP` | Repeatable Linux capability to drop |
| `--user USER` | Username or `UID:GID` |
| `--readonly` | Read-only container root filesystem |
| `--no-new-privs` | Disable acquiring new privileges |
| `--sysctl KEY=VALUE` | Repeatable sysctl setting |
| `--ulimit NAME=SOFT:HARD` | Repeatable ulimit |
| `--dns IP` | Repeatable DNS server |
| `--startup SCRIPT` | Inline startup script or `@FILE`; overrides normal command/entrypoint |
| `--healthcheck-cmd COMMAND` | Healthcheck command |
| `--healthcheck-interval SECONDS` | Healthcheck interval |
| `--healthcheck-retries N` | Consecutive failures before restart |
| `--healthcheck-timeout SECONDS` | Healthcheck timeout |
| `--seccomp-profile FILE` | Path to custom seccomp profile JSON (default: built-in profile) |
| `--apparmor-profile NAME` | AppArmor profile name |
| `--isolated` | Isolate container from other containers (network segmentation) |
| `--encrypted-backup` | Encrypt backup archives with AES-256-GCM |
| `--audit-log` | Enable audit logging for container events |

Examples:

```bash
cardinal run --rm alpine echo hello
cardinal run -d -n web -p 8080:80 nginx:alpine
cardinal run -d --name app --image python:3.12 --cmd 'python /app/main.py'
cardinal run -i -t --rm alpine sh
cardinal run -d --restart unless-stopped --restart-delay 1m --memory 4g --cpus 2 myapp:latest
```

Automatic restart policies are supervised by `cardinal-bootstrap.service`. A manually stopped `unless-stopped` container is not started by boot recovery; `always` is started after boot. The supervisor is installed automatically when possible for root, or explicitly with `cardinal bootstrap --install`.

Quick repeated crashes are protected: once the crash-loop budget (default 5 restarts, `--restart-max-attempts` and `--restart-window`) is exhausted, automatic restart is blocked and the container stays stopped until an explicit `cardinal start`.

Host bind sources must exist and must not use protected system paths such as `/root`, `/etc`, `/var`, `/usr`, `/opt`, or `/run`. Use a data directory such as `/data/myapp` or a named volume:

```bash
mkdir -p /data/myapp
cardinal run -d -v /data/myapp:/app myapp:latest
```

### `cardinal ps [-a]`

List running containers. `-a` includes stopped and created containers.

```bash
cardinal ps
cardinal ps -a
```

### `cardinal inspect [--sensitive] CONTAINER [CONTAINER ...]`

Print container state as JSON. Sensitive fields are hidden unless `--sensitive` is supplied.

```bash
cardinal inspect web
cardinal inspect --sensitive web
```

### `cardinal start CONTAINER`

Start stopped containers while preserving overlay and volumes.

### `cardinal stop [--all] CONTAINER`

Stop containers. `--all` stops every running container.

### `cardinal restart CONTAINER`

Stop and start a container.

### `cardinal rm [-f|-r] CONTAINER`

Remove a container. `-f` force-removes a running container (`-r` is an alias for `-f`). Removing a container removes its writable overlay; named volumes are kept.

### `cardinal rename CONTAINER NEW_NAME`

Rename a container.

### `cardinal set CONTAINER [OPTIONS]`

Change configuration without removing the container. If it was running, cardinal stops and starts it again.

| Option | Description |
|---|---|
| `--memory`, `--ram LIMIT` | Memory limit |
| `--cpus`, `--cpu N` | CPU limit |
| `--disk LIMIT` | Disk limit |
| `--restart POLICY` | Restart policy |
| `--restart-delay DURATION` | Recovery delay |
| `--restart-max-attempts N` | Crash-loop restart budget |
| `--restart-window DURATION` | Crash-loop budget window |
| `--workdir DIR` | Working directory |
| `-e KEY=VALUE` | Add environment variable |
| `--entrypoint COMMAND` | Entrypoint override |
| `--user USER` | User or UID:GID |
| `--readonly` | Read-only rootfs |
| `--no-new-privs` | Disable privilege escalation |
| `-h HOSTNAME` | Hostname |
| `--network MODE` | Network mode |
| `--startup SCRIPT` | Inline startup script or `@FILE`; runs before the container command |

```bash
cardinal set minecraft --restart unless-stopped --restart-delay 1m
cardinal set web --memory 2g --cpus 2
```

## 4. Network commands

### `cardinal network create [--subnet CIDR] NAME`

Create a user-defined Linux bridge network. If `--subnet` is omitted, cardinal selects a free private `/24`.

```bash
cardinal network create --subnet 10.20.0.0/24 appnet
cardinal network ls
cardinal network inspect appnet
cardinal run -d --network appnet alpine sleep infinity
cardinal network rm appnet
```

`network rm` refuses while IP addresses are allocated. Remove containers using the network first. Custom bridges require root or `CAP_NET_ADMIN` and `ip`/`iptables`.

### `cardinal network ls|list`

List user-defined bridge networks. The built-in `cardinal0` bridge is not listed.

### `cardinal network inspect NAME`

Show network ID, driver, subnet, gateway, bridge interface, and allocated IP count.

### `cardinal network rm|remove NAME`

Remove an unused user-defined network and its firewall rules.

## 5. Logs, monitoring, execution, and files

### `cardinal logs [-f] [--tail N] [--previous] [--all] CONTAINER`

Show container stdout/stderr. `-f` follows current output, `--tail` limits current output, `--previous` shows the rotated previous run, and `--all` shows current plus rotated logs. cardinal starts each new run with a fresh log file.

```bash
cardinal logs web
cardinal logs --tail 100 web
cardinal logs -f web
cardinal logs --previous web
cardinal logs --all web
```

### `cardinal attach CONTAINER`

Attach to the main process through the Unix console socket. It requires a running container and is Ctrl+C-safe: disconnecting does not stop the container.

### `cardinal exec [-i] [-t] CONTAINER COMMAND [ARGS ...]`

Run a new process inside a running container.

```bash
cardinal exec web nginx -s reload
cardinal exec -i -t web /bin/sh
```

### `cardinal console CONTAINER`

Open an interactive shell, preferring Bash and falling back to `/bin/sh`.

### `cardinal console-serve ...`

Internal console server used by cardinal; normally invoked by the runtime, not directly by users.

### `cardinal cp SRC DST`

Copy between host and container. Use `CONTAINER:/path` for a container endpoint; container-to-container copying is unsupported.

```bash
cardinal cp ./config.yml web:/etc/app/config.yml
cardinal cp web:/etc/app/config.yml ./backup/
```

### `cardinal fs ls|cat|tree|find ...`

Browse a running or stopped container's merged filesystem.

```text
cardinal fs ls CONTAINER [PATH]
cardinal fs cat CONTAINER PATH
cardinal fs tree CONTAINER [PATH]
cardinal fs find CONTAINER [PATH] [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
cardinal fs find [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
```

The last form searches every container. `--name` is a substring filter, `--grep` searches file contents, `--type` selects files or directories, and `--max-depth` limits recursion.

### `cardinal stats [CONTAINER] [--no-stream]`

Show cgroups v2 CPU, memory, I/O, and process statistics. `--no-stream` prints one sample and exits.

### `cardinal top CONTAINER`

Show processes running in a container.

### `cardinal info`

Show host, runtime, storage, CPU, memory, disk, and container summary information.

### `cardinal events [--since TIME]`

Stream container events as JSON. `--since` accepts RFC3339 or `YYYY-MM-DD HH:MM:SS`.

## 6. Ports, volumes, and backups

### `cardinal port CONTAINER`

Show configured port mappings.

### `cardinal port add CONTAINER HOST:CONTAINER[/PROTO]`

Add a TCP (default) or UDP mapping without recreating the container.

### `cardinal port remove|rm CONTAINER HOST[/PROTO]`

Remove a dynamic mapping. `rm` is an alias for `remove`.

### `cardinal volume create [OPTIONS] [NAME]`

Create a named local volume. If no name is supplied, cardinal generates one.

| Option | Description |
|---|---|
| `-d DRIVER` | Driver; default `local` |
| `-l`, `--label KEY=VALUE` | Repeatable label |

```bash
cardinal volume create app-data
cardinal volume create -l env=prod app-data
```

### `cardinal volume ls|list`

List named volumes.

### `cardinal volume inspect NAME [NAME ...]`

Show driver, mountpoint, creation time, and labels.

### `cardinal volume rm|remove NAME [NAME ...]`

Remove named volumes. Removal is destructive.

### `cardinal volume prune`

Remove local volumes not referenced by any container.

### `cardinal backup COMMAND`

| Command | Syntax | Description |
|---|---|---|
| `create` | `cardinal backup create CONTAINER [-o FILE.tar.gz]` | One-shot backup; container must be stopped |
| `list` / `ls` | `cardinal backup list` | List archives under the cardinal backup directory |
| `restore` | `cardinal backup restore CONTAINER FILE.tar.gz` | Restore into a stopped matching container |
| `enable` | `cardinal backup enable CONTAINER [OPTIONS]` | Enable scheduled backups |
| `disable` | `cardinal backup disable CONTAINER` | Disable scheduled backups |
| `status` | `cardinal backup status CONTAINER` | Show schedule and last result |
| `verify` | `cardinal backup verify FILE.tar.gz` | Verify a backup archive against its checksum sidecar |

`backup enable` options:

- `--interval DURATION` — default `24h`.
- `--retention N` — default `7`, allowed range `1..1000`.
- `--dir PATH` — custom destination; protected host paths and symlink components are rejected.

Backups contain writable overlay data and named volumes, not host bind mounts. Scheduled backups briefly stop a running container for consistency, archive it, then start it again. Enabling a schedule does not create an immediate archive; the first archive is created after the interval. Until that first archive exists, `backup status` may show the schedule's initialization time rather than a completed archive. Install the persistent supervisor with `cardinal bootstrap --install`.

For a complete guide to automatic backups, manual backups, restoration, downloading to your local machine, and edge cases, see the [Backups Guide](backups.md).

## 7. Compose-style configuration

### `cardinal up [-f FILE] [SERVICE]`

Load `cardinal.toml` (or a supplied file), pull images, resolve `depends_on`, and create/start configured containers. `--generate` writes a config from existing named containers.

| Option | Description |
|---|---|
| `-f FILE` | Config path |
| `--generate` | Generate config; defaults to `cardinal.toml` |

```bash
cardinal up
cardinal up -f production.toml api
cardinal up --generate -f generated.toml
```

### `cardinal down [-f FILE] [-a] [SERVICE]`

Remove configured containers. `-f` selects config, a positional service filters the operation, and `-a` removes all containers while ignoring config.

```bash
cardinal down
cardinal down -f production.toml api
cardinal down -a
```

## 8. API, cluster, services, and functions

### `cardinal serve [-p PORT] [-H HOST] [-d] [--token TOKEN] [--tls-cert FILE --tls-key FILE]`

Start the REST API. Defaults are `127.0.0.1:2375`; `CARDINAL_HOST` can override host/port and `CARDINAL_TOKEN` can provide the token. External binds require a token. Supply both TLS files to serve HTTPS; the API still requires a Bearer token for external binds.

```bash
cardinal serve
cardinal serve -H 0.0.0.0 -p 2375 --token "$CARDINAL_TOKEN" --tls-cert /etc/cardinal/server.crt --tls-key /etc/cardinal/server.key -d
```

### `cardinal doctor [--strict]` and `cardinal security check [--strict]`

Run read-only host/runtime checks. The commands inspect permissions, required Linux helpers, namespaces, cgroups, OverlayFS, rootless prerequisites, and API exposure. They never install packages or start/stop containers. Exit status is non-zero for failures; `--strict` also treats warnings as failures.

```bash
cardinal doctor
cardinal doctor --strict
cardinal security check
```

```bash
cardinal serve
cardinal serve -H 0.0.0.0 -p 2375 --token "$CARDINAL_TOKEN" -d
```

### `cardinal cluster COMMAND`

| Command | Syntax / options |
|---|---|
| `init` | `cardinal cluster init [--name NAME] [--bind ADDR] [--port PORT] [--api-port PORT] [--serve] [--token TOKEN]` |
| `join` | `cardinal cluster join PEER [--bind ADDR] [--port PORT] [--serve] [--token TOKEN]` |
| `join-token` | `cardinal cluster join-token` |
| `leave` | `cardinal cluster leave` |
| `info` | `cardinal cluster info` |
| `ls` / `list` | `cardinal cluster ls` |
| `node ls` / `node list` | `cardinal cluster node ls` |
| `node inspect` | `cardinal cluster node inspect ID` |
| `serve` | `cardinal cluster serve [-p PORT] [-H HOST] [--token TOKEN]` |

`CARDINAL_TOKEN` is used when `--token` is not supplied. Do not expose cluster/API ports without authentication and firewall rules.

### `cardinal service COMMAND`

| Command | Syntax |
|---|---|
| `create` | `cardinal service create --name NAME [--replicas N] [-p PORT[:TARGET]] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `cardinal service ls` |
| `rm` / `remove` | `cardinal service rm NAME [NAME ...]` |
| `scale` | `cardinal service scale NAME REPLICAS` |
| `update` | `cardinal service update NAME --image NEW_IMAGE` |

### `cardinal fn COMMAND`

| Command | Syntax |
|---|---|
| `deploy` | `cardinal fn deploy --name NAME [--port N] [--handler PATH] [--timeout SEC] [--idle SEC] [--memory LIMIT] [--cpus N] [--warm N] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `cardinal fn ls` |
| `rm` / `remove` | `cardinal fn rm NAME [NAME ...]` |
| `call` | `cardinal fn call NAME [--data PAYLOAD]` |

`fn deploy` defaults are port `8080`, handler `/handler`, timeout `30` seconds, idle timeout `300` seconds, and zero warm replicas. `-d` is an alias for `--data` in `fn call`.

### `cardinal blueprint COMMAND`

| Command | Syntax |
|---|---|
| `list` / `ls` | `cardinal blueprint list` |
| `info` / `show` | `cardinal blueprint info NAME` |
| `install` / `i` | `cardinal blueprint install NAME [-n NAME] [-d] [--memory LIMIT] [--cpus N] [-e KEY=VALUE] [-y]` |
| `repo list` / `repo ls` | `cardinal blueprint repo list` |
| `repo add` | `cardinal blueprint repo add URL [--name NAME] [--branch BRANCH]` |
| `repo remove` / `repo rm` | `cardinal blueprint repo remove NAME\|URL\|INDEX` |

Blueprint `info` accepts a full name or a matching prefix. `-y` skips installation prompts.

## 9. System and maintenance commands

### `cardinal system prune`

Remove unused containers and images according to the runtime's cleanup rules.

### `cardinal update [--check]`

Check for a newer release. Without `--check` (or `-c`), cardinal prompts before downloading and replacing the current binary. The update verifies a checksum when one is available. The download allows up to five minutes; on failure, each download method reports its own error. If the automatic download fails, install the release manually (see [Running cardinal](running.md), Section 2).

### `cardinal bootstrap [--install] [--remove]`

Install/start or remove the systemd unit `cardinal-bootstrap.service`. `-i` aliases `--install`; `-r` aliases `--remove`. With no action flag, it runs a one-time boot bootstrap pass.

```bash
cardinal bootstrap --install
systemctl status cardinal-bootstrap
cardinal bootstrap --remove
```

### `cardinal supervisor`

Run the persistent restart and scheduled-backup supervisor. It is normally started by systemd and should not be launched manually in a second instance.

### `cardinal version`, `cardinal --version`, `cardinal -v`

Print the installed version.

### `cardinal help`, `cardinal --help`, `cardinal -h`

Print the built-in command overview. Individual commands also print usage when required arguments are missing.

### Internal commands

`cardinal init <container-id> <merged-path>` initializes a container namespace and is invoked by the runtime. `cardinal console-serve` serves attach connections. These are implementation commands and are not intended as normal application entry points.

## 10. Data and environment

- `CARDINAL_DATA_DIR` changes the runtime state directory; the default for root is `/root/.cardinal`.
- `CARDINAL_TOKEN` supplies API/cluster authentication when a token flag is absent.
- `CARDINAL_HOST` can override the REST API host and port.
- Container state, overlays, logs, images, named volumes, sockets, and backups live below the cardinal data directory.
- A bind mount is not copied into a container backup. Archive the host source separately.

For task-oriented recipes, see [Command Examples](examples.md). For installation and troubleshooting, see [Running cardinal](running.md).

## Shell completion

cardinal now uses [spf13/cobra](https://github.com/spf13/cobra) under the hood, which
auto-generates a `completion` sub-command for bash, zsh, fish and PowerShell.

```bash
# bash
cardinal completion bash | sudo tee /etc/bash_completion.d/cardinal > /dev/null
# or for the current user only
cardinal completion bash > ~/.local/share/bash-completion/completions/cardinal

# zsh (place _cardinal anywhere in $fpath)
cardinal completion zsh > "${fpath[1]}/_cardinal"

# fish
cardinal completion fish > ~/.config/fish/completions/cardinal.fish

# PowerShell
cardinal completion powershell | Out-String | Invoke-Expression
```

After sourcing the result you should get:

- Tab completion for every top-level command (`cardinal <TAB>` → `pull  push  run  ...`).
- Per-command flag completion (`cardinal run --<TAB>` → `--restart-max-attempts --restart-delay ...`).
- File-name completion for mount sources.

## Global flags

`--json`, `--quiet`, and `--log-level` are accepted on every sub-command.

| Flag | Default | Meaning |
|---|---|---|
| `--log-level debug\|info\|warn\|error` | `info` | Minimum log level emitted to stderr. |
| `--json` | off | Format logs as JSON-lines (useful for log aggregators). |
| `--quiet` | off | Equivalent to `--log-level error`; suppresses informational output. |
