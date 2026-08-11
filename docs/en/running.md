# Running dck Containers

This guide covers the complete day-to-day workflow: install dck, pull an image, run an application, mount persistent files, configure environment variables, inspect logs, update code, and recover from common errors.

> dck runs containers on Linux. Commands below use Bash and should be run as `root` or with the required privileges.

## 1. Requirements

A supported Linux host should provide:

- `unshare`, `nsenter`, `mount`, `ip`, `iptables`, and `pgrep`;
- PID, mount, UTS, IPC, and network namespaces;
- OverlayFS;
- cgroups v2 for resource limits;
- `curl` for installation and registry operations.

Check the host before installing:

```bash
command -v unshare nsenter mount ip iptables pgrep curl
grep overlay /proc/filesystems
uname -a
```

## 2. Install or update dck

Debian/Ubuntu APT installation:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/scripts/install-apt.sh | sudo bash
```

Generic Linux installation:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

Verify the installation:

```bash
dck version
dck info
```

Update an existing installation:

```bash
dck update --check
dck update
```

After updating, verify the binary and run a disposable container:

```bash
dck version
dck run --rm alpine:latest echo "DCK UPDATE OK"
```

## 3. Pull and run an image

The image reference is positional. `--image` is also supported, but `--images` is not.

```bash
dck pull alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

Use a tag with a colon. These references have different meanings:

```text
python3.12                         repository library/python3.12
python:3.12                        image python, tag 3.12
nanozoo/python3.12:3.12--d46ab4d  repository and explicit tag
```

If a repository has no `latest` tag, specify a tag printed by search:

```bash
dck search nanozoo/python3.12
dck pull nanozoo/python3.12:3.12--d46ab4d
```

## 4. Run a long-lived service

The general form is:

`--restart-delay` accepts Go duration values such as `10s`, `30s`, or `1m`.

```bash
dck run -d \
  -n APP_NAME \
  -p HOST_PORT:CONTAINER_PORT \
  --restart unless-stopped \
  --restart-delay 1m \
  IMAGE[:TAG] \
  COMMAND [ARGUMENTS...]
```

Example web server:

```bash
dck pull nginx:alpine
dck run -d \
  -n web \
  -p 8080:80 \
  --restart unless-stopped \
  nginx:alpine

curl http://127.0.0.1:8080
dck ps -a
dck logs web
```

Supported restart policies are `no`, `always`, `on-failure`, and `unless-stopped`. Add `--restart-delay 1m` (or a shorter value such as `10s`) to control how long dck waits after an unexpected process exit before starting the container again. The delay does not override an intentional `dck stop`.

## 5. Bind mounts and named volumes

A volume specification is `source:container_target`. The target inside the container must be an absolute path:

```bash
--vol /data/myapp:/app
--vol myapp_data:/var/lib/myapp
```

Create a named volume with dck:

```bash
dck volume create app-data
dck volume ls
dck volume inspect app-data
```

For application source code, use a dedicated data directory outside protected host paths:

```bash
mkdir -p /data/myapp
cp -a /path/to/myapp/. /data/myapp/
dck run -d \
  -n myapp \
  --vol /data/myapp:/app \
  --workdir /app \
  --restart unless-stopped \
  IMAGE[:TAG] COMMAND
```

For security, dck rejects bind sources that resolve to sensitive host paths such as `/`, `/root`, `/etc`, `/proc`, `/sys`, and similar system directories. This prevents an accidental container mount from exposing host secrets. A path such as `/root/myapp` may therefore need to be moved to `/data/myapp` or another dedicated directory.

> The source directory must exist before `dck run`. The command `--vol "$PWD:/app"` is valid only when the current directory is an allowed host path.

## 6. Environment variables and `.env`

Pass individual variables with `-e`:

```bash
dck run -d -e APP_ENV=production -e PORT=8080 IMAGE[:TAG] COMMAND
```

Or use a file containing `KEY=VALUE` entries:

```bash
cat > .env <<'EOF'
APP_ENV=production
BOT_TOKEN=replace_me
EOF

chmod 600 .env
dck run -d --env-file .env IMAGE[:TAG] COMMAND
```

Do not commit `.env` files or print secrets in public logs. For a project at `/data/mybot`, use an explicit path:

```bash
dck run -d \
  -n mybot \
  --env-file /data/mybot/.env \
  --vol /data/mybot:/bot \
  --workdir /bot \
  IMAGE[:TAG] COMMAND
```

## 7. Python bot: complete example

Assume the project contains `main.py`, `requirements.txt`, and `.env` in `/data/alfheimguide`:

```bash
cd /data/alfheimguide
cp .env.example .env
chmod 600 .env
# Edit .env and set the required token/configuration.
```

Start it in the background:

```bash
dck pull python:3.12
dck rm -f alfheimguide 2>/dev/null || true

dck run -d \
  -n alfheimguide \
  --restart unless-stopped \
  --env-file "$PWD/.env" \
  --vol "$PWD:/bot" \
  --workdir /bot \
  python:3.12 \
  sh -c 'python -m pip install --no-cache-dir -r requirements.txt && exec python main.py'
```

Check the result:

```bash
dck ps -a
dck logs --tail 100 alfheimguide
dck logs -f alfheimguide
```

If dependencies are installed on every restart and that is undesirable, install them once into the container overlay and run only the application afterward, or build a dedicated image with a Dockerfile. The bind-mounted project files remain on the host and survive container removal.

## 8. Java or Minecraft server

For a custom Java JAR in `/data/minecraft`, make sure the JAR and all server data are in that directory:

```bash
mkdir -p /data/minecraft
ls -lh /data/minecraft/server.jar
```

Run Eclipse Temurin Java 21:

```bash
dck pull eclipse-temurin:21
dck rm -f minecraft 2>/dev/null || true

dck run -d \
  -n minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --vol /data/minecraft:/test \
  --workdir /test \
  eclipse-temurin:21 \
  java -jar server.jar nogui
```

The server must listen on `0.0.0.0:25565`, not only on `127.0.0.1`.

Check it:

```bash
dck ps -a
dck logs --tail 100 minecraft
ss -ltnp | grep 25565
```

If using a named volume instead:

```bash
dck volume create minecraft-data
dck run -d \
  -n minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --vol minecraft-data:/data \
  --workdir /data \
  eclipse-temurin:21 \
  java -jar server.jar nogui
```

## 9. Container lifecycle

```bash
dck ps                 # running containers
dck ps -a              # running and stopped containers
dck stop NAME          # stop without deleting data
dck start NAME         # start an existing stopped container
dck restart NAME       # stop and start
dck rm NAME            # remove a stopped container
dck rm -f NAME         # force removal
```

`dck stop` preserves the container overlay and mounted application data. `dck rm -f` removes the container overlay and dck-managed log/state files. Host bind-mounted files and named volume data are not removed by removing the container.

## 10. Logs and attach

For a root installation, dck stores the container stdout/stderr log at:

```text
/root/.dck/logs/<container-id>.log
```

The data root can be changed with `DCK_DATA_DIR`:

```bash
export DCK_DATA_DIR=/data/dck-state
dck info
```

Use the CLI instead of editing internal files:

```bash
dck logs NAME
dck logs --tail 100 NAME
dck logs -f NAME
dck attach NAME
```

`dck logs -f` follows new output. `dck attach` connects to the main process of a detached container. Press `Ctrl+C` safely to leave attach; it does not stop the container.

Starting a container creates a fresh dck stdout/stderr log, so previous dck log output is not accumulated across `stop`/`start` or `restart`. Application-specific logs are different: for example, Minecraft's `/data/logs/latest.log` is stored in the bind mount or named volume and is preserved by the application.

## 11. Inspect and operate inside a container

```bash
dck exec NAME command args...
dck exec -i -t NAME /bin/sh
dck console NAME
dck top NAME
dck port NAME
dck stats NAME --no-stream
dck fs ls NAME /path
dck fs cat NAME /path/file
dck cp ./local-file NAME:/path/
dck cp NAME:/path/file ./local-file
```

`attach` connects to the existing main process; `exec` starts a new process. Use `console` or `exec -i -t` for a shell.

## 12. Resource limits and security

```bash
dck run -d --memory 512m --cpus 1 --disk 5G IMAGE[:TAG] COMMAND
dck run -d --user 1000:1000 --cap-drop ALL --no-new-privs IMAGE[:TAG] COMMAND
dck run -d --readonly IMAGE[:TAG] COMMAND
```

Add only the capabilities that the application requires:

```bash
--cap-add NET_ADMIN
```

Use `--network none` for a workload that does not need networking, or `--network host` only when sharing the host network is intentional.

## 13. Update application code

With a bind mount, edit the host files and restart the container:

```bash
nano /data/alfheimguide/main.py
dck restart alfheimguide
dck logs --tail 100 alfheimguide
```

For a new dependency, update `requirements.txt` and restart if your startup command installs dependencies. For production, prefer a built image instead of installing packages on every boot.

## 14. Application recipes: bots, databases, and game servers

All examples use the same command shape:

```bash
dck run -d \
  -n NAME \
  -p HOST_PORT:CONTAINER_PORT \
  --restart POLICY \
  --restart-delay 1m \
  --env-file "$PWD/.env" \
  --vol "$PWD:/app" \
  --workdir /app \
  IMAGE[:TAG] COMMAND
```

Put all dck flags before the image name. Anything after the image and command is passed to the container process. In particular, write `-p 23323:23332` before `python:3.12`, not after `sh -c`.

### Restart behavior

| Policy | Process exits unexpectedly | `dck stop` | Host reboot |
|---|---|---|---|
| `no` | Stay stopped | Stay stopped | Stay stopped |
| `on-failure` | Restart only for a non-zero exit | Stay stopped | Not bootstrapped |
| `always` | Restart | Stay stopped when stopped explicitly | Start automatically |
| `unless-stopped` | Restart | Stay stopped until an explicit `dck start` | Start automatically unless it was manually stopped |

`dck run --restart always` and `dck run --restart unless-stopped` install the systemd bootstrap service when run as root. `--restart-delay` affects automatic recovery after a process exit; it does not delay the initial boot. To install bootstrap manually:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

The bootstrap service starts eligible containers after the host boots. It does not make `on-failure` a boot-autostart policy.

### Python bot with `.env`, mount, port, and automatic recovery

```bash
mkdir -p /data/bot
cd /data/bot
cp .env.example .env
chmod 600 .env
# Edit .env and set BOT_TOKEN and other secrets.

# Automatically restart after a crash and start after reboot.
dck pull python:3.12
dck run -d \
  -n bot \
  -p 23323:23332 \
  --restart unless-stopped \
  --restart-delay 1m \
  --env-file "$PWD/.env" \
  --vol "$PWD:/bot" \
  --workdir /bot \
  python:3.12 \
  sh -c "python -m pip install --no-cache-dir -r requirements.txt && exec python main.py"
```

For a bot that should stop permanently when its process exits, omit `--restart`:

```bash
dck run -d \
  -n bot-manual \
  --env-file "$PWD/.env" \
  --vol "$PWD:/bot" \
  --workdir /bot \
  python:3.12 \
  sh -c "python -m pip install --no-cache-dir -r requirements.txt && exec python main.py"
```

### PostgreSQL

Persistent database data belongs in a named volume, not in the container overlay:

```bash
dck volume create postgres-data
dck run -d \
  -n postgres \
  -p 5432:5432 \
  --restart unless-stopped \
  --vol postgres-data:/var/lib/postgresql/data \
  -e POSTGRES_DB=app \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD=change_me \
  postgres:16
```

Manual-only variant (no restart or boot autostart):

```bash
dck run -d \
  -n postgres-manual \
  -p 5433:5432 \
  --vol postgres-data:/var/lib/postgresql/data \
  -e POSTGRES_DB=app \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD=change_me \
  postgres:16
```

### MySQL

```bash
dck volume create mysql-data
dck run -d \
  -n mysql \
  -p 3306:3306 \
  --restart always \
  --vol mysql-data:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=change_me \
  -e MYSQL_DATABASE=app \
  -e MYSQL_USER=app \
  -e MYSQL_PASSWORD=change_me \
  mysql:8
```

### Redis

```bash
dck volume create redis-data
dck run -d \
  -n redis \
  -p 6379:6379 \
  --restart unless-stopped \
  --vol redis-data:/data \
  redis:7 \
  redis-server --appendonly yes
```

### MongoDB

```bash
dck volume create mongo-data
dck run -d \
  -n mongodb \
  -p 27017:27017 \
  --restart unless-stopped \
  --vol mongo-data:/data/db \
  mongo:8
```

### Minecraft Java server

A custom JAR can be kept in a host bind mount:

```bash
mkdir -p /data/minecraft
# Copy server.jar, eula.txt, server.properties, and worlds into /data/minecraft.

dck run -d \
  -n minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --restart-delay 1m \
  --vol /data/minecraft:/data \
  --workdir /data \
  eclipse-temurin:21 \
  java -Xms1G -Xmx4G -jar server.jar nogui
```

Manual-only Minecraft:

```bash
dck run -d \
  -n minecraft-manual \
  -p 25566:25565 \
  --vol /data/minecraft:/data \
  --workdir /data \
  eclipse-temurin:21 \
  java -Xms1G -Xmx4G -jar server.jar nogui
```

The Minecraft server must listen on `0.0.0.0:25565`. Its own logs and worlds are preserved in `/data/minecraft`; dck stdout/stderr logs are reset at each new container start. Add `--restart-delay 1m` when recovery should wait one minute after a crash.

### Terraria

Image configuration differs between Terraria images. Verify the image's documented environment variables before production use:

```bash
dck volume create terraria-data
dck run -d \
  -n terraria \
  -p 7777:7777 \
  --restart unless-stopped \
  --vol terraria-data:/config \
  terraria-server-image:latest
```

Without automatic restart:

```bash
dck run -d \
  -n terraria-manual \
  -p 7778:7777 \
  --vol terraria-data:/config \
  terraria-server-image:latest
```

Replace `terraria-server-image:latest` with the image you selected and follow its required EULA/configuration variables.

### Factorio

```bash
dck volume create factorio-data
dck run -d \
  -n factorio \
  -p 34197:34197/udp \
  --restart unless-stopped \
  --vol factorio-data:/factorio \
  factoriotools/factorio:stable
```

### Source-engine or other dedicated game server

Use the image's documented internal port and persistent data directory. The dck policy is independent of the game:

```bash
dck volume create game-data
dck run -d \
  -n dedicated-game \
  -p 27015:27015/udp \
  --restart always \
  --vol game-data:/server \
  game-server-image:latest \
  ./start-server.sh
```

For a one-time/manual server, remove `--restart always`. Use `--restart-delay 1m` when you want a one-minute wait after a crash before recovery. Always check the image documentation for EULA acceptance, ports, startup command, save location, and required environment variables.

### Inspect, stop, and recover

```bash
dck ps -a
dck logs --tail 100 minecraft
dck stats minecraft --no-stream
dck stop minecraft
dck start minecraft
dck restart minecraft
```

A process crash triggers the configured restart policy. A manual `dck stop` is intentional and prevents `unless-stopped` from starting again until `dck start` is run. To disable automatic recovery permanently, update the container:

```bash
dck stop bot
dck set bot --restart no
dck start bot
```

## 15. Troubleshooting

### `flag provided but not defined: -images`

Use `--image`, or pass the image positionally:

```bash
dck run --image python:3.12 python --version
dck run python:3.12 python --version
```

### `container mount target must be absolute`

The container target must start with `/`:

```bash
--vol /data/app:/app
```

Not `--vol /data/app:app`.

### `resolve bind source ... no such file or directory`

Create the host source first:

```bash
mkdir -p /data/app
```

### `bind source ... is a protected host path`

Move the project outside protected system directories, for example:

```bash
mkdir -p /data/app
cp -a /root/app/. /data/app/
```

### `Usage: dck run` followed by `-n: command not found`

A Bash continuation backslash must be the final character on its line. Do not insert blank lines after `\`. Use one line if copying through a fragile terminal:

```bash
dck run -d -n app --vol /data/app:/app --workdir /app IMAGE[:TAG] COMMAND
```

### Container is `created` but not `running`

Inspect the logs and remove the failed container before retrying:

```bash
dck ps -a
dck logs NAME
dck rm -f NAME
```

### The application is running but unreachable

Check that it listens on `0.0.0.0`, the port mapping is correct, and the host firewall allows the port:

```bash
dck port NAME
ss -ltnp
dck logs NAME
```

## 16. Data locations

For root, the default dck data directory is `/root/.dck`:

```text
/root/.dck/
├── images/       downloaded image root filesystems
├── containers/   container state JSON
├── overlay/      writable container layers
├── logs/         dck stdout/stderr logs
├── volumes/      named volumes
├── cache/        cached image layers
└── consoles/     attach sockets
```

Set `DCK_DATA_DIR` before running dck to use another state location. Application bind mounts such as `/data/alfheimguide` are separate from this internal state.
