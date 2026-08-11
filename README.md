<p align="center">
  <img src="https://img.shields.io/badge/version-1.22.19-blue?style=flat-square">
  <img src="https://img.shields.io/badge/go-1.25%2B-00ADD8?style=flat-square&logo=go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square">
  <img src="https://img.shields.io/badge/no%20daemon-%E2%9C%93-brightgreen?style=flat-square">
</p>

<h1 align="center">dck — Lightweight Container Runtime</h1>

<p align="center">
  <b>No daemon. No Docker. Just containers.</b><br>
  ~5 MB static binary · zero daemon · OCI images · bridge networking · cluster · FaaS
</p>

<p align="center">
  <a href="CONTRIBUTORS.md">👥 Contributors</a> ·
  <a href="https://github.com/animesao/dck/graphs/contributors">GitHub contributor graph</a> ·
  <a href="LICENSE">MIT License</a>
</p>

```bash
dck run --rm alpine echo "hello from dck!"
dck run -d -n web -p 8080:80 nginx:alpine
curl http://localhost:8080
```

---

## Quick Start

```bash
# Install via apt (Debian/Ubuntu)
curl -sSL https://raw.githubusercontent.com/animesao/dck/main/scripts/install-apt.sh | sudo bash

# Or build from source (Linux)
curl -sSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash

# dck-client
curl -sSL https://raw.githubusercontent.com/animesao/dck-client/main/install.sh | sudo bash

# Pull & run
dck pull nginx:alpine
dck run -d -n web -p 8080:80 nginx:alpine

# Check
dck ps
curl http://localhost:8080

# Logs & exec
dck logs web
dck exec web cat /etc/hostname

# Interactive
dck run -i -t alpine sh

# Stop & remove
dck stop web && dck rm web
```

**Requirements:** Linux with `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep` +
PID/Mount/Net/UTS/IPC namespaces + overlayfs.

---

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Image** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Pulled once via `dck pull`. |
| **Container** | Image + writable overlay layer. Changes live in the overlay, not the image. |
| **Overlay** | Diff layer on top of the image. Persists across restarts — packages stay installed. Stays mounted after `stop` — browse with `dck fs`. |
| **Volume** | Host bind mount into the container. `-v /opt/mybot:/bot` mounts `/opt/mybot` as `/bot`. |
| **Network** | Every container gets IP `10.0.2.X` on bridge `dck0`. Host at `10.0.2.1`. |

```
Host:        dck0  10.0.2.1/24
Container A: eth0  10.0.2.2
Container B: eth0  10.0.2.3

A → host:      ping 10.0.2.1      (host is gateway)
host → A:      ping 10.0.2.2      (host has route)
A → B:         ping 10.0.2.3      (via bridge)
A → B's port:  curl 10.0.2.1:8080 (DNAT: host_port → B:container_port)
```

---

## Usage

### Image Commands

```bash
dck pull alpine                    # Pull image
dck pull nginx:alpine              # With tag
dck search nginx                   # Search Docker Hub
dck images                         # List local images
dck rmi nginx:alpine               # Remove image
```

### Container Lifecycle

```bash
dck run --rm alpine echo hi                 # One-shot
dck run -d -n web -p 80:80 nginx            # Detached
dck run -i -t alpine sh                       # Interactive
dck ps -a                                   # List all containers
dck stop web                                # Stop (files remain accessible via dck fs)
dck start web                               # Start stopped
dck restart web                             # Restart
dck rm -f web                               # Force remove (deletes files)
dck rename web web-new                      # Rename container
dck set web --memory 2g --cpus 4            # Change container params (preserves data)
dck set web --restart always                # Enable auto-restart
dck set web --restart-delay 1m             # Wait 1 minute before recovery
dck system prune                            # Remove unused containers and images
dck info                                    # System information
dck commit web my-image:v1                  # Create image from container
```

### Logs & Attach

A new container start creates a fresh dck stdout/stderr log. Application-owned logs in bind mounts or named volumes are preserved.

```bash
dck logs web                                # Last output
dck logs -f web                             # Follow
dck attach web                              # Recent output + live stdin/stdout
dck fs ls web /etc/nginx                    # List files in container
dck fs cat web /etc/nginx/conf.d/default.conf  # Show file
dck fs find web --name "*.conf"             # Search files
dck exec web cat /etc/hostname              # Run command inside
dck exec -i -t web /bin/sh                    # Interactive shell
dck console web                             # Auto-detect shell
dck top web                                 # Processes inside container
```

### Filesystem Browser

Browse container files without starting a shell:

```bash
dck fs ls <container> [path]              # List files
dck fs cat <container> <path>             # Show file content
dck fs tree <container> [path]            # Directory tree
dck fs find <container> [path] [flags]    # Find files
  --name <pattern>    Filter by name (glob, e.g. "*.conf")
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
```

Works on both **running** and **stopped** containers — overlay stays mounted after `dck stop`.

### File Operations

Copy files between host and container without rebuilding:

```bash
# Copy from host to container
dck cp app.py web:/app/                     # Single file
dck cp ./static/ web:/usr/share/nginx/html/ # Directory
dck cp ./bot.py discord-bot:/bot/           # Bot code

# Copy from container to host
dck cp web:/etc/nginx/nginx.conf .          # Backup config
dck cp web:/var/log/nginx/ ./logs/          # Backup logs

# Upload files to running container
dck cp ./index.html web:/usr/share/nginx/html/index.html
dck cp ./config.yml myapp:/etc/app/config.yml
```

Use `-v` (bind mount) for live file sharing — changes on host are instantly visible inside the container.

`dck attach` is **Ctrl+C safe** — container keeps running.

> **exec vs attach:** `attach` connects to the main process stdin/stdout. `exec` runs a new command inside the container. `console` is a shortcut for `exec -i -t` with auto-detected shell.

### Options

| Flag | Description |
|------|-------------|
| `-d` | Detach (background) |
| `-n` | Container name |
| `-p` | Port mapping `host:container` |
| `-v` | Volume mount `src:dst` |
| `-e` | Environment variable (repeatable) |
| `-i` | Interactive (keep stdin) |
| `-t` | Allocate TTY |
| `--rm` | Auto-remove on exit |
| `--restart` | Restart policy: `no`, `always`, `on-failure`, `unless-stopped` |
| `--restart-delay` | Wait before automatic restart (e.g. `10s`, `1m`) |
| `--memory` | RAM limit (e.g. `512m`, `2g`) |
| `--cpus` | CPU limit (e.g. `1.5`, `4`) |
| `--disk` | Disk limit (e.g. `10G`) |

| `-h` | Hostname |
| `--startup` | Startup script (inline or `@file`) — overrides CMD |
| `--healthcheck-cmd` | Health check command |
| `--healthcheck-interval` | Health check interval (seconds) |
| `--healthcheck-retries` | Health check retries |
| `--healthcheck-timeout` | Health check timeout (seconds) |

---

## Examples

### Web Server

```bash
dck run -d --restart always -n web -p 80:80 nginx:alpine
curl localhost
```

### Python Flask App

```bash
mkdir -p /opt/flask-app && cd /opt/flask-app
cat > app.py << 'EOF'
from flask import Flask
app = Flask(__name__)
@app.route('/')
def hello():
    return 'Hello from dck!'
if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
EOF
echo "flask==3.0.0" > requirements.txt

dck run -d --restart always \
  -n flask -p 5000:5000 \
  -v /opt/flask-app:/app \
  python:3.11-slim sh -c "\
    pip install -r /app/requirements.txt && \
    python /app/app.py"
curl http://localhost:5000
```

### PostgreSQL

```bash
dck run -d --restart always \
  -n pg -p 5432:5432 \
  -v pg_data:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=myapp \
  postgres:16
psql -h localhost -U postgres -d myapp
```

### MySQL

```bash
dck run -d --restart always \
  -n mysql -p 3306:3306 \
  -v mysql_data:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=myapp \
  mysql:8
mysql -h localhost -u root -prootpass myapp
```

### Redis

```bash
dck run -d --restart always \
  -n redis -p 6379:6379 \
  -v redis_data:/data \
  redis:7 --appendonly yes
redis-cli -h localhost ping
```

### Minecraft Server

```bash
# Pre-built image (itzg/minecraft-server)
dck run -d --restart always \
  -n mc -p 25565:25565 \
  -v mc_data:/data \
  -e EULA=TRUE -e TYPE=PAPER -e VERSION=1.20.4 \
  -e MEMORY=2G -e DIFFICULTY=normal \
  itzg/minecraft-server
```

### Minecraft Server (чистый Java + `--startup`)

Сначала создай скрипт `mc-startup.sh`:

```bash
#!/bin/bash

# ==============================================================
# Paper Minecraft Server download and startup script
# Version: 1.21.11 (build 116)
# ==============================================================

set -e

# --- Version and URL (your direct link) ---
SERVER_JAR="paper-1.21.11-116.jar"
API_URL="https://fill-data.papermc.io/v1/objects/e708e8c132dc143ffd73528cccb9532e2eb17628b1a0eee74469bf466c7003f8/paper-1.21.11-116.jar"

# --- Java ---
JAVA_CMD="java"
JDK_DIR="./jdk"

check_java_version() {
    local cmd="$1"
    if ! command -v "$cmd" &>/dev/null; then
        return 1
    fi
    local ver
    ver=$("$cmd" -version 2>&1 | head -1 | cut -d '"' -f2 | sed 's/^1\.//' | cut -d '.' -f1)
    [ "$ver" -ge 21 ]
}

# --- Local Java ---
if [ -f "$JDK_DIR/bin/java" ]; then
    echo "ℹ️  Found local Java in $JDK_DIR"
    if check_java_version "$JDK_DIR/bin/java"; then
        JAVA_CMD="$JDK_DIR/bin/java"
        echo "✅ Using local Java 21+"
    else
        echo "⚠️  Local Java is outdated. Removing it."
        rm -rf "$JDK_DIR"
    fi
fi

# --- System Java or download ---
if [ "$JAVA_CMD" = "java" ]; then
    if check_java_version "java"; then
        echo "✅ Found system Java 21+"
    else
        echo "⬇️  Downloading Java 21..."
        JDK_URL="https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.2%2B13/OpenJDK21U-jdk_x64_linux_hotspot_21.0.2_13.tar.gz"
        JDK_TAR="OpenJDK21U-jdk_x64_linux_hotspot_21.0.2_13.tar.gz"
        curl -# -L -o "$JDK_TAR" "$JDK_URL" || { echo "❌ Failed to download Java"; exit 1; }
        mkdir -p "$JDK_DIR"
        tar -xzf "$JDK_TAR" -C "$JDK_DIR" --strip-components=1 || { echo "❌ Failed to extract Java"; exit 1; }
        rm -f "$JDK_TAR"
        [ -f "$JDK_DIR/bin/java" ] || { echo "❌ Java not found after extraction"; exit 1; }
        JAVA_CMD="$JDK_DIR/bin/java"
        echo "✅ Java 21 installed locally."
    fi
fi

# --- Final Java check ---
[ -x "$JAVA_CMD" ] || { echo "❌ Error: Java not found ($JAVA_CMD)"; exit 1; }
echo "🔍 Using Java: $("$JAVA_CMD" -version 2>&1 | head -1)"

# --- JAR validation (ZIP signature) ---
is_jar_valid() {
    local f="$1"
    [ -f "$f" ] || return 1
    local hex
    hex=$(dd if="$f" bs=1 count=4 2>/dev/null | od -An -tx1 | tr -d ' ')
    [ "$hex" = "504b0304" ]
}

# --- Download Paper with response check ---
download_paper() {
    echo "⬇️  Downloading Paper 1.21.11 (build 116)..."
    
    http_code=$(curl -s -L -w "%{http_code}" -o "$SERVER_JAR" "$API_URL")
    
    if [ "$http_code" -ne 200 ]; then
        echo "❌ HTTP error $http_code."
        if [ -f "$SERVER_JAR" ]; then
            echo "   Response content (first 5 lines):"
            head -n 5 "$SERVER_JAR"
        fi
        rm -f "$SERVER_JAR"
        return 1
    fi

    if is_jar_valid "$SERVER_JAR"; then
        echo "✅ Download successful, JAR is valid."
        return 0
    else
        echo "❌ Downloaded file is corrupted or not a JAR."
        rm -f "$SERVER_JAR"
        return 1
    fi
}

# --- Download logic ---
if [ -f "$SERVER_JAR" ] && is_jar_valid "$SERVER_JAR"; then
    echo "ℹ️  File $SERVER_JAR already exists and is valid."
else
    [ -f "$SERVER_JAR" ] && rm -f "$SERVER_JAR"
    if ! download_paper; then
        echo "⚠️  First attempt failed. Retrying in 5 seconds..."
        sleep 5
        if ! download_paper; then
            echo "❌ Failed to download a valid JAR after two attempts."
            exit 1
        fi
    fi
fi

# --- EULA ---
if [ ! -f "eula.txt" ]; then
    echo "📄 Creating eula.txt..."
    echo "eula=true" > eula.txt
else
    echo "ℹ️  eula.txt already exists."
fi

# --- Memory settings ---
MAX_PERCENT=${MAX_RAM_PERCENT:-80.0}
INIT_PERCENT=${INIT_RAM_PERCENT:-40.0}
echo "🧠 JVM: MaxRAMPercentage=$MAX_PERCENT%, InitialRAMPercentage=$INIT_PERCENT%"

# --- Launch ---
echo "🚀 Starting Paper 1.21.11 (build 116) server..."
exec "$JAVA_CMD" -XX:MaxRAMPercentage="$MAX_PERCENT" -XX:InitialRAMPercentage="$INIT_PERCENT" -jar "$SERVER_JAR" nogui
```

Запуск:

```bash
dck run -d --restart always \
  -n mc-paper -p 25565:25565 \
  -v mc_data:/data --memory 4G --cpus 4 \
  --startup @mc-startup.sh \
  eclipse-temurin:21-jdk
```

More Minecraft examples (modded servers, custom JARs, backups) → [docs/en/websites.md](docs/en/websites.md#minecraft-server)

---

### Bots (Telegram, Discord)

Full bot deployment guide → [docs/en/bots.md](docs/en/bots.md)

---

### Copy files to container

Upload your website, bot code, or configs into a running container:

```bash
# Website files
dck cp ./index.html mc:/usr/share/nginx/html/
dck cp ./style.css mc:/usr/share/nginx/html/

# Bot code
dck cp ./bot.py discord-bot:/bot/
dck cp ./config.yml tg-bot:/bot/

# App configs
dck cp ./nginx.conf web:/etc/nginx/conf.d/default.conf

# Entire directories
dck cp ./static/ web:/usr/share/nginx/html/static/
```

See [deployment docs](docs/en/websites.md#file-operations) for more.

### Node.js App

```bash
mkdir -p /opt/node-app && cd /opt/node-app
cat > index.js << 'EOF'
const http = require('http');
http.createServer((req, res) => res.end('Hello from dck!\n')).listen(3000);
EOF

dck run -d --restart always \
  -n node-app -p 3000:3000 \
  -v /opt/node-app:/app \
  node:20 node /app/index.js
curl http://localhost:3000
```

### Discord Bot

```bash
mkdir -p /opt/discord-bot && cd /opt/discord-bot

cat > bot.py << 'EOF'
import os, discord
from discord.ext import commands
TOKEN = os.environ["BOT_TOKEN"]
intents = discord.Intents.default()
intents.message_content = True
bot = commands.Bot(command_prefix="!", intents=intents)
@bot.event
async def on_ready():
    print(f"Logged in as {bot.user}")
@bot.command()
async def ping(ctx):
    await ctx.send("pong")
bot.run(TOKEN)
EOF
echo "discord.py==2.4.0" > requirements.txt

dck run -d --restart always \
  -n discord-bot \
  -v /opt/discord-bot:/bot \
  --workdir /bot \
  -e BOT_TOKEN=your_token_here \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

### Telegram Bot

```bash
mkdir -p /opt/tg-bot && cd /opt/tg-bot

cat > bot.py << 'EOF'
import os
from telegram import Update
from telegram.ext import Application, CommandHandler
TOKEN = os.environ["BOT_TOKEN"]
async def start(update: Update, context):
    await update.message.reply_text("Hello from dck!")
async def ping(update: Update, context):
    await update.message.reply_text("pong")
app = Application.builder().token(TOKEN).build()
app.add_handler(CommandHandler("start", start))
app.add_handler(CommandHandler("ping", ping))
app.run_polling()
EOF
echo "python-telegram-bot==20.7" > requirements.txt

dck run -d --restart always \
  -n tg-bot \
  -v /opt/tg-bot:/bot \
  --workdir /bot \
  -e BOT_TOKEN=your_token_here \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

### Bot + Database

```bash
# 1. PostgreSQL
dck run -d --restart always \
  -n bot-db \
  -v bot_pgdata:/var/lib/postgresql/data \
  -e POSTGRES_DB=botdb \
  -e POSTGRES_USER=bot -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Bot connects via 10.0.2.1
dck run -d --restart always \
  -n db-bot \
  -v /opt/mybot:/bot \
  -e BOT_TOKEN=token -e DB_HOST=10.0.2.1 \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

Packages install into the overlay and persist across restarts.

---

## dck-wings — Container Management Agent

[dck-wings](https://github.com/animesao/dck-wings) is a REST API daemon for managing containers remotely. It runs as a systemd service and allows frontends (like dck-panel) to control containers over HTTP.

```bash
# Install
bash <(curl -sfL https://raw.githubusercontent.com/animesao/dck-wings/main/install.sh)

# Start
systemctl enable --now dck-wings

# API (auth via Bearer token from /etc/dck-wings/config.toml)
curl -H "Authorization: Bearer <api_key>" http://localhost:8080/api/containers
```

---



## Dynamic Port Management

Add or remove port mappings on running containers without restart.

```bash
# Add a port
dck port add <container> <host>:<container>[/proto]

# Remove a port
dck port remove <container> <host>[/proto]
dck port rm <container> <host>[/proto]     # alias
```

- Applies iptables DNAT rules instantly — no restart needed
- Ports persist in container state across restarts





---

## Auto-Start on Boot

Containers with `--restart always` start automatically after reboot — dck auto-installs the systemd service when you `dck run --restart always`, `dck set <c> --restart always`, or `dck up`.

```bash
dck bootstrap --install      # manually install systemd service
dck bootstrap --remove       # remove it
```

```
System boot → systemd → dck-bootstrap.service → dck bootstrap
  └─ For each container with restart=always:
      1. Setup overlayfs
      2. Run unshare with namespaces
      3. Setup veth + iptables
```

---

## dck.toml (Multi-Container Config)

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

```bash
dck up              # Create/start all containers
dck up web          # Start only web
dck down            # Stop/remove all
dck down -a         # Remove ALL containers (ignore config)
```

### Config Fields

| Field | Description | Example |
|-------|-------------|---------|
| `image` | Container image (required) | `"nginx:alpine"` |
| `command` | Startup command | `"python3 app.py"` |
| `ports` | Port mappings | `["443:80", "3000:3000"]` |
| `volumes` | Volume mounts | `["./data:/data"]` |
| `env` | Environment variables | `{ KEY = "val" }` |
| `restart` | Restart policy | `"always"` (default) |
| `hostname` | Container hostname | `"myserver"` |
| `healthcheck` | Health check config | `{ cmd = "...", interval = 30, retries = 3, timeout = 5 }` |

Healthcheck runs the command inside the container at the given interval. After `retries` consecutive failures, the container is killed and restarted.

---

## Startup Scripts

Use `--startup` to run a custom script instead of the image's default command:

```bash
# Inline script
dck run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Load from file
dck run -d --startup @./myscript.sh ubuntu
```

The script is written to `/startup.sh` inside the container and executed via `/bin/sh`. When a startup script is present, it **overrides** the normal `CMD`/`entrypoint`.

The following environment variables are injected automatically for startup scripts:

| Variable | Description |
|----------|-------------|
| `DCK_CONTAINER_ID` | Container ID |
| `DCK_CONTAINER_NAME` | Container name |
| `DCK_IMAGE_NAME` | Image name |
| `DCK_IMAGE_TAG` | Image tag |
| `DCK_HOSTNAME` | Container hostname |
| `DCK_MEMORY` | Memory limit (bytes) |
| `DCK_CPU` | CPU limit (cores) |
| `DCK_IP` | Container IP address |
| `DCK_RESTART` | Restart policy |

## Architecture

```
Storage: `$DCK_DATA_DIR` (default: `/root/.dck/`)

images/        OCI rootfs per tag
containers/    State JSON files
overlay/       upper/work/merged per container
logs/          Container stdout/stderr
consoles/      Unix sockets for attach
networks/      IP allocation pool

dck run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc dck init <id>
  │   └─ dck init → pivot_root to overlay → setup /proc/lo/eth0 → exec CMD
  └─ dck console-serve <id>
      ├─ reads stdout pipe
      ├─ writes to log file
      ├─ listens on Unix socket
      └─ broadcasts to all attach clients
```

---

## Comparison

| Feature | dck | Docker |
|---------|-----|--------|
| Daemon | No daemon | dockerd required |
| Binary size | ~5 MB | ~100+ MB |
| Namespaces | PID, Mount, Net, UTS, IPC | All |
| Bridge network | dck0 (10.0.2.0/24) | docker0 |
| Port mapping | iptables DNAT | iptables DNAT |
| Auto-start | systemd oneshot | systemd dockerd |
| Image format | OCI/Docker V2 | OCI/Docker V2 |
| Multi-stage build | ✅ | ✅ |
| Compose depends_on | ✅ | ✅ |
| Cluster orchestration | ✅ | ✅ (Swarm) |
| Rolling updates | ✅ | ✅ |

---

## Changelog

**v1.22.19** — Current release. OCI image extraction, forward symlink targets, protected bind-source validation, fresh dck logs on every new container start, cluster orchestration, FaaS, blueprints, services, Compose, healthchecks, startup scripts, dynamic ports, events, stats, and Docker-compatible REST API.

**v1.20.0** — Dynamic port management (`dck port add/rm`). Russian (ru) docs.

**v1.15.0** — `pivot_root` security fix. `dck stop --all`. `dck exec -i/-t` flags. Disk limit fix.

**v1.14.0** — Disk limit support (`--disk`). Multi-arch image resolution.

**v1.13.0** — `--startup` flag, `--healthcheck-*` flags, DCK_* env vars, cgroups v2 resource limits.

**v1.11.0** — Debian packaging, APT repository, release workflow.

**v1.10.0** — `dck stats` command (live CPU/RAM/IO/PIDs from cgroup v2).

**v1.4.7** — `dck attach` rewritten (Unix socket, history + live, Ctrl+C safe).

**v1.3.0** — `dck.toml` config, `dck up`/`dck down`.

**v1.1.0** — First stable release.

---

## Updating

```bash
dck update
```

Downloads the latest binary and replaces `/usr/local/bin/dck`.

---

## Documentation

- [Contributors and contribution guide](CONTRIBUTORS.md)

| English | Русский |
|---|---|
| [Running Guide](docs/en/running.md) | [Руководство по запуску](docs/ru/running.md) |
| [Usage & Commands](docs/en/usage.md) | [Команды и использование](docs/ru/usage.md) |
| [Deploying Websites](docs/en/websites.md) | [Развёртывание сайтов](docs/ru/websites.md) |
| [Bots (Telegram, Discord)](docs/en/bots.md) | [Боты (Telegram, Discord)](docs/ru/bots.md) |
| [Compose / Deployment](docs/en/compose.md) | [Compose / Развёртывание](docs/ru/compose.md) |
| [Compose Examples (15 configs)](docs/en/compose-examples.md) | [Примеры Compose](docs/ru/compose-examples.md) |
| [Cluster Orchestration](docs/en/cluster.md) | [Кластерная оркестрация](docs/ru/cluster.md) |
| [FaaS / Serverless](docs/en/faas.md) | [FaaS / Serverless](docs/ru/faas.md) |
| [Build & Versioning](docs/build.md) | [Сборка и версионирование](docs/build.md) |

---

## Uninstall

```bash
dck bootstrap --remove
rm /usr/local/bin/dck
rm -rf ~/.dck
```

## Contributors

The project maintainer and verified contributors are listed in
[`CONTRIBUTORS.md`](CONTRIBUTORS.md). GitHub's contributor graph is generated
from commit authors, so contributors should use their own GitHub identity when
opening pull requests or submitting commits.

- [animesao](https://github.com/animesao) — maintainer and primary author.
- `github-actions[bot]` — automated release and versioning workflow.

See [CONTRIBUTORS.md](CONTRIBUTORS.md) for contribution rules and the process
for adding new contributors with their permission.

## License

This project is released under the MIT License. The full license text is in
[LICENSE](LICENSE).

Contributor and maintainer attribution: [CONTRIBUTORS.md](CONTRIBUTORS.md).
