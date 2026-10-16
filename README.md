<!-- cardinal-version:start -->
**Documentation version:** `2.2.0`
**Project release:** `v2.2.0`
<!-- cardinal-version:end -->

<p align="center">
  <img src="img/cardinal.png" alt="cardinal logo" width="200">
</p>

<p align="center">
  <!-- cardinal-version-badge:start -->
  <img src="https://img.shields.io/badge/version-v2.3.14-blue?style=flat-square">
  <!-- cardinal-version-badge:end -->
  <img src="https://img.shields.io/badge/go-1.26%2B-00ADD8?style=flat-square&logo=go">
  <img src="https://img.shields.io/badge/license-Apache--2.0-green?style=flat-square">
  <img src="https://img.shields.io/badge/no%20daemon-%E2%9C%93-brightgreen?style=flat-square">
</p>

<h1 align="center">cardinal — Lightweight Container Runtime</h1>

<p align="center">
  <b>No daemon. No Docker. Just containers.</b><br>
  ~5 MB static binary · zero daemon · OCI images · bridge networking · cluster · FaaS
</p>

<p align="center">
  <a href="CONTRIBUTING.md">🤝 Contributing</a> ·
  <a href="https://github.com/animesao/cardinal/graphs/contributors">GitHub contributors</a> ·
  <a href="LICENSE">Apache-2.0 License</a>
</p>

```bash
cardinal run --rm alpine echo "hello from cardinal!"
cardinal run -d -n web -p 8081:80 nginx:alpine
curl http://localhost:8081
```

---

## Quick Start

```bash
# Universal installer (Linux distributions) — downloads only from GitHub
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash

# Debian/Ubuntu APT repository installer (optional)
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/install-apt.sh | sudo bash

# Pull & run
cardinal pull nginx:alpine
cardinal run -d -n web -p 8081:80 nginx:alpine

# Check
cardinal ps
curl http://localhost:8081

# Logs & exec
cardinal logs web
cardinal exec web cat /etc/hostname

# Interactive
cardinal run -i -t alpine sh

# Stop & remove
cardinal stop web && cardinal rm web
```

**Requirements:** Linux with `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep` +
PID/Mount/Net/UTS/IPC namespaces + overlayfs.

> **Reserved host ports:** `cardinal-wings` (the panel agent) listens on
> `127.0.0.1:8080` by default and `cardinal serve` on `2375`, so the
> examples above map nginx to host port `8081`. `cardinal run` warns when
> the requested host port is already bound on the host, and refuses a port
> already mapped by another container. If `curl http://localhost:8080`
> returns `{"error":"missing bearer token"}`, you reached wings — not your
> container (check with `ss -tlnp | grep 8080` and use another host port,
> e.g. `-p 8081:80`).

### Release formats

GitHub Releases provide native `.deb`, `.rpm`, `.pkg.tar.zst`, and `.apk`
packages for `amd64`, `arm64`, and `armv6`, `.snap` packages for `amd64` and
`arm64`, plus self-contained AppImages for `amd64` and `arm64`. AppImage needs
no package manager, but cardinal still requires the Linux kernel features listed
above. True ARMv6 hosts should use the raw `cardinal-linux-armv6` binary or its
`.tar.gz` archive because the standard AppImage runtime does not support ARMv6.

On a Linux desktop, double-clicking the AppImage opens a terminal-based installer
that installs the embedded static binary to `/usr/local/bin/cardinal` and enables
the systemd supervisor when available. The original AppImage remains portable.
You can start the same installer explicitly with

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Could not determine the latest release" >&2; exit 1; }
FILE="cardinal-${TAG#v}-linux-amd64.AppImage"
curl -fL -o "$FILE" "https://github.com/animesao/cardinal/releases/download/$TAG/$FILE"
chmod +x "$FILE"
"./$FILE" --install
```
The AppImage remains a CLI runtime; pass a cardinal command such as `version` or
`run` for normal portable use. On a headless VPS, run the AppImage from SSH
instead of double-clicking it.

---

## Running with `--image` / `--cmd` / `--workdir`

Вместо позиционных аргументов можно передать образ, команду и рабочую директорию через флаги. Это удобно для длинных команд и скриптов:

```text
cardinal run -d \
  --image ОБРАЗ \
  --cmd "КОМАНДА" \
  --workdir /path \
  --restart always \
  -n ИМЯ -p ПОРТ \
  -v SRC:DST --memory 4G --cpus 4 \
  --network host
```

### Примеры:

**Minecraft Paper:**

```bash
cardinal run -d --restart always \
  -n mc-paper -p 25565:25565 \
  -v $PWD:/data --memory 4G --cpus 4 \
  -workdir /data \
  -image eclipse-temurin:21-jdk \
  -cmd "java -Xmx3500M -jar paper-1.21.11-116.jar nogui"
```

**Discord-бот:**

```bash
cardinal run -d --restart always \
  -n discord-bot \
  -v /data/bot:/bot --workdir /bot \
  -e BOT_TOKEN=your_token \
  -image python:3.12 \
  -cmd "sh -c 'pip install -r /bot/requirements.txt && exec python /bot/bot.py'"
```

**Telegram-бот:**

```bash
cardinal run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot --workdir /bot \
  -e BOT_TOKEN=your_token \
  -image python:3.12 \
  -cmd "sh -c 'pip install -r /bot/requirements.txt && exec python /bot/bot.py'"
```

**PostgreSQL:**

```bash
cardinal run -d --restart always \
  -n postgres -p 5432:5432 \
  -v pg_data:/var/lib/postgresql/data \
  -e POSTGRES_DB=myapp -e POSTGRES_PASSWORD=secret \
  -image postgres:16
```

**Redis:**

```bash
cardinal run -d --restart always \
  -n redis -p 6379:6379 \
  -v redis_data:/data \
  -image redis:7 \
  -cmd "redis-server --appendonly yes"
```

**Nginx (статический сайт):**

```bash
cardinal run -d --restart always \
  -n web -p 8081:80 \
  -v /data/site:/usr/share/nginx/html \
  -network host \
  -image nginx:alpine
```

> **Сетевой доступ:** bridge-контейнеры получают интернет и DNS из коробки (по умолчанию `8.8.8.8`/`8.8.4.4`, свой сервер — через `--dns`). `-network host` нужен только если приложение должно слушать хостовые интерфейсы напрямую.

---

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Image** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Pulled once via `cardinal pull`. |
| **Container** | Image + writable overlay layer. Changes live in the overlay, not the image. |
| **Overlay** | Diff layer on top of the image. Persists across restarts — packages stay installed. Stays mounted after `stop` — browse with `cardinal fs`. |
| **Volume** | Host bind mount into the container. `-v /data/mybot:/bot` mounts `/data/mybot` as `/bot`. |
| **Network** | Every container gets IP `10.0.2.X` on bridge `cardinal0`. Host at `10.0.2.1`. |

```
Host:        cardinal0  10.0.2.1/24
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
cardinal pull alpine                    # Pull image
cardinal pull nginx:alpine              # With tag
cardinal search nginx                   # Search Docker Hub
cardinal images                         # List local images
cardinal rmi nginx:alpine               # Remove image
cardinal verify nginx:alpine            # Verify image digests
```

### Container Lifecycle

```bash
cardinal run --rm alpine echo hi                 # One-shot
cardinal run -d -n web -p 80:80 nginx            # Detached
cardinal run -i -t alpine sh                       # Interactive
cardinal ps -a                                   # List all containers
cardinal stop web                                # Stop (files remain accessible via cardinal fs)
cardinal start web                               # Start stopped
cardinal restart web                             # Restart
cardinal rm -f web                               # Force remove (deletes files)
cardinal rename web web-new                      # Rename container
cardinal set web --memory 2g --cpus 4            # Change container params (preserves data)
cardinal set web --restart always                # Enable auto-restart
cardinal set web --restart-delay 1m             # Wait 1 minute before recovery
cardinal set web --restart-max-attempts 10      # Allow 10 crash restarts before blocking
cardinal backup enable web --interval 6h --retention 14  # Scheduled backups
cardinal backup status web                       # Show backup settings
cardinal backup disable web                      # Disable scheduled backups
cardinal system df                               # Show disk usage by images, containers, volumes
cardinal system prune                            # Remove unused containers and images
cardinal info                                    # System information
cardinal commit web my-image:v1                  # Create image from container
```

### System Disk Usage

Check how much disk space cardinal is using:

```bash
cardinal system df
```

Output:
```
TYPE            TOTAL     SIZE      PATH
Images (3)     45.2 GB   12 items  /root/.cardinal/images
Containers (5) 2.1 GB    10 items  /root/.cardinal/containers
Overlay (5)    1.8 GB    20 items  /root/.cardinal/overlay
Volumes (2)    512.0 MB  8 items   /root/.cardinal/volumes
Logs (3)       128.5 MB  3 items   /root/.cardinal/logs
Backups        2.3 GB    6 items   /root/.cardinal/backups
Cache          3.1 GB    15 items  /root/.cardinal/cache

Total disk usage: 55.1 GB
Data directory:   /root/.cardinal
```

### Automatic Backups

Scheduled backups are managed by the persistent systemd supervisor. The archive includes the container writable overlay and named volumes, but not host bind mounts. To keep the archive consistent, cardinal briefly stops a running container, creates the backup, then starts it again. Enabling a schedule does not create an archive immediately; the first archive is created after the configured interval.

```bash
cardinal backup enable minecraft --interval 6h --retention 14
cardinal backup status minecraft
cardinal backup list
cardinal backup disable minecraft
```

Backups are stored by default under `$CARDINAL_DATA_DIR/backups/<container>/`. Use a dedicated directory when desired:

```bash
cardinal backup enable minecraft --interval 24h --retention 7 --dir /data/backups/minecraft
```

The supervisor must be installed for scheduled backups to run after the CLI exits:

```bash
cardinal bootstrap --install
```

Manual backups remain available with `cardinal backup create`; restore only into a stopped container. Verify an archive with `cardinal backup verify FILE.tar.gz`.

### Logs & Attach

A new container start creates a fresh cardinal stdout/stderr log. Application-owned logs in bind mounts or named volumes are preserved.

```bash
cardinal logs web                                # Current run
cardinal logs --previous web                     # Previous run
cardinal logs --all web                          # Current + rotated runs
cardinal logs -f web                             # Follow current run
cardinal attach web                              # Recent output + live stdin/stdout
cardinal fs ls web /etc/nginx                    # List files in container
cardinal fs cat web /etc/nginx/conf.d/default.conf  # Show file
cardinal fs find web --name "*.conf"             # Search files
cardinal exec web cat /etc/hostname              # Run command inside
cardinal exec -i -t web /bin/sh                    # Interactive shell
cardinal console web                             # Auto-detect shell
cardinal top web                                 # Processes inside container
```

### Filesystem Browser

Browse container files without starting a shell:

```bash
cardinal fs ls <container> [path]              # List files
cardinal fs cat <container> <path>             # Show file content
cardinal fs tree <container> [path]            # Directory tree
cardinal fs find <container> [path] [flags]    # Find files
  --name <pattern>    Filter by name (glob, e.g. "*.conf")
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
```

Works on both **running** and **stopped** containers — overlay stays mounted after `cardinal stop`.

### File Operations

Copy files between host and container without rebuilding:

```bash
# Copy from host to container
cardinal cp app.py web:/app/                     # Single file
cardinal cp ./static/ web:/usr/share/nginx/html/ # Directory
cardinal cp ./bot.py discord-bot:/bot/           # Bot code

# Copy from container to host
cardinal cp web:/etc/nginx/nginx.conf .          # Backup config
cardinal cp web:/var/log/nginx/ ./logs/          # Backup logs

# Upload files to running container
cardinal cp ./index.html web:/usr/share/nginx/html/index.html
cardinal cp ./config.yml myapp:/etc/app/config.yml
```

Use `-v` (bind mount) for live file sharing — changes on host are instantly visible inside the container.

`cardinal attach` is **Ctrl+C safe** — container keeps running.

> **exec vs attach:** `attach` connects to the main process stdin/stdout. `exec` runs a new command inside the container. `console` is a shortcut for `exec -i -t` with auto-detected shell.

### Options

| Flag | Description |
|------|-------------|
| `-d` | Detach (background) |
| `-n` | Container name |
| `-p` | Port mapping `host:container[/proto]`; host ranges like `8000-8010:80` are supported |
| `-v` | Volume mount `src:dst` (add `:ro`/`:rw` for read-only/read-write) |
| `-e` | Environment variable (repeatable) |
| `-i` | Interactive (keep stdin) |
| `-t` | Allocate TTY |
| `--rm` | Auto-remove on exit |
| `--restart` | Restart policy: `no`, `always`, `on-failure`, `unless-stopped` (`always`/`unless-stopped` are supervised for detached containers) |
| `--restart-delay` | Wait before automatic restart (e.g. `10s`, `1m`) |
| `--restart-max-attempts` | Crash-loop budget: automatic restart is blocked after N failures within the window (default 5) |
| `--restart-window` | Window for the crash-loop budget (e.g. `10m`, `1h`) |
| `--memory` | RAM limit (e.g. `512m`, `2g`) |
| `--cpus` | CPU limit (e.g. `1.5`, `4`) |
| `--disk` | Disk limit (e.g. `10G`) |
| `-h` | Hostname |
| `--startup` | Startup script (inline or `@file`) — overrides CMD |
| `--healthcheck-cmd` | Health check command |
| `--healthcheck-interval` | Health check interval (seconds) |
| `--healthcheck-retries` | Health check retries |
| `--healthcheck-timeout` | Health check timeout (seconds) |
| `--seccomp-profile` | Accepted but not yet enforced (fails closed); planned |
| `--apparmor-profile` | Accepted but not yet enforced (fails closed); planned |
| `--isolated` | Isolate container from other containers (network segmentation) |
| `--encrypted-backup` | Encrypt backup archives with AES-256-GCM |
| `--audit-log` | Enable audit logging for container events |

---

## Examples

### Web Server

```bash
cardinal run -d --restart always -n web -p 80:80 nginx:alpine
curl localhost
```

### Python Flask App

```bash
mkdir -p /data/flask-app && cd /data/flask-app
cat > app.py << 'EOF'
from flask import Flask
app = Flask(__name__)
@app.route('/')
def hello():
    return 'Hello from cardinal!'
if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
EOF
echo "flask==3.0.0" > requirements.txt

cardinal run -d --restart always \
  -n flask -p 5000:5000 \
  -v /data/flask-app:/app \
  python:3.11-slim sh -c "\
    pip install -r /app/requirements.txt && \
    python /app/app.py"
curl http://localhost:5000
```

### PostgreSQL

```bash
cardinal run -d --restart always \
  -n pg -p 5432:5432 \
  -v pg_data:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=myapp \
  postgres:16
psql -h localhost -U postgres -d myapp
```

### MySQL

```bash
cardinal run -d --restart always \
  -n mysql -p 3306:3306 \
  -v mysql_data:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=myapp \
  mysql:8
mysql -h localhost -u root -prootpass myapp
```

### Redis

```bash
cardinal run -d --restart always \
  -n redis -p 6379:6379 \
  -v redis_data:/data \
  redis:7 --appendonly yes
redis-cli -h localhost ping
```

### Minecraft Server

```bash
# Pre-built image (itzg/minecraft-server)
cardinal run -d --restart always \
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

Простой запуск (Java уже в образе, jar на сервере):

```bash
cardinal run -d --restart always \
  -n mc-paper -p 25565:25565 \
  -v $PWD:/data --memory 4G --cpus 4 \
  -workdir /data \
  -network host \
  eclipse-temurin:21-jdk \
  java -Xmx3500M -jar paper-1.21.11-116.jar nogui
```

> **DNS в контейнере:** bridge-контейнеры резолвят внешние хосты из коробки (дефолт `8.8.8.8`/`8.8.4.4`, переопределить — флагом `--dns`). `-network host` нужен только для прямого доступа к сети хоста, не для DNS.

More Minecraft examples (modded servers, custom JARs, backups) → [docs/en/websites.md](docs/en/websites.md#minecraft-server)

---

### Bots (Telegram, Discord)

Full bot deployment guide → [docs/en/bots.md](docs/en/bots.md)

---

### Copy files to container

Upload your website, bot code, or configs into a running container:

```bash
# Website files
cardinal cp ./index.html mc:/usr/share/nginx/html/
cardinal cp ./style.css mc:/usr/share/nginx/html/

# Bot code
cardinal cp ./bot.py discord-bot:/bot/
cardinal cp ./config.yml tg-bot:/bot/

# App configs
cardinal cp ./nginx.conf web:/etc/nginx/conf.d/default.conf

# Entire directories
cardinal cp ./static/ web:/usr/share/nginx/html/static/
```

See [deployment docs](docs/en/websites.md#file-operations) for more.

### Node.js App

```bash
mkdir -p /data/node-app && cd /data/node-app
cat > index.js << 'EOF'
const http = require('http');
http.createServer((req, res) => res.end('Hello from cardinal!\n')).listen(3000);
EOF

cardinal run -d --restart always \
  -n node-app -p 3000:3000 \
  -v /data/node-app:/app \
  node:20 node /app/index.js
curl http://localhost:3000
```

### Discord Bot

```bash
mkdir -p /data/discord-bot && cd /data/discord-bot

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

cardinal run -d --restart always \
  -n discord-bot \
  -v /data/discord-bot:/bot \
  --workdir /bot \
  -e BOT_TOKEN=your_token_here \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

### Telegram Bot

```bash
mkdir -p /data/tg-bot && cd /data/tg-bot

cat > bot.py << 'EOF'
import os
from telegram import Update
from telegram.ext import Application, CommandHandler
TOKEN = os.environ["BOT_TOKEN"]
async def start(update: Update, context):
    await update.message.reply_text("Hello from cardinal!")
async def ping(update: Update, context):
    await update.message.reply_text("pong")
app = Application.builder().token(TOKEN).build()
app.add_handler(CommandHandler("start", start))
app.add_handler(CommandHandler("ping", ping))
app.run_polling()
EOF
echo "python-telegram-bot==20.7" > requirements.txt

cardinal run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot \
  --workdir /bot \
  -e BOT_TOKEN=your_token_here \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

### Bot + Database

```bash
# 1. PostgreSQL
cardinal run -d --restart always \
  -n bot-db \
  -v bot_pgdata:/var/lib/postgresql/data \
  -e POSTGRES_DB=botdb \
  -e POSTGRES_USER=bot -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Bot connects via 10.0.2.1
cardinal run -d --restart always \
  -n db-bot \
  -v /data/mybot:/bot \
  -e BOT_TOKEN=token -e DB_HOST=10.0.2.1 \
  --startup "pip install -r /bot/requirements.txt && exec python /bot/bot.py" \
  python:3.11-slim
```

Packages install into the overlay and persist across restarts cardinal.

---

## cardinal-wings — Container Management Agent

[cardinal-wings](https://github.com/animesao/cardinal-wings) is a REST API daemon for managing containers remotely. It runs as a systemd service and allows frontends (like cardinal-panel) to control containers over HTTP.

```bash
# Install (GitHub Releases; prints URL + API token for the panel)
curl -fsSL https://github.com/animesao/cardinal-wings/releases/latest/download/install.sh -o /tmp/install-wings.sh
sudo bash /tmp/install-wings.sh

# Start
systemctl enable --now cardinal-wings

# API (auth via Bearer token from /etc/cardinal-wings/config.toml)
curl -H "Authorization: Bearer <api_key>" http://localhost:8080/v1/containers
```

> If GitHub is unreachable (e.g. `curl: (28) SSL connection timeout`), install
> [cardinal-wings](https://github.com/animesao/cardinal-wings) manually from its
> GitHub releases. The cardinal installer downloads everything exclusively
> from [the official cardinal repository](https://github.com/animesao/cardinal).

---



## Dynamic Port Management

Add or remove port mappings on running containers without restart.

```bash
# Add a port
cardinal port add <container> <host>:<container>[/proto]

# Remove a port
cardinal port remove <container> <host>[/proto]
cardinal port rm <container> <host>[/proto]     # alias
```

- Applies iptables DNAT rules instantly — no restart needed
- Ports persist in container state across restarts





---

## Auto-Start and Recovery on Boot

Containers with `--restart always` or `--restart unless-stopped` start automatically after reboot. cardinal installs a persistent systemd supervisor when you run a container with an automatic restart policy. The supervisor survives the short-lived `cardinal run -d` command and owns boot recovery; the container monitor applies `--restart-delay` after crashes.

Repeated quick crashes are protected by a crash-loop budget (default 5 restarts, tunable with `--restart-max-attempts` and `--restart-window`). Once the budget is exhausted, automatic restart is blocked — `cardinal inspect NAME` then shows `"restart_blocked": true` — and the container stays stopped until an explicit `cardinal start`.

```bash
cardinal bootstrap --install      # install and start the supervisor
cardinal bootstrap --remove       # stop and remove it
systemctl status cardinal-bootstrap
```

```
System boot → systemd → cardinal-bootstrap.service → cardinal supervisor
  └─ Adopt detached containers with an automatic restart policy
      1. Setup overlayfs
      2. Run unshare with namespaces
      3. Setup veth + iptables
      4. Monitor crashes and apply restart-delay
```

---

## cardinal.toml (Multi-Container Config)

Define containers in a TOML file, start everything with one command.

```toml
[container.web]
image = "nginx:alpine"
ports = ["80:80", "443:80", "8000-8002:8080"]
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
cardinal up              # Create/start all containers
cardinal up web          # Start only web
cardinal down            # Stop/remove all
cardinal down -a         # Remove ALL containers (ignore config)
```

### Config Fields

| Field | Description | Example |
|-------|-------------|---------|
| `image` | Container image (required) | `"nginx:alpine"` |
| `command` | Startup command | `"python3 app.py"` |
| `ports` | Port mappings (ranges like `8000-8010:80` supported) | `["443:80", "3000:3000"]` |
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
cardinal run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Load from file
cardinal run -d --startup @./myscript.sh ubuntu
```

The script is written to `/startup.sh` inside the container and executed via `/bin/sh`. When a startup script is present, it **overrides** the normal `CMD`/`entrypoint`.

The following environment variables are injected automatically for startup scripts:

| Variable | Description |
|----------|-------------|
| `CARDINAL_CONTAINER_ID` | Container ID |
| `CARDINAL_CONTAINER_NAME` | Container name |
| `CARDINAL_IMAGE_NAME` | Image name |
| `CARDINAL_IMAGE_TAG` | Image tag |
| `CARDINAL_HOSTNAME` | Container hostname |
| `CARDINAL_MEMORY` | Memory limit (bytes) |
| `CARDINAL_CPU` | CPU limit (cores) |
| `CARDINAL_IP` | Container IP address |
| `CARDINAL_RESTART` | Restart policy |

## Architecture

```
Storage: `$CARDINAL_DATA_DIR` (default: `/root/.cardinal/`)

images/        OCI rootfs per tag
containers/    State JSON files
overlay/       upper/work/merged per container
logs/          Container stdout/stderr
consoles/      Unix sockets for attach
networks/      IP allocation pool

cardinal run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc cardinal init <id>
  │   └─ cardinal init → pivot_root to overlay → setup /proc/lo/eth0 → exec CMD
  └─ cardinal console-serve <id>
      ├─ reads stdout pipe
      ├─ writes to log file
      ├─ listens on Unix socket
      └─ broadcasts to all attach clients
```

---

## Comparison

| Feature | cardinal | Docker |
|---------|-----|--------|
| Daemon | No daemon | dockerd required |
| Binary size | ~5 MB | ~100+ MB |
| Namespaces | PID, Mount, Net, UTS, IPC | All |
| Bridge network | cardinal0 (10.0.2.0/24) | docker0 |
| Port mapping | iptables DNAT | iptables DNAT |
| Auto-start | persistent systemd supervisor | systemd dockerd |
| Image format | OCI/Docker V2 | OCI/Docker V2 |
| Multi-stage build | ✅ | ✅ |
| Compose depends_on | ✅ | ✅ |
| Cluster orchestration | ✅ | ✅ (Swarm) |
| Rolling updates | ✅ | ✅ |
| Seccomp profile | ❌ (planned, fail-closed) | ✅ |
| AppArmor profile | ❌ (planned, fail-closed) | ✅ |
| Network segmentation | ✅ (--isolated) | ✅ (network policies) |
| Backup encryption | ✅ (AES-256-GCM) | ❌ |
| Audit logging | ✅ | ❌ (auditd integration) |

---

## Remote mode

Point the CLI at a remote `cardinal serve` with `--host/-H` and `--token`
(or `CARDINAL_REMOTE_HOST` / `CARDINAL_TOKEN` env). Besides `ps`, `info` and
`logs --tail`, remote mode supports `start`, `stop`, `restart`, `logs -f`,
`exec`, `stats`, `top`, `inspect` and `events`:

```bash
cardinal --host http://192.168.1.10:2375 start web
cardinal --host http://192.168.1.10:2375 stop web
cardinal --host http://192.168.1.10:2375 stop --all
cardinal --host http://192.168.1.10:2375 restart web
cardinal --host http://192.168.1.10:2375 logs -f web
cardinal --host http://192.168.1.10:2375 exec web cat /etc/hostname
cardinal --host http://192.168.1.10:2375 stats web --no-stream
cardinal --host http://192.168.1.10:2375 top web
cardinal --host http://192.168.1.10:2375 inspect web
cardinal --host http://192.168.1.10:2375 events --since 2026-10-04T00:00:00Z
```

`stop --all` lists containers remotely and stops each one client-side (continues on error).

Limits:

- No interactive exec: `-i`/`-t` fail loudly (use local `cardinal exec` or the wings terminal).
- Remote `stats` polls `?stream=0` once per second (no server-side stream).
- Remote `inspect` returns Docker-schema JSON from `serve`, which differs from the local state dump; `--sensitive` is a no-op remotely.
- Remote `events --since` filters client-side from subscribe time.
- `logs --previous`/`--all` stay local-only.
- Remote `logs -f` survives log rotation (offset reset on fresh `start`) but ends if the container is removed.

---

## API authentication (cardinal serve)

`cardinal serve` exposes a Docker-compatible REST API: whoever can call it
can `exec` into and `create` arbitrary containers — that is root on the host.
So the API is gated by a Bearer token, **required on any bind since v2.3.13**
(loopback included; previously `127.0.0.1` worked without one, which meant
any local process — or any container, via the gateway — could drive it).

Generate a token once:

```bash
openssl rand -hex 32
```

Run with it (flag or env — flag wins if both are set):

```bash
cardinal serve -H 127.0.0.1 -p 2375 --token <токен>
CARDINAL_TOKEN=<токен> cardinal serve -H 127.0.0.1 -p 2375
```

Call the API with the token:

```bash
curl -s -H "Authorization: Bearer <токен>" http://127.0.0.1:2375/version
curl -s -H "Authorization: Bearer <токен>" http://127.0.0.1:2375/containers/json
```

Systemd unit (the token is root-equivalent — keep the file `0600`):

```ini
[Service]
ExecStart=/usr/local/bin/cardinal serve -H 127.0.0.1 -p 2375
Environment=CARDINAL_TOKEN=<токен>
```

```bash
sudo chmod 600 /etc/systemd/system/cardinal-serve.service
sudo systemctl daemon-reload && sudo systemctl restart cardinal-serve
```

Notes:

- External binds (`-H 0.0.0.0`) additionally want `--tls-cert`/`--tls-key`,
  otherwise the Bearer token travels in cleartext.
- `cardinal-wings` passes a random token itself when it spawns
  `cardinal serve` — nothing to configure there.
- Portainer / remote CLI: same `Authorization: Bearer <токен>` header
  (`cardinal --host http://IP:2375 --token <токен> ...`).

---

## Changelog

<!-- cardinal-release:start -->
**v2.3.14** — Fix: cobra wrapper больше не выдёргивает `--token`/`-H` из `cardinal serve` (флаг доходил пустым, serve с обязательным токеном отказывался стартовать — ломал спавн из wings и e2e).

**v2.3.13** — Security hardening: minimal container `/dev` (pruned to pseudo-devices, `MKNOD` out of the default capabilities), API refuses to serve without a token (loopback included), registry token-realm pinning, decompression-bomb caps on layer extraction, registry allowlist enforced on pull/push, egress DROP for `127/8` + `169.254/16`, backup-restore warning without checksum sidecar, blueprint repos https-only, cluster RPCs fail closed without token, no-echo password prompt in `login`.

**v2.3.12** — `cardinal attach`: ввод консоли пишется в лог (фикс errcheck в CI).

**v2.3.11** — `cardinal attach`: ввод консоли пишется в лог с префиксом `> ` — реконнекты возвращают команды рядом с ответами, а не одни ответы.

**v2.3.10** — `cardinal attach`: реплей хвоста лога начинается с границы строки (без обрубков, дедуп на панели срабатывает, команды рядом с ответами).

**v2.3.9** — `cardinal attach`: локальное эхо piped-строк stdin (фикс errcheck в CI).

**v2.3.8** — `cardinal attach` показывает набираемые строки при piped-вводе (stdin контейнера — pipe без эха; в терминале поведение не изменилось).

**v2.3.7** — Security hardening: serve unit with the API token is written `0600` (was world-readable), DNS/hostnames are validated before landing in `resolv.conf` and neighbours' `/etc/hosts`, port protocols are allowlisted before reaching `iptables`.

**v2.3.6** — `cardinal stats` redraws the table in place instead of scrolling (both local and remote modes).

**v2.3.5** — Stats fix: `disk_usage_bytes` now includes named volumes (servers kept in `volumes/` showed kilobytes on gigabyte data); full tree walks cached per container (60s TTL).

**v2.3.1** — Green build: test-harness fixes only (errcheck-clean stubs, race-free SSE/logs-follow tests). No runtime changes since v2.3.0.
<!-- cardinal-release:end -->

**v2.3.0** — Remote mode expansion: `exec` (captured output), `stats`, `top`, `inspect`, `events` (SSE), `start`/`stop`/`restart` and streaming `logs -f` via `cardinal --host URL [--token]`; host-port conflict warnings in `run`/`port add`.

**v2.2.0** — Read-only remote mode: `cardinal --host URL [--token]` queries a remote `cardinal serve` for `ps`, `info` and `logs --tail` (new `internal/client`, `CARDINAL_REMOTE_HOST`/`CARDINAL_TOKEN` env).

**v1.24.0** — Major security hardening: seccomp profile (blocks 30+ dangerous syscalls), AppArmor profile, device restrictions (/dev/shm, /dev/mqueue, /proc/sys, /sys read-only), network segmentation (`--isolated`), backup encryption (AES-256-GCM with `--encrypt`), audit logging for container lifecycle events, new CLI flags (`--seccomp-profile`, `--apparmor-profile`, `--isolated`, `--encrypted-backup`, `--audit-log`).

**v1.23.0** — Persistent restart supervisor with configurable delays and crash-loop protection (`--restart-max-attempts`/`--restart-window`, `restart_blocked`), per-container scheduled backups with checksum verification (`cardinal backup verify`), offline image verification (`cardinal verify`), reliable `cardinal update`, runtime hardening (zombie-exit detection, `cardinal rm` tombstones, safe OCI layer extraction, protected bind sources, `:ro`/`:rw` and tmpfs/NFS volume modes), instant startup for `--network none`/`host` containers, and the complete bilingual EN/RU documentation suite.

**v1.22.38** — Reliable `cardinal update` (5-minute download timeout, per-method errors), crash-loop protection with `--restart-max-attempts`/`--restart-window` and `restart_blocked` state, zombie-exit detection so exited detached containers are finalized and restarted on schedule, `cardinal rm` no longer races supervisor auto-restarts (tombstone marker), and instant startup for `--network none`/`host` containers (no 20s eth0 wait).

**v1.22.31** — OCI image extraction, protected bind-source validation, persistent restart policies with delays and systemd recovery, rotated cardinal logs, inspect JSON, manual and scheduled safe container backups with retention, cluster orchestration, FaaS, blueprints, services, Compose, healthchecks, startup scripts, dynamic ports, events, stats, Docker-compatible REST API, cross-architecture CI builds, read-only doctor/security diagnostics, optional HTTPS API, and complete EN/RU CLI references with practical examples.

**v1.20.0** — Dynamic port management (`cardinal port add/rm`). Russian (ru) docs.

**v1.15.0** — `pivot_root` security fix. `cardinal stop --all`. `cardinal exec -i/-t` flags. Disk limit fix.

**v1.14.0** — Disk limit support (`--disk`). Multi-arch image resolution.

**v1.13.0** — `--startup` flag, `--healthcheck-*` flags, CARDINAL_* env vars, cgroups v2 resource limits.

**v1.11.0** — Debian packaging, APT repository, release workflow.

**v1.10.0** — `cardinal stats` command (live CPU/RAM/IO/PIDs from cgroup v2).

**v1.4.7** — `cardinal attach` rewritten (Unix socket, history + live, Ctrl+C safe).

**v1.3.0** — `cardinal.toml` config, `cardinal up`/`cardinal down`.

**v1.1.0** — First stable release.

---

## Updating

```bash
cardinal update
```

Downloads the latest binary and replaces `/usr/local/bin/cardinal`. The download allows up to five minutes; if it fails, each method (Go client, curl, wget) reports its own error. For manual installation, see Section 2 of the [Running Guide](docs/en/running.md).

---

## Documentation

- [Contributors and contribution guide](CONTRIBUTING.md)
- [Security model and vulnerability reporting](SECURITY.md)
- [Build, CI, and versioning](docs/build.md) — local checks, cross-compilation, and release automation

### Installation Guides

| English | Русский |
|---|---|
| [Linux (Universal)](docs/en/install-linux.md) | [Linux (Универсальная)](docs/ru/install-linux.md) |
| [Debian / Ubuntu](docs/en/install-debian.md) | [Debian / Ubuntu](docs/ru/install-debian.md) |
| [Fedora / RHEL](docs/en/install-fedora.md) | [Fedora / RHEL](docs/ru/install-fedora.md) |
| [Arch Linux](docs/en/install-arch.md) | [Arch Linux](docs/ru/install-arch.md) |
| [Alpine Linux](docs/en/install-alpine.md) | [Alpine Linux](docs/ru/install-alpine.md) |
| [NixOS](docs/en/install-nixos.md) | [NixOS](docs/ru/install-nixos.md) |
| [Gentoo / Funtoo](docs/en/install-gentoo.md) | [Gentoo / Funtoo](docs/ru/install-gentoo.md) |
| [Void Linux](docs/en/install-void.md) | [Void Linux](docs/ru/install-void.md) |
| [Snap](docs/en/install-snap.md) | [Snap](docs/ru/install-snap.md) |
| [AppImage](docs/en/install-appimage.md) | [AppImage](docs/ru/install-appimage.md) |
| [Manual Binary](docs/en/install-manual.md) | [Ручная установка](docs/ru/install-manual.md) |

### Guides

| English | Русский |
|---|---|
| [Running Guide](docs/en/running.md) | [Руководство по запуску](docs/ru/running.md) |
| [Complete CLI Reference](docs/en/commands.md) | [Полный справочник CLI](docs/ru/commands.md) |
| [Command Examples](docs/en/examples.md) | [Примеры команд](docs/ru/examples.md) |
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
cardinal bootstrap --remove
rm /usr/local/bin/cardinal
rm -rf ~/.cardinal
```

## Contributing

The project maintainer and verified contributors are listed in
[`CONTRIBUTING.md`](CONTRIBUTING.md). GitHub's contributor graph is generated
from commit authors, so contributors should use their own GitHub identity when
opening pull requests or submitting commits.

- [animesao](https://github.com/animesao) — maintainer and primary author.
- [MASHINISTPROJECT](https://github.com/MASHINISTPROJECT) — installer, image pull progress bar, uninstall script fixes.
- `github-actions[bot]` — automated release and versioning workflow.
- `dependabot[bot]` — automated dependency updates.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution rules and the process
for adding new contributors with their permission.

## License

This project is released under the Apache License, Version 2.0.
The full license text is in [LICENSE](LICENSE).

* License: Apache-2.0 — [LICENSE](LICENSE)
* Attribution: [NOTICE](NOTICE)
* Brand policy: [TRADEMARKS.md](TRADEMARKS.md)
* Commercial model: [COMMERCIAL.md](COMMERCIAL.md)

cardinal-wings is a separate project with its own license
(AGPL-3.0-or-later) — see
[animesao/cardinal-wings](https://github.com/animesao/cardinal-wings).

Contributor and maintainer attribution: [CONTRIBUTING.md](CONTRIBUTING.md).
