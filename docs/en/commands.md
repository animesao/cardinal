<!-- dck-version:start -->
**Documentation version:** `1.23.26`
**Project release:** `v1.23.26`
<!-- dck-version:end -->

# dck CLI Command Reference

This is the complete command reference for the Linux `dck` binary. It describes the command tree, positional arguments, short and long flags, aliases, defaults, and safety rules.

> **Platform:** dck runs on Linux and requires namespaces, OverlayFS, cgroups v2, `unshare`, `nsenter`, `mount`, `ip`, `iptables`, and `pgrep`.

## 1. Syntax and prefixes

```text
dck COMMAND [SUBCOMMAND] [OPTIONS] [POSITIONAL-ARGUMENTS]
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
| `--` | End options; following values are command arguments | `dck run alpine sh -c -- 'echo -n hi'` |

Short boolean options are separate flags: use `-i -t`, not the combined `-it` form. Long options are generally not interchangeable unless this reference lists an alias. Quote values containing spaces, `$`, `*`, `:`, or shell syntax.

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

### `dck pull [--platform os/arch] IMAGE[:TAG]`

Pull an image from Docker Hub or a configured registry. If no tag is given, the registry's `latest` tag is used. `--platform` selects a manifest platform such as `linux/amd64` or `linux/arm64`.

```bash
dck pull alpine
dck pull alpine:3.20
dck pull --platform linux/arm64 eclipse-temurin:21
```

### `dck push IMAGE[:TAG] [-u USER] [-p PASSWORD]`

Push a local image. The short credentials flags are `-u` and `-p`; prefer `dck login` or environment variables for automation.

```bash
dck push myapp:v1
dck push -u registry-user -p 'secret' registry.example.com/team/myapp:v1
```

### `dck images`

List locally stored images.

```bash
dck images
```

### `dck search TERM`

Search Docker Hub. A tag filter can be supplied after a colon.

```bash
dck search nginx
dck search python:3.12
```

### `dck rmi IMAGE[:TAG]`

Remove a local image. The default tag is `latest`.

```bash
dck rmi alpine:3.20
```

### `dck verify IMAGE[:TAG]`

Verify a local image's config and layer digests against its stored manifests.

```bash
dck verify alpine:3.20
```

### `dck commit CONTAINER IMAGE[:TAG]`

Create an image from a container's current writable state.

```bash
dck commit web myregistry/web:snapshot
```

### `dck build -t NAME[:TAG] [OPTIONS] [CONTEXT]`

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
dck build -t myapp:dev .
dck build -t myapp:prod -f Dockerfile.prod --build-arg VERSION=1.0 ./src
```

### `dck export IMAGE[:TAG] [-o FILE.tar.gz]`

Export an image to an archive. `-o` selects the output path.

```bash
dck export myapp:v1 -o /data/images/myapp-v1.tar.gz
```

### `dck import FILE.tar.gz [FILE.tar.gz ...]`

Import one or more image archives.

```bash
dck import /data/images/myapp-v1.tar.gz
```

### `dck login REGISTRY [-u USER] [-p PASSWORD] [--password-stdin]`

Save registry credentials. Without credentials, dck prompts interactively. `--password-stdin` reads the password from standard input.

```bash
dck login registry.example.com
echo "$REGISTRY_PASSWORD" | dck login registry.example.com -u "$REGISTRY_USER" --password-stdin
```

### `dck logout REGISTRY [REGISTRY ...]`

Remove saved credentials.

```bash
dck logout registry.example.com
```

## 3. Container lifecycle

### `dck run [OPTIONS] IMAGE [COMMAND ...]`

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

Examples:

```bash
dck run --rm alpine echo hello
dck run -d -n web -p 8080:80 nginx:alpine
dck run -d --name app --image python:3.12 --cmd 'python /app/main.py'
dck run -i -t --rm alpine sh
dck run -d --restart unless-stopped --restart-delay 1m --memory 4g --cpus 2 myapp:latest
```

Automatic restart policies are supervised by `dck-bootstrap.service`. A manually stopped `unless-stopped` container is not started by boot recovery; `always` is started after boot. The supervisor is installed automatically when possible for root, or explicitly with `dck bootstrap --install`.

Quick repeated crashes are protected: once the crash-loop budget (default 5 restarts, `--restart-max-attempts` and `--restart-window`) is exhausted, automatic restart is blocked and the container stays stopped until an explicit `dck start`.

Host bind sources must exist and must not use protected system paths such as `/root`, `/etc`, `/var`, `/usr`, `/opt`, or `/run`. Use a data directory such as `/data/myapp` or a named volume:

```bash
mkdir -p /data/myapp
dck run -d -v /data/myapp:/app myapp:latest
```

### `dck ps [-a]`

List running containers. `-a` includes stopped and created containers.

```bash
dck ps
dck ps -a
```

### `dck inspect [--sensitive] CONTAINER [CONTAINER ...]`

Print container state as JSON. Sensitive fields are hidden unless `--sensitive` is supplied.

```bash
dck inspect web
dck inspect --sensitive web
```

### `dck start CONTAINER`

Start stopped containers while preserving overlay and volumes.

### `dck stop [--all] CONTAINER`

Stop containers. `--all` stops every running container.

### `dck restart CONTAINER`

Stop and start a container.

### `dck rm [-f] CONTAINER`

Remove a container. `-f` force-removes a running container. Removing a container removes its writable overlay; named volumes are kept.

### `dck rename CONTAINER NEW_NAME`

Rename a container.

### `dck set CONTAINER [OPTIONS]`

Change configuration without removing the container. If it was running, dck stops and starts it again.

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

```bash
dck set minecraft --restart unless-stopped --restart-delay 1m
dck set web --memory 2g --cpus 2
```

## 4. Network commands

### `dck network create [--subnet CIDR] NAME`

Create a user-defined Linux bridge network. If `--subnet` is omitted, dck selects a free private `/24`.

```bash
dck network create --subnet 10.20.0.0/24 appnet
dck network ls
dck network inspect appnet
dck run -d --network appnet alpine sleep infinity
dck network rm appnet
```

`network rm` refuses while IP addresses are allocated. Remove containers using the network first. Custom bridges require root or `CAP_NET_ADMIN` and `ip`/`iptables`.

### `dck network ls|list`

List user-defined bridge networks. The built-in `dck0` bridge is not listed.

### `dck network inspect NAME`

Show network ID, driver, subnet, gateway, bridge interface, and allocated IP count.

### `dck network rm|remove NAME`

Remove an unused user-defined network and its firewall rules.

## 5. Logs, monitoring, execution, and files

### `dck logs [-f] [--tail N] [--previous] [--all] CONTAINER`

Show container stdout/stderr. `-f` follows current output, `--tail` limits current output, `--previous` shows the rotated previous run, and `--all` shows current plus rotated logs. dck starts each new run with a fresh log file.

```bash
dck logs web
dck logs --tail 100 web
dck logs -f web
dck logs --previous web
dck logs --all web
```

### `dck attach CONTAINER`

Attach to the main process through the Unix console socket. It requires a running container and is Ctrl+C-safe: disconnecting does not stop the container.

### `dck exec [-i] [-t] CONTAINER COMMAND [ARGS ...]`

Run a new process inside a running container.

```bash
dck exec web nginx -s reload
dck exec -i -t web /bin/sh
```

### `dck console CONTAINER`

Open an interactive shell, preferring Bash and falling back to `/bin/sh`.

### `dck console-serve ...`

Internal console server used by dck; normally invoked by the runtime, not directly by users.

### `dck cp SRC DST`

Copy between host and container. Use `CONTAINER:/path` for a container endpoint; container-to-container copying is unsupported.

```bash
dck cp ./config.yml web:/etc/app/config.yml
dck cp web:/etc/app/config.yml ./backup/
```

### `dck fs ls|cat|tree|find ...`

Browse a running or stopped container's merged filesystem.

```text
dck fs ls CONTAINER [PATH]
dck fs cat CONTAINER PATH
dck fs tree CONTAINER [PATH]
dck fs find CONTAINER [PATH] [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
dck fs find [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
```

The last form searches every container. `--name` is a substring filter, `--grep` searches file contents, `--type` selects files or directories, and `--max-depth` limits recursion.

### `dck stats [CONTAINER] [--no-stream]`

Show cgroups v2 CPU, memory, I/O, and process statistics. `--no-stream` prints one sample and exits.

### `dck top CONTAINER`

Show processes running in a container.

### `dck info`

Show host, runtime, storage, CPU, memory, disk, and container summary information.

### `dck events [--since TIME]`

Stream container events as JSON. `--since` accepts RFC3339 or `YYYY-MM-DD HH:MM:SS`.

## 6. Ports, volumes, and backups

### `dck port CONTAINER`

Show configured port mappings.

### `dck port add CONTAINER HOST:CONTAINER[/PROTO]`

Add a TCP (default) or UDP mapping without recreating the container.

### `dck port remove|rm CONTAINER HOST[/PROTO]`

Remove a dynamic mapping. `rm` is an alias for `remove`.

### `dck volume create [OPTIONS] [NAME]`

Create a named local volume. If no name is supplied, dck generates one.

| Option | Description |
|---|---|
| `-d DRIVER` | Driver; default `local` |
| `-l`, `--label KEY=VALUE` | Repeatable label |

```bash
dck volume create app-data
dck volume create -l env=prod app-data
```

### `dck volume ls|list`

List named volumes.

### `dck volume inspect NAME [NAME ...]`

Show driver, mountpoint, creation time, and labels.

### `dck volume rm|remove NAME [NAME ...]`

Remove named volumes. Removal is destructive.

### `dck volume prune`

Remove local volumes not referenced by any container.

### `dck backup COMMAND`

| Command | Syntax | Description |
|---|---|---|
| `create` | `dck backup create CONTAINER [-o FILE.tar.gz]` | One-shot backup; container must be stopped |
| `list` / `ls` | `dck backup list` | List archives under the dck backup directory |
| `restore` | `dck backup restore CONTAINER FILE.tar.gz` | Restore into a stopped matching container |
| `enable` | `dck backup enable CONTAINER [OPTIONS]` | Enable scheduled backups |
| `disable` | `dck backup disable CONTAINER` | Disable scheduled backups |
| `status` | `dck backup status CONTAINER` | Show schedule and last result |
| `verify` | `dck backup verify FILE.tar.gz` | Verify a backup archive against its checksum sidecar |

`backup enable` options:

- `--interval DURATION` — default `24h`.
- `--retention N` — default `7`, allowed range `1..1000`.
- `--dir PATH` — custom destination; protected host paths and symlink components are rejected.

Backups contain writable overlay data and named volumes, not host bind mounts. Scheduled backups briefly stop a running container for consistency, archive it, then start it again. Enabling a schedule does not create an immediate archive; the first archive is created after the interval. Until that first archive exists, `backup status` may show the schedule's initialization time rather than a completed archive. Install the persistent supervisor with `dck bootstrap --install`.

For a complete guide to automatic backups, manual backups, restoration, downloading to your local machine, and edge cases, see the [Backups Guide](backups.md).

## 7. Compose-style configuration

### `dck up [-f FILE] [SERVICE]`

Load `dck.toml` (or a supplied file), pull images, resolve `depends_on`, and create/start configured containers. `--generate` writes a config from existing named containers.

| Option | Description |
|---|---|
| `-f FILE` | Config path |
| `--generate` | Generate config; defaults to `dck.toml` |

```bash
dck up
dck up -f production.toml api
dck up --generate -f generated.toml
```

### `dck down [-f FILE] [-a] [SERVICE]`

Remove configured containers. `-f` selects config, a positional service filters the operation, and `-a` removes all containers while ignoring config.

```bash
dck down
dck down -f production.toml api
dck down -a
```

## 8. API, cluster, services, and functions

### `dck serve [-p PORT] [-H HOST] [-d] [--token TOKEN] [--tls-cert FILE --tls-key FILE]`

Start the REST API. Defaults are `127.0.0.1:2375`; `DCK_HOST` can override host/port and `DCK_TOKEN` can provide the token. External binds require a token. Supply both TLS files to serve HTTPS; the API still requires a Bearer token for external binds.

```bash
dck serve
dck serve -H 0.0.0.0 -p 2375 --token "$DCK_TOKEN" --tls-cert /etc/dck/server.crt --tls-key /etc/dck/server.key -d
```

### `dck doctor [--strict]` and `dck security check [--strict]`

Run read-only host/runtime checks. The commands inspect permissions, required Linux helpers, namespaces, cgroups, OverlayFS, rootless prerequisites, and API exposure. They never install packages or start/stop containers. Exit status is non-zero for failures; `--strict` also treats warnings as failures.

```bash
dck doctor
dck doctor --strict
dck security check
```

```bash
dck serve
dck serve -H 0.0.0.0 -p 2375 --token "$DCK_TOKEN" -d
```

### `dck cluster COMMAND`

| Command | Syntax / options |
|---|---|
| `init` | `dck cluster init [--name NAME] [--bind ADDR] [--port PORT] [--api-port PORT] [--serve] [--token TOKEN]` |
| `join` | `dck cluster join PEER [--bind ADDR] [--port PORT] [--serve] [--token TOKEN]` |
| `join-token` | `dck cluster join-token` |
| `leave` | `dck cluster leave` |
| `info` | `dck cluster info` |
| `ls` / `list` | `dck cluster ls` |
| `node ls` / `node list` | `dck cluster node ls` |
| `node inspect` | `dck cluster node inspect ID` |
| `serve` | `dck cluster serve [-p PORT] [-H HOST] [--token TOKEN]` |

`DCK_TOKEN` is used when `--token` is not supplied. Do not expose cluster/API ports without authentication and firewall rules.

### `dck service COMMAND`

| Command | Syntax |
|---|---|
| `create` | `dck service create --name NAME [--replicas N] [-p PORT[:TARGET]] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `dck service ls` |
| `rm` / `remove` | `dck service rm NAME [NAME ...]` |
| `scale` | `dck service scale NAME REPLICAS` |
| `update` | `dck service update NAME --image NEW_IMAGE` |

### `dck fn COMMAND`

| Command | Syntax |
|---|---|
| `deploy` | `dck fn deploy --name NAME [--port N] [--handler PATH] [--timeout SEC] [--idle SEC] [--memory LIMIT] [--cpus N] [--warm N] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `dck fn ls` |
| `rm` / `remove` | `dck fn rm NAME [NAME ...]` |
| `call` | `dck fn call NAME [--data PAYLOAD]` |

`fn deploy` defaults are port `8080`, handler `/handler`, timeout `30` seconds, idle timeout `300` seconds, and zero warm replicas. `-d` is an alias for `--data` in `fn call`.

### `dck blueprint COMMAND`

| Command | Syntax |
|---|---|
| `list` / `ls` | `dck blueprint list` |
| `info` / `show` | `dck blueprint info NAME` |
| `install` / `i` | `dck blueprint install NAME [-n NAME] [-d] [--memory LIMIT] [--cpus N] [-e KEY=VALUE] [-y]` |
| `repo list` / `repo ls` | `dck blueprint repo list` |
| `repo add` | `dck blueprint repo add URL [--name NAME] [--branch BRANCH]` |
| `repo remove` / `repo rm` | `dck blueprint repo remove NAME\|URL\|INDEX` |

Blueprint `info` accepts a full name or a matching prefix. `-y` skips installation prompts.

## 9. System and maintenance commands

### `dck system prune`

Remove unused containers and images according to the runtime's cleanup rules.

### `dck update [--check]`

Check for a newer release. Without `--check` (or `-c`), dck prompts before downloading and replacing the current binary. The update verifies a checksum when one is available. The download allows up to five minutes; on failure, each download method reports its own error. If the automatic download fails, install the release manually (see [Running dck](running.md), Section 2).

### `dck bootstrap [--install] [--remove]`

Install/start or remove the systemd unit `dck-bootstrap.service`. `-i` aliases `--install`; `-r` aliases `--remove`. With no action flag, it runs a one-time boot bootstrap pass.

```bash
dck bootstrap --install
systemctl status dck-bootstrap
dck bootstrap --remove
```

### `dck supervisor`

Run the persistent restart and scheduled-backup supervisor. It is normally started by systemd and should not be launched manually in a second instance.

### `dck version`, `dck --version`, `dck -v`

Print the installed version.

### `dck help`, `dck --help`, `dck -h`

Print the built-in command overview. Individual commands also print usage when required arguments are missing.

### Internal commands

`dck init <container-id> <merged-path>` initializes a container namespace and is invoked by the runtime. `dck console-serve` serves attach connections. These are implementation commands and are not intended as normal application entry points.

## 10. Data and environment

- `DCK_DATA_DIR` changes the runtime state directory; the default for root is `/root/.dck`.
- `DCK_TOKEN` supplies API/cluster authentication when a token flag is absent.
- `DCK_HOST` can override the REST API host and port.
- Container state, overlays, logs, images, named volumes, sockets, and backups live below the dck data directory.
- A bind mount is not copied into a container backup. Archive the host source separately.

For task-oriented recipes, see [Command Examples](examples.md). For installation and troubleshooting, see [Running dck](running.md).
