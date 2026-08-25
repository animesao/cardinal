<!-- cardinal-version:start -->
**Documentation version:** `2.0.3`
**Project release:** `v2.0.3`
<!-- cardinal-version:end -->

# Usage & Commands

cardinal is a lightweight container runtime — no daemon, no Docker. Just containers.
~5 MB static binary, OCI images, bridge networking, cluster orchestration, FaaS.

> For the exhaustive command tree, every alias/prefix, and all flags, see the [Complete CLI Command Reference](commands.md). For copy-paste deployment recipes, see [Command Examples](examples.md).

---

## Table of Contents

- [Running Guide](running.md)
- [Deploying Websites](websites.md)
- [Image Management](#image-management)
  - [cardinal pull](#cardinal-pull---platform-osarch-imagetag)
  - [cardinal search](#cardinal-search-term)
  - [cardinal images](#cardinal-images)
  - [cardinal rmi](#cardinal-rmi-imagetag)
  - [cardinal export](#cardinal-export-image--o-filetargz)
  - [cardinal import](#cardinal-import-filetargz)
  - [cardinal build](#cardinal-build--t-nametag-options-)
  - [cardinal commit](#cardinal-commit-container-imagetag)
  - [cardinal push](#cardinal-push-imagetag)
  - [cardinal login / cardinal logout](#cardinal-login-registry--cardinal-logout-registry)
- [Container Lifecycle](#container-lifecycle)
- [Running Containers (`cardinal run`)](#cardinal-run)
- [Working with Containers](#working-with-containers)
- [Exec & Attach](#exec--attach)
- [Logs & Monitoring](#logs--monitoring)
- [Networking](#networking)
- [Filesystem Browser](#filesystem-browser--cardinal-fs)
- [Storage & Volumes](#storage--volumes)
- [Resource Limits](#resource-limits)
- [Security](#security)
- [Environment Variables](#environment-variables)
- [Healthchecks](#healthchecks)
- [Startup Scripts](#startup-scripts)
- [Port Mapping](#port-mapping)
- [cardinal.toml / Compose](#cardinaltoml--compose)
- [Multi-Container Config](#cardinal-up--cardinal-down)
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

### `cardinal pull [--platform os/arch] <image>[:tag]`

Pull an image from a registry (Docker Hub by default).

```bash
cardinal pull nginx
cardinal pull alpine:3.19
cardinal pull --platform linux/arm64 eclipse-temurin:21-jre
cardinal pull registry.example.com/myapp:v1.0
```

Private registries: set `DOCKER_USERNAME` / `DOCKER_PASSWORD` env vars,
or use `-u user -p pass` on push.

### `cardinal search <term>`

Search for images on Docker Hub.

```bash
cardinal search nginx
cardinal search python
cardinal search alpine
cardinal search python:3.11          # filter by tag
```

Shows image name, description, stars, pull count, and available tags. Use `image:tag` syntax to filter by specific tag.

### `cardinal images`

List locally stored images.

```bash
cardinal images
```

### `cardinal rmi <image>[:tag]`

Remove an image from local storage.

```bash
cardinal rmi nginx:alpine
```

### `cardinal verify <image>[:tag]`

Verify a local image's config and layer digests.

```bash
cardinal verify nginx:alpine
```

### `cardinal export <image> -o <file.tar.gz>`

Export an image to a tar.gz file (for backup or transfer).

```bash
cardinal export myapp:v1 -o myapp-v1.tar.gz
```

### `cardinal import <file.tar.gz>`

Import an image from a tar.gz file.

```bash
cardinal import myapp-v1.tar.gz
```

### `cardinal build -t <name>:<tag> [options] .`

Build an image from a Dockerfile.

```bash
cardinal build -t myapp:v1 .
cardinal build -t myapp:v1 --build-arg VERSION=1.0 -f Dockerfile.prod .
```

**Supported Dockerfile instructions:**
FROM, RUN, COPY, ADD, WORKDIR, ENV, CMD, ENTRYPOINT, EXPOSE, LABEL, USER,
VOLUME, SHELL, ARG, HEALTHCHECK, STOPSIGNAL, ONBUILD.

**Features:**
- ✅ Multi-stage builds (`FROM ... AS alias`, `COPY --from=`)
- ✅ ARG substitution (`$VAR` / `${VAR}` in all instructions)
- ✅ HEALTHCHECK with `--start-period`
- `--no-cache` is accepted for CLI compatibility; cardinal currently rebuilds every Dockerfile instruction and does not reuse instruction results

### `cardinal commit <container> <image>[:tag]`

Create a new image from a container's current state (including all changes in the overlay).

```bash
cardinal commit myproject myproject-snapshot:v1
```

This saves everything you installed (packages, files, configs) into a reusable image.

### `cardinal push <image>[:tag]`

Push a local image to a registry.

```bash
cardinal push myapp:v1
cardinal push registry.example.com/myapp:v1
```

Auth: `-u user -p pass` or `DOCKER_USERNAME` / `DOCKER_PASSWORD`.

### `cardinal login <registry>` / `cardinal logout <registry>`

Log in or out of a registry for authenticated pulls/pushes.

```bash
cardinal login registry.example.com
cardinal logout registry.example.com
```

---

## Container Lifecycle

### `cardinal ps`

List containers.

```bash
cardinal ps           # running only
cardinal ps -a        # all containers (including stopped)
```

### `cardinal run [options] <image> [command]`

Create and start a container. This is the main command.

```bash
# One-shot command
cardinal run --rm alpine echo "hello"

# Detached web server
cardinal run -d -n web -p 80:80 nginx:alpine

# Interactive shell
cardinal run -i -t --rm alpine sh

# With resource limits
cardinal run -d --memory 512m --cpus 1.5 node:20 node app.js

# With volume and env
cardinal run -d -v /data:/data -e DB_URL=postgres://... myapp

# With long flags and auto-restart
cardinal run -d --name myapp --ports 8080:80 --volume /app:/app --restart always --image nginx:alpine
```

**Important:** cardinal uses Go's `flag` package, so flags must be passed separately:
- ✅ `cardinal run -i -t alpine sh` (correct)
- ❌ `cardinal run -it alpine sh` (will error — use `-i -t`)

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
| `--restart-max-attempts <n>` | Crash-loop budget: stop auto-restart after N failures within the window (default 5) | `--restart-max-attempts 5` |
| `--restart-window <duration>` | Window for the crash-loop budget | `--restart-window 10m` |
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

### `cardinal stop <container>`

Stop a running container (sends SIGTERM, then SIGKILL after timeout).

```bash
cardinal stop web
cardinal stop web db       # stop multiple
```

### `cardinal start <container>`

Start a stopped container. All data in the overlay is preserved.

```bash
cardinal start web
```

### `cardinal restart <container>`

Restart a container (stop + start).

```bash
cardinal restart web
```

### `cardinal rm [-f] <container>`

Remove a container. `-f` forces removal of running containers.

```bash
cardinal rm web
cardinal rm -f web         # force remove even if running
```

**Warning:** Removing a container deletes its overlay layer — all changes (installed packages, files) are lost.

### `cardinal set <container> [options]`

Modify container parameters without deleting (overlay data preserved). Stops, updates, and restarts.

```bash
cardinal set mc --memory 4g --cpus 2
cardinal set mc --restart always
cardinal set mc -e DIFFICULTY=hard
cardinal set mc --workdir /data-mc
```

### `cardinal rename <container> <new-name>`

Rename a container.

```bash
cardinal rename web web-new
```

### `cardinal port <container>`

Show port mappings for a container.

```bash
cardinal port web
```

### `cardinal port add <container> <host>:<container>[/proto]`

Add a port mapping to a running container. Applies iptables DNAT instantly — no restart needed.

```bash
cardinal port add web 8080:80
cardinal port add web 53:53/udp
```

### `cardinal port remove <container> <host>[/proto]`

Remove a port mapping from a running container.

```bash
cardinal port remove web 8080
cardinal port rm web 8080    # alias
```

### `cardinal top <container>`

Show running processes inside a container.

```bash
cardinal top web
```

---

## Exec & Attach

### `cardinal exec [-i] [-t] <container> <command>`

Execute a command inside a running container.

```bash
# Run a command (non-interactive)
cardinal exec web nginx -s reload

# Interactive shell with TTY
cardinal exec -i -t myproject sh

# Interactive Python
cardinal exec -i -t myproject python3
```

This creates a **new process** inside the container. It enters the container's namespaces (PID, mount, network, IPC) and runs the command directly at the container's root filesystem (no chroot needed — the container root is already set via pivot_root).

### `cardinal attach <container>`

Attach to the container's **main process** stdin/stdout (only works for containers started with `-d`).

```bash
cardinal run -d -i -t -n myproject alpine sh
cardinal attach myproject    # connect to sh
```

> **exec vs attach:** `attach` connects to the main process stdin/stdout. `exec` runs a new command inside the container. `console` is a shortcut for `exec -i -t` with auto-detected shell.

`cardinal attach` is **Ctrl+C safe** — the container keeps running.

### `cardinal console <container>`

Auto-detect and start an interactive shell inside the container. Equivalent to `cardinal exec -i -t <container> sh`.

```bash
cardinal console myproject
```

---

## Logs & Monitoring

### `cardinal logs [-f] [--tail <n>] <container>`

Show container stdout/stderr logs.

```bash
cardinal logs web            # current-run output
cardinal logs -f web         # follow new output
cardinal logs --tail 20 web  # last 20 lines
cardinal logs -f --tail 10 web  # last 10 lines + follow
```

A fresh cardinal log is created at every new container start, so output from previous `stop`/`start` or `restart` cycles is not appended indefinitely. Logs written by the application itself (for example Minecraft's `/data/logs/latest.log`) remain in the mounted application storage.

For root, cardinal logs are stored under `/root/.cardinal/logs/<container-id>.log`; set `CARDINAL_DATA_DIR` to change the cardinal state location. See the [running guide](running.md) for operational examples.

### `cardinal backup create|list|restore|enable|disable|status|verify`

Create manual archives or enable a persistent schedule for one container. Scheduled backups include the writable overlay and named volumes, but not host bind mounts; back up bind-mounted application directories separately. cardinal briefly stops a running container for a consistent archive and starts it again afterward. The first scheduled archive is created after the configured interval, not immediately.

```bash
cardinal backup enable minecraft --interval 6h --retention 14
cardinal backup status minecraft
cardinal backup list
cardinal backup disable minecraft

# Optional custom destination (must not be a protected host path)
cardinal backup enable minecraft --interval 24h --retention 7 --dir /data/backups/minecraft
```

The systemd supervisor must be installed for scheduled backups to continue after the CLI exits:

```bash
cardinal bootstrap --install
```

`cardinal backup create NAME -o file.tar.gz` remains the manual one-shot operation. Restore only into a stopped container with `cardinal backup restore NAME file.tar.gz`. Verify an archive against its checksum with `cardinal backup verify FILE.tar.gz`; when no checksum sidecar exists, cardinal reports the archive as unverified.

### `cardinal stats [container]`

Show live resource usage stats: CPU, memory, I/O, and PIDs. Uses cgroups v2.

```bash
cardinal stats               # all running containers
cardinal stats web           # specific container
```

### `cardinal info`

Show system information: kernel version, storage driver, data directory, CPU model, memory, disk usage.

```bash
cardinal info
```

---

## Networking

### Network modes

| Mode | Description |
|---|---|
| `bridge` (default) | Each container gets IP `10.0.2.X` on bridge `cardinal0`. Host at `10.0.2.1`. |
| `none` | No network (loopback only) |
| `host` | Shares host network namespace (for VPN containers, packet capture) |
| `<name>` | User-defined Linux bridge network created with `cardinal network create` | `--network appnet` |

```bash
cardinal run -d -n web -p 80:80 nginx:alpine       # bridge (default)
cardinal run -d --network none alpine sleep infinity
cardinal run -d --network host myvpn-container

cardinal network create --subnet 10.20.0.0/24 appnet
cardinal network ls
cardinal run -d --network appnet -n app alpine sleep infinity
cardinal network inspect appnet
cardinal network rm appnet   # only after containers using it are removed
```

### Network layout

```
Host:        cardinal0  10.0.2.1/24
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
cardinal run -d --dns 1.1.1.1 --dns 8.8.8.8 nginx
```

---



## Storage & Volumes

### Volume mount syntax

```bash
# Bind mount (host directory)
-v /host/path:/container/path
-v /host/path:/container/path:ro     # read-only
-v /host/path:/container/path:shared # shared mount

# Named volume (managed by cardinal)
-v myvolume:/container/path

# tmpfs (in-memory)
-v tmpfs:/container/path:size=1G,mode=0777

# NFS
-v nfs://server:/export:/container/path:nfsopts=hard,intr
```

### Named volumes

cardinal can manage named volumes stored under `~/.cardinal/volumes/`.

```bash
# Create a volume
cardinal volume create mydata

# List volumes
cardinal volume ls

# Inspect a volume
cardinal volume inspect mydata

# Remove a volume
cardinal volume rm mydata

# Remove unused volumes
cardinal volume prune
```

### How storage works

```
Storage: /root/.cardinal/

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
They persist across restarts (`cardinal stop` + `cardinal start`) but are **deleted** when the container is removed (`cardinal rm`).

To save changes permanently, use `cardinal commit` to create an image from the container.

### Filesystem Browser — `cardinal fs`

Browse container files without starting a shell. Works on both **running** and **stopped** containers — overlay stays mounted after `stop`.

```bash
cardinal fs ls <container> [path]              # List files
cardinal fs cat <container> <path>             # Show file content
cardinal fs tree <container> [path]            # Directory tree
cardinal fs find [container] [path] [flags]    # Find files
  --name <pattern>    Filter by name (substring, e.g. "index")
  --grep <text>       Search inside files
  --type f|d          Files or directories only
  --max-depth <n>     Max recursion depth
```

Examples:
```bash
cardinal fs ls web /etc/nginx
cardinal fs cat web /etc/nginx/conf.d/default.conf
cardinal fs tree mc-server /data --max-depth 2
cardinal fs find web --name "*.conf" --grep "server_name"
cardinal fs find --name "index"                              # search all containers
```

### Copying files

```bash
# From container to host
cardinal cp web:/etc/nginx/nginx.conf ./nginx.conf

# From host to container
cardinal cp ./app.py web:/app/
```

---

## Resource Limits

### Memory

```bash
cardinal run -d --memory 512m nginx    # 512 megabytes
cardinal run -d --memory 1g nginx      # 1 gigabyte
cardinal run -d --memory 2g nginx      # 2 gigabytes
```

Uses cgroups v2 memory controller. If the container exceeds the limit, it gets OOM-killed.

### CPU

```bash
cardinal run -d --cpus 1.5 nginx       # 1.5 CPU cores
cardinal run -d --cpus 2 nginx         # 2 CPU cores
```

Uses CFS quota via cgroups v2.

### Disk

```bash
cardinal run -d --disk 1G nginx        # 1 GB disk limit
cardinal run -d --disk 10G nginx       # 10 GB disk limit
```

Creates a sparse ext4 image mounted as the overlay's writable layer. Requires `mkfs.ext4`.

---

## Security

### User

Run the container as a non-root user:

```bash
cardinal run -d --user 1000 nginx
cardinal run -d --user 1000:1000 nginx   # UID:GID
```

### Capabilities

By default, cardinal keeps the Docker-compatible safe capability set needed by common images (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `FSETID`, `KILL`, `SETGID`, `SETUID`, `SETPCAP`, `NET_BIND_SERVICE`, `NET_RAW`, `SYS_CHROOT`, `MKNOD`, `AUDIT_WRITE`, and `SETFCAP`). Dangerous capabilities such as `SYS_ADMIN` and `SYS_MODULE` remain dropped. This lets standard images such as `nginx:alpine` initialize their filesystems normally.

```bash
# Add capabilities
cardinal run -d --cap-add NET_ADMIN nginx
cardinal run -d --cap-add NET_ADMIN --cap-add SYS_PTRACE nginx

# Drop all capabilities (maximum restriction)
cardinal run -d --cap-drop ALL nginx

# Add specific capabilities back after --cap-drop ALL
cardinal run -d --cap-drop ALL --cap-add NET_BIND_SERVICE nginx
```

### Read-only rootfs

```bash
cardinal run -d --readonly nginx
```

Makes the container's root filesystem read-only. Writes to volumes still work.

### No new privileges

```bash
cardinal run -d --no-new-privs nginx
```

Disables acquiring new privileges (setuid, setgid, capabilities) for all processes in the container.

### Sysctls

```bash
cardinal run -d --sysctl net.ipv4.ip_forward=1 nginx
```

### Seccomp Profile

cardinal applies a default seccomp profile that blocks 30+ dangerous syscalls including `mount`, `ptrace`, `reboot`, `kexec_load`, `bpf`, and `init_module`.

```bash
# Use default seccomp profile (automatic)
cardinal run -d nginx

# Use custom seccomp profile
cardinal run -d --seccomp-profile /path/to/profile.json nginx
```

### AppArmor Profile

cardinal applies a default AppArmor profile (`cardinal-container`) that restricts access to sensitive host paths and limits container capabilities.

```bash
# Use default AppArmor profile (automatic)
cardinal run -d nginx

# Use custom AppArmor profile
cardinal run -d --apparmor-profile my-profile nginx
```

### Network Isolation

Isolate a container from all other containers to prevent lateral movement:

```bash
cardinal run -d --isolated nginx

# Allow specific communication
cardinal run -d --isolated --network appnet nginx
```

### Audit Logging

Enable audit logging to record all container lifecycle events:

```bash
cardinal run -d --audit-log nginx

# Events are logged to ~/.cardinal/audit/audit-YYYY-MM-DD.log
cat ~/.cardinal/audit/audit-$(date +%Y-%m-%d).log
```

### Backup Encryption

Encrypt backup archives with AES-256-GCM:

```bash
# Generate encryption key
cardinal backup generate-key

# Set key via environment variable
export CARDINAL_BACKUP_KEY="your-hex-key"

# Create encrypted backup
cardinal backup create nginx -e

# Create encrypted backup with custom output
cardinal backup create nginx -o /data/backups/nginx.enc -e
```

---

## Environment Variables

```bash
# Single variable
cardinal run -e MY_VAR=value nginx

# Multiple variables
cardinal run -e DB_HOST=localhost -e DB_PORT=5432 nginx

# From file
cardinal run --env-file .env nginx
```

**.env file format:**
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=admin
```

### Auto-injected CARDINAL_* variables

When a container starts, cardinal injects the following environment variables:

| Variable | Description |
|---|---|
| `CARDINAL_CONTAINER_ID` | Full container ID |
| `CARDINAL_CONTAINER_NAME` | Container name |
| `CARDINAL_IMAGE_NAME` | Image name (e.g. `library/alpine`) |
| `CARDINAL_IMAGE_TAG` | Image tag (e.g. `latest`) |
| `CARDINAL_HOSTNAME` | Container hostname |
| `CARDINAL_MEMORY` | Memory limit in bytes |
| `CARDINAL_CPU` | CPU limit in cores |
| `CARDINAL_IP` | Container IP address |
| `CARDINAL_RESTART` | Restart policy |
| `CARDINAL_PORT_TCP_80` | Port mappings (one per mapped port) |

Inside the container, utility scripts are available at `/cardinal/`:
- `/cardinal/info` — show container info
- `/cardinal/env` — show CARDINAL_* environment variables
- `/cardinal/help` — show help

---

## Healthchecks

Healthchecks run a command inside the container at a given interval. After `retries` consecutive failures, the container is killed and restarted.

```bash
cardinal run -d \
  --healthcheck-cmd "curl -f http://localhost || exit 1" \
  --healthcheck-interval 30 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  nginx
```

Healthchecks can also be defined in compose files and cardinal.toml.

---

## Startup Scripts

Use `--startup` to run a custom script instead of the image's default command:

```bash
# Inline script
cardinal run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Load from file
cardinal run -d --startup @./myscript.sh ubuntu
```

The script is written to `/startup.sh` inside the container and executed via `/bin/sh`.
When a startup script is present, it **overrides** the normal CMD/entrypoint.

---

## cardinal.toml / Compose

### cardinal.toml format

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

cardinal supports standard Docker Compose YAML format. See [compose.md](compose.md) for full documentation.

---

## cardinal up / cardinal down

### `cardinal up [name] [-f <file>]`

Create and start containers from a compose file.

Auto-detection order:
1. `cardinal.toml`
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
cardinal up                    # auto-detect
cardinal up myapp              # start only the "myapp" service
cardinal up -f compose.prod.yaml
cardinal up myapp              # start only the "myapp" service
cardinal bootstrap --install  # install boot recovery separately
cardinal up --generate         # generate cardinal.toml from existing containers
```

### `cardinal down [name] [-f <file>]`

Stop and remove containers from a compose file.

```bash
cardinal down                  # stop + remove
cardinal down myapp            # stop + remove only "myapp"
cardinal down -f cardinal.toml
cardinal down -a               # remove ALL containers (ignore config)
# Remove named volumes separately, only when safe:
cardinal volume prune
```

---

## cardinal serve

Start a Docker-compatible REST API server.

```bash
cardinal serve -p 2375  # localhost only by default; use --token for external bind
```

Compatible with Docker clients, Portainer, VS Code Dev Containers, and CI tools.

---

## Auto-Start on Boot

Detached containers with `--restart always` or `--restart unless-stopped` start automatically after reboot. The persistent supervisor does not adopt `on-failure` containers after the short-lived detached CLI exits.

cardinal auto-installs and starts a persistent systemd supervisor when you:
- `cardinal run --restart always <image>`
- `cardinal set <container> --restart always`
- `cardinal up` (if any container has restart: "always")

You can also manage it manually:

```bash
cardinal bootstrap --install      # install systemd service
cardinal bootstrap --remove       # remove systemd service
cardinal bootstrap                # start all restart=always containers now
```

Boot flow:
```
System boot → systemd → cardinal-bootstrap.service → cardinal supervisor
  └─ For each detached container with restart=always or unless-stopped:
      1. Setup overlayfs
      2. Run unshare with namespaces
      3. Setup veth + iptables
```

---

## Cluster Orchestration

cardinal supports multi-node clustering with service management, DNS-based service discovery,
and rolling updates.

For full documentation, see [cluster.md](cluster.md).

```bash
# Initialize a cluster
cardinal cluster init --name prod --bind 0.0.0.0 --port 2375 --token '<strong-random-token>'

# Join a cluster
cardinal cluster join 10.0.0.1:2375

# Show connection address for other nodes
cardinal cluster join-token

# Cluster overview (name, nodes, services)
cardinal cluster info

# List nodes (with CPU, memory, labels)
cardinal cluster node ls

# Detailed node info
cardinal cluster node inspect <id>

# List nodes (short view)
cardinal cluster ls

# Start API server (accepts remote replica requests)
cardinal cluster serve -p 2375

# Or start API server automatically on init/join
cardinal cluster init --name prod --serve
cardinal cluster join 10.0.0.1:7946 --serve

# Leave cluster
cardinal cluster leave
```

---

## Service Management

Services allow running replicated containers across a cluster.

For full documentation, see [cluster.md](cluster.md).

```bash
cardinal service create --name web --replicas 3 --port 80:80 nginx:alpine
cardinal service ls
cardinal service scale web 5
cardinal service update web --image nginx:1.25
cardinal service rm web
```

---

## FaaS / Serverless

cardinal can deploy container images as serverless functions with auto-scaling and scale-to-zero.

For full documentation, see [faas.md](faas.md).

```bash
# Deploy a function
cardinal fn deploy --name hello --port 8080 --timeout 30 --idle 300 ghcr.io/myorg/hello-func

# Invoke
cardinal fn call hello --data '{"name": "cardinal"}'

# List functions
cardinal fn ls

# Remove
cardinal fn rm hello
```

---

## Blueprints

Blueprints are pre-configured container templates that can be installed from repositories.

```bash
# List available blueprints
cardinal blueprint list

# Show blueprint details with examples
cardinal blueprint info mysql-8
cardinal blueprint info minecraft-server

# Install a blueprint
cardinal blueprint install nginx-proxy

# Add a custom blueprint repository
cardinal blueprint repo add https://github.com/user/my-blueprints

# List repositories
cardinal blueprint repo list

# Remove a repository
cardinal blueprint repo remove my-blueprints
```

---

## Events

Stream container lifecycle events in JSON format.

```bash
cardinal events                          # live stream
cardinal events --since "2026-07-07 12:00:00"  # events since timestamp
```

Events: `start`, `stop`, `kill`, `oom`, `healthcheck_failed`, etc.

---

## System Commands

### `cardinal system prune`

Remove unused containers and images.

```bash
cardinal system prune
```

### `cardinal update [--check]`

Check for updates and self-update the cardinal binary.

```bash
cardinal update              # update to latest version
cardinal update --check      # only check version
```

### `cardinal version`

Show version information.

```bash
cardinal version
```

---

## Architecture

```
cardinal run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc cardinal init <id>
  │   └─ cardinal init → pivot_root to overlay → setup /proc/lo/eth0 → exec CMD
  └─ cardinal console-serve <id>
      ├─ reads stdout pipe
      ├─ writes to log file
      ├─ listens on Unix socket
      └─ broadcasts to all attach clients
```

### Key Concepts

| Concept | Description |
|---|---|
| **Image** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Pulled once via `cardinal pull`. |
| **Container** | Image + writable overlay layer. Changes live in the overlay, not the image. |
| **Overlay** | Diff layer on top of the image. Persists across restarts — packages stay installed. |
| **Volume** | Host bind mount into the container. `-v /data/mybot:/bot` mounts `/data/mybot` as `/bot`. |
| **Network** | Every container gets IP `10.0.2.X` on bridge `cardinal0`. Host at `10.0.2.1`. |

### Execution Flow

1. `cardinal run` pulls the image (if not cached)
2. Creates an overlay filesystem (lower=image rootfs, upper=container layer, merged=container root)
3. Runs `unshare` with PID, mount, net, UTS, IPC namespaces
4. Inside the namespace, `cardinal init` does `pivot_root` to the overlay, mounts /proc, sets up networking
5. Executes the container command (CMD or `--startup` script)
6. If detached, `cardinal console-serve` captures stdout and serves it via Unix socket for `cardinal attach`

---

## Troubleshooting

### cardinal rm -f <container> hangs

If a container won't stop, try:

```bash
# Force kill the process
kill -9 $(cat /root/.cardinal/containers/*.json | grep -o '"pid":[0-9]*' | grep -o '[0-9]*')

# Then remove
cardinal rm -f <container>

# Manual cleanup if state files are corrupt
rm -f /root/.cardinal/containers/<id>.json
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
ip link show cardinal0

# Ensure IP forwarding is enabled
sysctl net.ipv4.ip_forward

# Reinstall network base
cardinal system prune && cardinal pull alpine && cardinal run --rm alpine ping 8.8.8.8
```

### Port mapping not working

```bash
# Check iptables rules
iptables -t nat -L -n | grep cardinal

# UFW may block ports — check ufw status
ufw status
```

### Rootless mode

cardinal supports rootless execution on systems with `newuidmap`/`newgidmap`.
Rootless containers use userspace networking (slirp4netns-style).

### Comparison with Docker

| Feature | cardinal | Docker |
|---|---|---|
| Daemon | No daemon | dockerd required |
| Binary size | ~5 MB | ~100+ MB |
| Namespaces | PID, Mount, Net, UTS, IPC | All |
| Bridge network | cardinal0 (10.0.2.0/24) | docker0 |
| Port mapping | iptables DNAT | iptables DNAT |
| Auto-start | persistent systemd supervisor | systemd dockerd |
| Image format | OCI/Docker V2 | OCI/Docker V2 |
