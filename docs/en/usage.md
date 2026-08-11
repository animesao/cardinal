# Usage & Commands

dck is a lightweight container runtime — no daemon, no Docker. Just containers.
~5 MB static binary, OCI images, bridge networking, cluster orchestration, FaaS.

> For the exhaustive command tree, every alias/prefix, and all flags, see the [Complete CLI Command Reference](commands.md). For copy-paste deployment recipes, see [Command Examples](examples.md).

---

## Table of Contents

- [Running Guide](running.md)
- [Deploying Websites](websites.md)
- [Image Management](#image-management)
  - [dck pull](#dck-pull---platform-osarch-imagetag)
  - [dck search](#dck-search-term)
  - [dck images](#dck-images)
  - [dck rmi](#dck-rmi-imagetag)
  - [dck export](#dck-export-image--o-filetargz)
  - [dck import](#dck-import-filetargz)
  - [dck build](#dck-build--t-nametag-options-)
  - [dck commit](#dck-commit-container-imagetag)
  - [dck push](#dck-push-imagetag)
  - [dck login / dck logout](#dck-login-registry--dck-logout-registry)
- [Container Lifecycle](#container-lifecycle)
- [Running Containers (`dck run`)](#dck-run)
- [Working with Containers](#working-with-containers)
- [Exec & Attach](#exec--attach)
- [Logs & Monitoring](#logs--monitoring)
- [Networking](#networking)
- [Filesystem Browser](#filesystem-browser--dck-fs)
- [Storage & Volumes](#storage--volumes)
- [Resource Limits](#resource-limits)
- [Security](#security)
- [Environment Variables](#environment-variables)
- [Healthchecks](#healthchecks)
- [Startup Scripts](#startup-scripts)
- [Port Mapping](#port-mapping)
- [dck.toml / Compose](#dcktoml--compose)
- [Multi-Container Config](#dck-up--dck-down)
- [Cluster Orchestration](#cluster-orchestration)
- [Service Management](#service-management)
- [FaaS / Serverless](#faas--serverless)
- [Blueprints](#blueprints)
- [Image Build & Export](#image-build--export)
- [Registry Operations](#registry-operations)
- [System Commands](#system-commands)
- [Events](#events)
- [Architecture](#architecture)
- [Troubleshooting](#troubleshooting)

---

## Image Management

### `dck pull [--platform os/arch] <image>[:tag]`

Pull an image from a registry (Docker Hub by default).

```bash
dck pull nginx
dck pull alpine:3.19
dck pull --platform linux/arm64 eclipse-temurin:21-jre
dck pull registry.example.com/myapp:v1.0
```

Private registries: set `DOCKER_USERNAME` / `DOCKER_PASSWORD` env vars,
or use `-u user -p pass` on push.

### `dck search <term>`

Search for images on Docker Hub.

```bash
dck search nginx
dck search python
dck search alpine
dck search python:3.11          # filter by tag
```

Shows image name, description, stars, pull count, and available tags. Use `image:tag` syntax to filter by specific tag.

### `dck images`

List locally stored images.

```bash
dck images
```

### `dck rmi <image>[:tag]`

Remove an image from local storage.

```bash
dck rmi nginx:alpine
```

### `dck export <image> -o <file.tar.gz>`

Export an image to a tar.gz file (for backup or transfer).

```bash
dck export myapp:v1 -o myapp-v1.tar.gz
```

### `dck import <file.tar.gz>`

Import an image from a tar.gz file.

```bash
dck import myapp-v1.tar.gz
```

### `dck build -t <name>:<tag> [options] .`

Build an image from a Dockerfile.

```bash
dck build -t myapp:v1 .
dck build -t myapp:v1 --build-arg VERSION=1.0 -f Dockerfile.prod .
```

**Supported Dockerfile instructions:**
FROM, RUN, COPY, ADD, WORKDIR, ENV, CMD, ENTRYPOINT, EXPOSE, LABEL, USER,
VOLUME, SHELL, ARG, HEALTHCHECK, STOPSIGNAL, ONBUILD.

**Features:**
- ✅ Multi-stage builds (`FROM ... AS alias`, `COPY --from=`)
- ✅ ARG substitution (`$VAR` / `${VAR}` in all instructions)
- ✅ HEALTHCHECK with `--start-period`
- `--no-cache` is accepted for CLI compatibility; dck currently rebuilds every Dockerfile instruction and does not reuse instruction results

### `dck commit <container> <image>[:tag]`

Create a new image from a container's current state (including all changes in the overlay).

```bash
dck commit myproject myproject-snapshot:v1
```

This saves everything you installed (packages, files, configs) into a reusable image.

### `dck push <image>[:tag]`

Push a local image to a registry.

```bash
dck push myapp:v1
dck push registry.example.com/myapp:v1
```

Auth: `-u user -p pass` or `DOCKER_USERNAME` / `DOCKER_PASSWORD`.

### `dck login <registry>` / `dck logout <registry>`

Log in or out of a registry for authenticated pulls/pushes.

```bash
dck login registry.example.com
dck logout registry.example.com
```

---

## Container Lifecycle

### `dck ps`

List containers.

```bash
dck ps           # running only
dck ps -a        # all containers (including stopped)
```

### `dck run [options] <image> [command]`

Create and start a container. This is the main command.

```bash
# One-shot command
dck run --rm alpine echo "hello"

# Detached web server
dck run -d -n web -p 80:80 nginx:alpine

# Interactive shell
dck run -i -t --rm alpine sh

# With resource limits
dck run -d --memory 512m --cpus 1.5 node:20 node app.js

# With volume and env
dck run -d -v /data:/data -e DB_URL=postgres://... myapp

# With long flags and auto-restart
dck run -d --name myapp --ports 8080:80 --volume /app:/app --restart always --image nginx:alpine
```

**Important:** dck uses Go's `flag` package, so flags must be passed separately:
- ✅ `dck run -i -t alpine sh` (correct)
- ❌ `dck run -it alpine sh` (will error — use `-i -t`)

#### Run options

| Flag | Description | Example |
|---|---|---|
| `-d` | Detach (run in background) | `-d` |
| `-n <name>` | Container name | `-n myapp` |
| `-p H:C[/proto]` | Port mapping `host:container/tcp\|udp` | `-p 8080:80` |
| `--ports H:C` | Port mapping (alias for `-p`) | `--ports 8080:80` |
| `-v S:D` | Volume mount `source:dest` | `-v /data:/data` |
| `--volume S:D` | Volume mount (alias for `-v`) | `--volume /data:/data` |
| `--vol S:D` | Volume mount (alias for `-v`) | `--vol myvol:/data` |
| `-e K=V` | Environment variable | `-e DB_HOST=localhost` |
| `--env-file <f>` | Read env vars from file | `--env-file .env` |
| `-i` | Interactive (keep stdin open) | `-i` |
| `-t` | Allocate TTY (pseudo-terminal) | `-t` |
| `--rm` | Remove container on exit | `--rm` |
| `--restart <policy>` | Restart: `no`, `always`, `on-failure`, `unless-stopped`; detached boot supervision applies to `always`/`unless-stopped` | `--restart always` |
| `--restart-delay <duration>` | Delay crash recovery, e.g. `10s` or `1m`; does not delay initial boot | `--restart-delay 1m` |
| `--memory <lim>` | Memory limit | `--memory 2g` |
| `--ram <lim>` | Memory limit (alias for `--memory`) | `--ram 1g` |
| `--cpus <num>` | CPU limit | `--cpus 1.5` |
| `--cpu <num>` | CPU limit (alias for `--cpus`) | `--cpu 2` |
| `--disk <lim>` | Disk limit (creates ext4 image) | `--disk 10G` |
| `--workdir <dir>` | Working directory inside container | `--workdir /app` |
| `-h <name>` | Container hostname | `-h myserver` |
| `--entrypoint <cmd>` | Override image entrypoint | `--entrypoint /bin/bash` |
| `--image <img>` | Container image (instead of positional arg) | `--image nginx:alpine` |
| `--cmd <cmd>` | Container command (instead of positional args) | `--cmd "python app.py"` |
| `--command <cmd>` | Container command (alias for `--cmd`) | `--command "java -jar server.jar"` |
| `--cap-add <cap>` | Add capability | `--cap-add NET_ADMIN` |
| `--cap-drop <cap>` | Drop capability | `--cap-drop ALL` |
| `--user <uid>` | Run as UID or `UID:GID` | `--user 1000:1000` |
| `--readonly` | Read-only rootfs | `--readonly` |
| `--no-new-privs` | Disable privilege escalation | `--no-new-privs` |
| `--sysctl <k=v>` | Sysctl parameter | `--sysctl net.ipv4.ip_forward=1` |
| `--ulimit <opt>` | Ulimit: `name=soft:hard` | `--ulimit nofile=1024:2048` |
| `-l, --label <k=v>` | Container label | `-l env=prod` |
| `--dns <ip>` | DNS server (can repeat) | `--dns 8.8.8.8` |
| `--network <mode>` | Network: `bridge` (default), `none`, `host`, or a user-defined network name | `--network appnet` |
| `--startup <s>` | Startup script (inline or `@file`) | `--startup @setup.sh` |
| `--healthcheck-cmd <cmd>` | Health check command | `--healthcheck-cmd "curl -f http://localhost"` |
| `--healthcheck-interval <s>` | Health check interval (seconds) | `--healthcheck-interval 30` |
| `--healthcheck-retries <n>` | Health check retries | `--healthcheck-retries 5` |
| `--healthcheck-timeout <s>` | Health check timeout (seconds) | `--healthcheck-timeout 10` |

### `dck stop <container>`

Stop a running container (sends SIGTERM, then SIGKILL after timeout).

```bash
dck stop web
dck stop web db       # stop multiple
```

### `dck start <container>`

Start a stopped container. All data in the overlay is preserved.

```bash
dck start web
```

### `dck restart <container>`

Restart a container (stop + start).

```bash
dck restart web
```

### `dck rm [-f] <container>`

Remove a container. `-f` forces removal of running containers.

```bash
dck rm web
dck rm -f web         # force remove even if running
```

**Warning:** Removing a container deletes its overlay layer — all changes (installed packages, files) are lost.

### `dck set <container> [options]`

Modify container parameters without deleting (overlay data preserved). Stops, updates, and restarts.

```bash
dck set mc --memory 4g --cpus 2
dck set mc --restart always
dck set mc -e DIFFICULTY=hard
dck set mc --workdir /data-mc
```

### `dck rename <container> <new-name>`

Rename a container.

```bash
dck rename web web-new
```

### `dck port <container>`

Show port mappings for a container.

```bash
dck port web
```

### `dck port add <container> <host>:<container>[/proto]`

Add a port mapping to a running container. Applies iptables DNAT instantly — no restart needed.

```bash
dck port add web 8080:80
dck port add web 53:53/udp
```

### `dck port remove <container> <host>[/proto]`

Remove a port mapping from a running container.

```bash
dck port remove web 8080
dck port rm web 8080    # alias
```

### `dck top <container>`

Show running processes inside a container.

```bash
dck top web
```

---

## Exec & Attach

### `dck exec [-i] [-t] <container> <command>`

Execute a command inside a running container.

```bash
# Run a command (non-interactive)
dck exec web nginx -s reload

# Interactive shell with TTY
dck exec -i -t myproject sh

# Interactive Python
dck exec -i -t myproject python3
```

This creates a **new process** inside the container. It enters the container's namespaces (PID, mount, network, IPC) and runs the command directly at the container's root filesystem (no chroot needed — the container root is already set via pivot_root).

### `dck attach <container>`

Attach to the container's **main process** stdin/stdout (only works for containers started with `-d`).

```bash
dck run -d -i -t -n myproject alpine sh
dck attach myproject    # connect to sh
```

> **exec vs attach:** `attach` connects to the main process stdin/stdout. `exec` runs a new command inside the container. `console` is a shortcut for `exec -i -t` with auto-detected shell.

`dck attach` is **Ctrl+C safe** — the container keeps running.

### `dck console <container>`

Auto-detect and start an interactive shell inside the container. Equivalent to `dck exec -i -t <container> sh`.

```bash
dck console myproject
```

---

## Logs & Monitoring

### `dck logs [-f] [--tail <n>] <container>`

Show container stdout/stderr logs.

```bash
dck logs web            # current-run output
dck logs -f web         # follow new output
dck logs --tail 20 web  # last 20 lines
dck logs -f --tail 10 web  # last 10 lines + follow
```

A fresh dck log is created at every new container start, so output from previous `stop`/`start` or `restart` cycles is not appended indefinitely. Logs written by the application itself (for example Minecraft's `/data/logs/latest.log`) remain in the mounted application storage.

For root, dck logs are stored under `/root/.dck/logs/<container-id>.log`; set `DCK_DATA_DIR` to change the dck state location. See the [running guide](running.md) for operational examples.

### `dck backup create|list|restore|enable|disable|status`

Create manual archives or enable a persistent schedule for one container. Scheduled backups include the writable overlay and named volumes, but not host bind mounts; back up bind-mounted application directories separately. dck briefly stops a running container for a consistent archive and starts it again afterward. The first scheduled archive is created after the configured interval, not immediately.

```bash
dck backup enable minecraft --interval 6h --retention 14
dck backup status minecraft
dck backup list
dck backup disable minecraft

# Optional custom destination (must not be a protected host path)
dck backup enable minecraft --interval 24h --retention 7 --dir /data/backups/minecraft
```

The systemd supervisor must be installed for scheduled backups to continue after the CLI exits:

```bash
dck bootstrap --install
```

`dck backup create NAME -o file.tar.gz` remains the manual one-shot operation. Restore only into a stopped container with `dck backup restore NAME file.tar.gz`.

### `dck stats [container]`

Show live resource usage stats: CPU, memory, I/O, and PIDs. Uses cgroups v2.

```bash
dck stats               # all running containers
dck stats web           # specific container
```

### `dck info`

Show system information: kernel version, storage driver, data directory, CPU model, memory, disk usage.

```bash
dck info
```

---

## Networking

### Network modes

| Mode | Description |
|---|---|
| `bridge` (default) | Each container gets IP `10.0.2.X` on bridge `dck0`. Host at `10.0.2.1`. |
| `none` | No network (loopback only) |
| `host` | Shares host network namespace (for VPN containers, packet capture) |
| `<name>` | User-defined Linux bridge network created with `dck network create` | `--network appnet` |

```bash
dck run -d -n web -p 80:80 nginx:alpine       # bridge (default)
dck run -d --network none alpine sleep infinity
dck run -d --network host myvpn-container

dck network create --subnet 10.20.0.0/24 appnet
dck network ls
dck run -d --network appnet -n app alpine sleep infinity
dck network inspect appnet
dck network rm appnet   # only after containers using it are removed
```

### Network layout

```
Host:        dck0  10.0.2.1/24
Container A: eth0  10.0.2.2
Container B: eth0  10.0.2.3

A → host:      ping 10.0.2.1      (host is gateway)
host → A:      ping 10.0.2.2      (host has route)
A → B:         ping 10.0.2.3      (via bridge)
A → B's port:  curl 10.0.2.1:8080 (DNAT: host_port → B:container_port)
```

### Port mapping

```bash
# TCP (default)
-p 8080:80
-p 8080:80/tcp

# UDP
-p 53:53/udp

# Multiple ports
-p 80:80 -p 443:443
```

Port mapping uses iptables DNAT rules and supports UFW auto-configuration.

### Custom DNS

```bash
dck run -d --dns 1.1.1.1 --dns 8.8.8.8 nginx
```

---



## Storage & Volumes

### Volume mount syntax

```bash
# Bind mount (host directory)
-v /host/path:/container/path
-v /host/path:/container/path:ro     # read-only
-v /host/path:/container/path:shared # shared mount

# Named volume (managed by dck)
-v myvolume:/container/path

# tmpfs (in-memory)
-v tmpfs:/container/path:size=1G,mode=0777

# NFS
-v nfs://server:/export:/container/path:nfsopts=hard,intr
```

### Named volumes

dck can manage named volumes stored under `~/.dck/volumes/`.

```bash
# Create a volume
dck volume create mydata

# List volumes
dck volume ls

# Inspect a volume
dck volume inspect mydata

# Remove a volume
dck volume rm mydata

# Remove unused volumes
dck volume prune
```

### How storage works

```
Storage: /root/.dck/

images/        OCI rootfs per tag (read-only)
containers/    State JSON files
overlay/       upper/work/merged per container (writable layer)
volumes/       Named volumes
logs/          Container stdout/stderr (fresh on each new start)
cache/         Cached image layers
consoles/      Unix sockets for attach
backups/       Scheduled container archives
```

**Overlay:** Each container gets a writable overlay layer on top of the read-only image.
Changes (installed packages, modified files, created files) live in the overlay.
They persist across restarts (`dck stop` + `dck start`) but are **deleted** when the container is removed (`dck rm`).

To save changes permanently, use `dck commit` to create an image from the container.

### Filesystem Browser — `dck fs`

Browse container files without starting a shell. Works on both **running** and **stopped** containers — overlay stays mounted after `stop`.

```bash
dck fs ls <container> [path]              # List files
dck fs cat <container> <path>             # Show file content
dck fs tree <container> [path]            # Directory tree
dck fs find [container] [path] [flags]    # Find files
  --name <pattern>    Filter by name (substring, e.g. "index")
  --grep <text>       Search inside files
  --type f|d          Files or directories only
  --max-depth <n>     Max recursion depth
```

Examples:
```bash
dck fs ls web /etc/nginx
dck fs cat web /etc/nginx/conf.d/default.conf
dck fs tree mc-server /data --max-depth 2
dck fs find web --name "*.conf" --grep "server_name"
dck fs find --name "index"                              # search all containers
```

### Copying files

```bash
# From container to host
dck cp web:/etc/nginx/nginx.conf ./nginx.conf

# From host to container
dck cp ./app.py web:/app/
```

---

## Resource Limits

### Memory

```bash
dck run -d --memory 512m nginx    # 512 megabytes
dck run -d --memory 1g nginx      # 1 gigabyte
dck run -d --memory 2g nginx      # 2 gigabytes
```

Uses cgroups v2 memory controller. If the container exceeds the limit, it gets OOM-killed.

### CPU

```bash
dck run -d --cpus 1.5 nginx       # 1.5 CPU cores
dck run -d --cpus 2 nginx         # 2 CPU cores
```

Uses CFS quota via cgroups v2.

### Disk

```bash
dck run -d --disk 1G nginx        # 1 GB disk limit
dck run -d --disk 10G nginx       # 10 GB disk limit
```

Creates a sparse ext4 image mounted as the overlay's writable layer. Requires `mkfs.ext4`.

---

## Security

### User

Run the container as a non-root user:

```bash
dck run -d --user 1000 nginx
dck run -d --user 1000:1000 nginx   # UID:GID
```

### Capabilities

By default, dck drops dangerous Linux capabilities (SYS_ADMIN, SYS_MODULE, etc.).

```bash
# Add capabilities
dck run -d --cap-add NET_ADMIN nginx
dck run -d --cap-add NET_ADMIN --cap-add SYS_PTRACE nginx

# Drop all capabilities
dck run -d --cap-drop ALL nginx

# Add specific capabilities back after --cap-drop ALL
dck run -d --cap-drop ALL --cap-add NET_BIND_SERVICE nginx
```

### Read-only rootfs

```bash
dck run -d --readonly nginx
```

Makes the container's root filesystem read-only. Writes to volumes still work.

### No new privileges

```bash
dck run -d --no-new-privs nginx
```

Disables acquiring new privileges (setuid, setgid, capabilities) for all processes in the container.

### Sysctls

```bash
dck run -d --sysctl net.ipv4.ip_forward=1 nginx
```

---

## Environment Variables

```bash
# Single variable
dck run -e MY_VAR=value nginx

# Multiple variables
dck run -e DB_HOST=localhost -e DB_PORT=5432 nginx

# From file
dck run --env-file .env nginx
```

**.env file format:**
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=admin
```

### Auto-injected DCK_* variables

When a container starts, dck injects the following environment variables:

| Variable | Description |
|---|---|
| `DCK_CONTAINER_ID` | Full container ID |
| `DCK_CONTAINER_NAME` | Container name |
| `DCK_IMAGE_NAME` | Image name (e.g. `library/alpine`) |
| `DCK_IMAGE_TAG` | Image tag (e.g. `latest`) |
| `DCK_HOSTNAME` | Container hostname |
| `DCK_MEMORY` | Memory limit in bytes |
| `DCK_CPU` | CPU limit in cores |
| `DCK_IP` | Container IP address |
| `DCK_RESTART` | Restart policy |
| `DCK_PORT_TCP_80` | Port mappings (one per mapped port) |

Inside the container, utility scripts are available at `/dck/`:
- `/dck/info` — show container info
- `/dck/env` — show DCK_* environment variables
- `/dck/help` — show help

---

## Healthchecks

Healthchecks run a command inside the container at a given interval. After `retries` consecutive failures, the container is killed and restarted.

```bash
dck run -d \
  --healthcheck-cmd "curl -f http://localhost || exit 1" \
  --healthcheck-interval 30 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  nginx
```

Healthchecks can also be defined in compose files and dck.toml.

---

## Startup Scripts

Use `--startup` to run a custom script instead of the image's default command:

```bash
# Inline script
dck run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Load from file
dck run -d --startup @./myscript.sh ubuntu
```

The script is written to `/startup.sh` inside the container and executed via `/bin/sh`.
When a startup script is present, it **overrides** the normal CMD/entrypoint.

---

## dck.toml / Compose

### dck.toml format

Define containers in a TOML file, start everything with one command.

```toml
[container.web]
image = "nginx:alpine"
ports = ["80:80", "443:80"]
volumes = ["./html:/usr/share/nginx/html"]
restart = "always"

[container.db]
image = "postgres:16"
ports = ["5432:5432"]
env = { POSTGRES_PASSWORD = "secret", POSTGRES_DB = "myapp" }
volumes = ["pg_data:/var/lib/postgresql/data"]
restart = "always"
```

### compose.yaml / docker-compose.yaml

dck supports standard Docker Compose YAML format. See [compose.md](compose.md) for full documentation.

---

## dck up / dck down

### `dck up [name] [-f <file>]`

Create and start containers from a compose file.

Auto-detection order:
1. `dck.toml`
2. `compose.yaml`
3. `compose.yml`
4. `docker-compose.yaml`
5. `docker-compose.yml`

`depends_on` is respected — containers start in dependency order. Supports `service_started`
(default), `service_healthy` (waits for healthcheck), and `service_completed_successfully`.

```toml
[container.db]
image = "postgres:16"
healthcheck = { cmd = "pg_isready -U postgres", interval = 10, retries = 5 }

[container.app]
image = "myapp:latest"
depends_on = { db = "service_healthy" }
```

```bash
dck up                    # auto-detect
dck up myapp              # start only the "myapp" service
dck up -f compose.prod.yaml
dck up myapp              # start only the "myapp" service
dck bootstrap --install  # install boot recovery separately
dck up --generate         # generate dck.toml from existing containers
```

### `dck down [name] [-f <file>]`

Stop and remove containers from a compose file.

```bash
dck down                  # stop + remove
dck down myapp            # stop + remove only "myapp"
dck down -f dck.toml
dck down -a               # remove ALL containers (ignore config)
# Remove named volumes separately, only when safe:
dck volume prune
```

---

## dck serve

Start a Docker-compatible REST API server.

```bash
dck serve -p 2375  # localhost only by default; use --token for external bind
```

Compatible with Docker clients, Portainer, VS Code Dev Containers, and CI tools.

---

## Auto-Start on Boot

Detached containers with `--restart always` or `--restart unless-stopped` start automatically after reboot. The persistent supervisor does not adopt `on-failure` containers after the short-lived detached CLI exits.

dck auto-installs and starts a persistent systemd supervisor when you:
- `dck run --restart always <image>`
- `dck set <container> --restart always`
- `dck up` (if any container has restart: "always")

You can also manage it manually:

```bash
dck bootstrap --install      # install systemd service
dck bootstrap --remove       # remove systemd service
dck bootstrap                # start all restart=always containers now
```

Boot flow:
```
System boot → systemd → dck-bootstrap.service → dck supervisor
  └─ For each detached container with restart=always or unless-stopped:
      1. Setup overlayfs
      2. Run unshare with namespaces
      3. Setup veth + iptables
```

---

## Cluster Orchestration

dck supports multi-node clustering with service management, DNS-based service discovery,
and rolling updates.

For full documentation, see [cluster.md](cluster.md).

```bash
# Initialize a cluster
dck cluster init --name prod --bind 0.0.0.0 --port 2375 --token '<strong-random-token>'

# Join a cluster
dck cluster join 10.0.0.1:2375

# Show connection address for other nodes
dck cluster join-token

# Cluster overview (name, nodes, services)
dck cluster info

# List nodes (with CPU, memory, labels)
dck cluster node ls

# Detailed node info
dck cluster node inspect <id>

# List nodes (short view)
dck cluster ls

# Start API server (accepts remote replica requests)
dck cluster serve -p 2375

# Or start API server automatically on init/join
dck cluster init --name prod --serve
dck cluster join 10.0.0.1:7946 --serve

# Leave cluster
dck cluster leave
```

---

## Service Management

Services allow running replicated containers across a cluster.

For full documentation, see [cluster.md](cluster.md).

```bash
dck service create --name web --replicas 3 --port 80:80 nginx:alpine
dck service ls
dck service scale web 5
dck service update web --image nginx:1.25
dck service rm web
```

---

## FaaS / Serverless

dck can deploy container images as serverless functions with auto-scaling and scale-to-zero.

For full documentation, see [faas.md](faas.md).

```bash
# Deploy a function
dck fn deploy --name hello --port 8080 --timeout 30 --idle 300 ghcr.io/myorg/hello-func

# Invoke
dck fn call hello --data '{"name": "dck"}'

# List functions
dck fn ls

# Remove
dck fn rm hello
```

---

## Blueprints

Blueprints are pre-configured container templates that can be installed from repositories.

```bash
# List available blueprints
dck blueprint list

# Show blueprint details with examples
dck blueprint info mysql-8
dck blueprint info minecraft-server

# Install a blueprint
dck blueprint install nginx-proxy

# Add a custom blueprint repository
dck blueprint repo add https://github.com/user/my-blueprints

# List repositories
dck blueprint repo list

# Remove a repository
dck blueprint repo remove my-blueprints
```

---

## Events

Stream container lifecycle events in JSON format.

```bash
dck events                          # live stream
dck events --since "2026-07-07 12:00:00"  # events since timestamp
```

Events: `start`, `stop`, `kill`, `oom`, `healthcheck_failed`, etc.

---

## System Commands

### `dck system prune`

Remove unused containers and images.

```bash
dck system prune
```

### `dck update [--check]`

Check for updates and self-update the dck binary.

```bash
dck update              # update to latest version
dck update --check      # only check version
```

### `dck version`

Show version information.

```bash
dck version
```

---

## Architecture

```
dck run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc dck init <id>
  │   └─ dck init → pivot_root to overlay → setup /proc/lo/eth0 → exec CMD
  └─ dck console-serve <id>
      ├─ reads stdout pipe
      ├─ writes to log file
      ├─ listens on Unix socket
      └─ broadcasts to all attach clients
```

### Key Concepts

| Concept | Description |
|---|---|
| **Image** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Pulled once via `dck pull`. |
| **Container** | Image + writable overlay layer. Changes live in the overlay, not the image. |
| **Overlay** | Diff layer on top of the image. Persists across restarts — packages stay installed. |
| **Volume** | Host bind mount into the container. `-v /data/mybot:/bot` mounts `/data/mybot` as `/bot`. |
| **Network** | Every container gets IP `10.0.2.X` on bridge `dck0`. Host at `10.0.2.1`. |

### Execution Flow

1. `dck run` pulls the image (if not cached)
2. Creates an overlay filesystem (lower=image rootfs, upper=container layer, merged=container root)
3. Runs `unshare` with PID, mount, net, UTS, IPC namespaces
4. Inside the namespace, `dck init` does `pivot_root` to the overlay, mounts /proc, sets up networking
5. Executes the container command (CMD or `--startup` script)
6. If detached, `dck console-serve` captures stdout and serves it via Unix socket for `dck attach`

---

## Troubleshooting

### dck rm -f <container> hangs

If a container won't stop, try:

```bash
# Force kill the process
kill -9 $(cat /root/.dck/containers/*.json | grep -o '"pid":[0-9]*' | grep -o '[0-9]*')

# Then remove
dck rm -f <container>

# Manual cleanup if state files are corrupt
rm -f /root/.dck/containers/<id>.json
```

### Overlay not mounting

Ensure overlayfs is supported:

```bash
lsmod | grep overlay
modprobe overlay   # if not loaded
```

### Network not working

```bash
# Check bridge exists
ip link show dck0

# Ensure IP forwarding is enabled
sysctl net.ipv4.ip_forward

# Reinstall network base
dck system prune && dck pull alpine && dck run --rm alpine ping 8.8.8.8
```

### Port mapping not working

```bash
# Check iptables rules
iptables -t nat -L -n | grep dck

# UFW may block ports — check ufw status
ufw status
```

### Rootless mode

dck supports rootless execution on systems with `newuidmap`/`newgidmap`.
Rootless containers use userspace networking (slirp4netns-style).

### Comparison with Docker

| Feature | dck | Docker |
|---|---|---|
| Daemon | No daemon | dockerd required |
| Binary size | ~5 MB | ~100+ MB |
| Namespaces | PID, Mount, Net, UTS, IPC | All |
| Bridge network | dck0 (10.0.2.0/24) | docker0 |
| Port mapping | iptables DNAT | iptables DNAT |
| Auto-start | persistent systemd supervisor | systemd dockerd |
| Image format | OCI/Docker V2 | OCI/Docker V2 |
