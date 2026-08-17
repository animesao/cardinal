<!-- dck-version:start -->
**Documentation version:** `1.25.0`
**Project release:** `v1.25.0`
<!-- dck-version:end -->

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

### Universal installer (all distros)

The install script auto-detects your distro and installs dck + dependencies:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

Supported distros: **Ubuntu, Debian, Arch, Manjaro, Fedora, RHEL, CentOS, Rocky, Alma, openSUSE, Alpine, Void Linux**, and more.

### Distribution-specific methods

**Arch Linux / Manjaro (AUR):**

```bash
# Using an AUR helper (yay/paru)
yay -S dck
# Or from source
git clone https://aur.archlinux.org/dck.git
cd dck
makepkg -si
```

**Fedora / RHEL / CentOS:**

Download and install the latest RPM asset from the [GitHub release page](https://github.com/animesao/dck/releases/latest):

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Could not determine the latest release" >&2; exit 1; }
VERSION="${TAG#v}"
FILE="dck-${VERSION}-linux-amd64.rpm"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo dnf install "./$FILE"
# On older systems:
# sudo rpm -Uvh "./$FILE"
```

**Debian / Ubuntu (.deb):**

Download and install the latest DEB asset from the [GitHub release page](https://github.com/animesao/dck/releases/latest):

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Could not determine the latest release" >&2; exit 1; }
VERSION="${TAG#v}"
FILE="dck-${VERSION}-linux-amd64.deb"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo apt install "./$FILE"
```

**Snap package (from GitHub Releases):**

Snap packages are built for every release and attached to GitHub Releases; they
are not uploaded automatically to the Snap Store. Download and install the
versioned `.snap` asset directly:

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"\]*\)".*/\1/p')"
test -n "$TAG" || { echo "Could not determine the latest release" >&2; exit 1; }
FILE="dck-${TAG#v}-linux-amd64.snap"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo snap install --dangerous --classic "$FILE"
```

Use the `arm64` suffix on ARM64 hosts. The Snap uses classic confinement
because dck needs host namespace, mount, cgroup, and networking capabilities.

**Manual binary install:**

```bash
curl -fsSL https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64 -o /tmp/dck-new
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
dck bootstrap --install
```

**AppImage (amd64 and arm64):**

AppImage is a self-contained executable format. Download the matching asset
from the [latest GitHub release](https://github.com/animesao/dck/releases/latest),
make it executable, and run it directly:

```bash
# x86_64 / amd64: resolve the current release asset automatically
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Could not determine the latest release" >&2; exit 1; }
FILE="dck-${TAG#v}-linux-amd64.AppImage"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
chmod +x "$FILE"
"./$FILE" version

# Install the embedded binary and enable the supervisor
"./$FILE" --install
```

For ARM64, use the matching `dck-*-linux-arm64.AppImage` asset instead. AppImage does
not require a package manager, but dck still needs the host's Linux namespaces,
cgroups, OverlayFS, mount, and networking capabilities. The release does not
publish a standard AppImage for ARMv6 because the official AppImage runtime
supports x86_64, aarch64, and armhf, not true ARMv6. ARMv6 users should use
the `dck-linux-armv6` binary or `.tar.gz` archive.

#### Install by double-clicking on a Linux desktop

The dck AppImage is a CLI runtime, not a graphical application. When you
double-click the AppImage in a Linux file manager, it opens an available
terminal and runs the desktop installer. The installer extracts the embedded
static dck binary to `/usr/local/bin/dck`, asks for administrator permission
when needed, and installs/starts `dck-bootstrap.service` when systemd is
available. Your containers and images remain in the existing dck data
directory, and the original AppImage remains portable.

You can also start the same installer from a terminal. Use the filename of the AppImage you downloaded:

```bash
APPIMAGE="$(find . -maxdepth 1 -type f -name 'dck-*-linux-amd64.AppImage' -print -quit)"
test -n "$APPIMAGE" || { echo "AppImage not found in the current directory" >&2; exit 1; }
chmod +x "$APPIMAGE"
"$APPIMAGE" --install
```

To use the AppImage only as a portable CLI, pass a normal dck command instead. This block locates the downloaded file in the current directory:

```bash
APPIMAGE="$(find . -maxdepth 1 -type f -name 'dck-*-linux-amd64.AppImage' -print -quit)"
test -n "$APPIMAGE" || { echo "AppImage not found in the current directory" >&2; exit 1; }
chmod +x "$APPIMAGE"
"$APPIMAGE" version
"$APPIMAGE" run --rm --network none alpine:latest echo OK
```

If double-clicking does nothing, make the file executable and choose **Run**
rather than **Display** in the file manager. A desktop environment must have a
terminal emulator installed (`x-terminal-emulator`, GNOME Terminal, Konsole,
XFCE Terminal, or MATE Terminal). On a headless VPS, use the terminal commands
above instead. If an older AppImage reports `cannot stat ... Permission denied`
from `sudo install`, download the latest AppImage again: older builds tried to
read the FUSE-mounted binary as root instead of copying it first to `/tmp`.

### Verify the installation

```bash
dck version
dck info
```

Update an existing installation:

```bash
dck update --check
dck update
```

When dck is running from an AppImage, `dck update` cannot modify the
read-only AppImage mount. The updater now installs the verified new static
binary to `/usr/local/bin/dck` instead and leaves the original AppImage
unchanged. If the desktop user cannot write there, it requests `sudo`.

After updating, verify the binary and run a disposable container:

```bash
dck version
dck run --rm alpine:latest echo "DCK UPDATE OK"
```

If `dck update` cannot download the binary (older releases failed with `Failed to download binary: all methods failed`), install the release manually. Replace the version and architecture as needed:

```bash
curl -fsSL --connect-timeout 10 -o /tmp/dck-new \
  https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
sudo systemctl restart dck-bootstrap   # if the systemd supervisor is installed
```

Binary names are `dck-linux-amd64`, `dck-linux-arm64`, and `dck-linux-armv6`. Releases also provide native packages for each supported architecture: `.deb`, `.rpm`, `.pkg.tar.zst`, and `.apk` with `amd64`, `arm64`, or `armv6` in the filename. AppImage assets are published for `amd64` and `arm64`; ARMv6 uses the raw binary or `.tar.gz` archive. Choose the package matching both your distribution and CPU architecture. If GitHub is unavailable, download the asset from another trusted network or transfer it over SSH; do not use unverified third-party mirrors for release binaries. The `${VERSION}` placeholder means the tag without its leading `v` (for example, `1.23.17`).

## 3. Pull and run an image

The image reference is positional. `--image` is also supported, but `--images` is not.

```bash
dck pull alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

For a pull → verify → run workflow, check the image's integrity after pulling and before running. `dck verify` compares the config digest and every layer digest against the stored manifest locally, without contacting the registry:

```bash
dck pull alpine:latest
dck verify alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

`dck verify` exits non-zero if the image is not present locally, the config digest does not match the stored metadata or manifest, or any layer file is corrupt. It is a fast offline sanity check for images restored from `dck import` or transferred between hosts.

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

Quick repeated crashes are protected: once the crash-loop budget (default 5 restarts, tunable with `--restart-max-attempts` and `--restart-window`) is exhausted, automatic restart is blocked — `dck inspect NAME` then shows `"restart_blocked": true` — and the container stays stopped until an explicit `dck start`.

## 5. Bind mounts and named volumes

A volume specification is `source:container_target`. The target inside the container must be an absolute path:

```bash
--vol /data/myapp:/app
--vol myapp_data:/var/lib/myapp
```

Append `:ro` or `:rw` to make the mount read-only or read-write (read-write is the default), or a propagation mode such as `:shared`/`:rshared`:

```bash
--vol /data/myapp:/app:ro
--vol /data/config:/etc/app:shared
```

`tmpfs:` (in-memory) and `nfs://server:/export:/container/path` specs are also supported.

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

## 11. Automatic backups

Enable a per-container schedule. The backup contains the writable overlay and named volumes, but not host bind mounts. Back up bind-mounted directories such as `/data/minecraft` separately. dck briefly stops a running container so the archive is consistent before starting it again. Enabling the schedule does not create an archive immediately; the first archive is created after the configured interval.

```bash
dck backup enable minecraft --interval 6h --retention 14
dck backup status minecraft
dck backup list
dck backup disable minecraft
```

By default archives are written to `$DCK_DATA_DIR/backups/<container>/`. Set a dedicated directory if required:

```bash
dck backup enable minecraft \
  --interval 24h \
  --retention 7 \
  --dir /data/backups/minecraft
```

Install the persistent supervisor once so the schedule continues after the terminal and after a reboot. If a backup fails, the supervisor records a retry time instead of retrying in a tight loop:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

A manual backup can still be created with `dck backup create NAME`; stop the container first. Restore only into a stopped container. Manual and scheduled archives cover dck-managed overlay data and named volumes, not host bind mounts:

```bash
dck backup restore minecraft /data/backups/minecraft/minecraft-20260811-120000.tar.gz
```

Automatic backup settings are stored in the container state and survive `stop`, `start`, and dck upgrades. Retention removes the oldest scheduled archives after a successful backup; it does not delete manually placed files outside the container's scheduled archive naming pattern.

Verify an archive against its checksum with `dck backup verify FILE.tar.gz`. When no checksum sidecar exists, dck reports the archive as valid but unverified.

## 12. Inspect and operate inside a container

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

## 13. Resource limits and security

```bash
dck run -d --memory 512m --cpus 1 --disk 5G IMAGE[:TAG] COMMAND
dck run -d --user 1000:1000 --cap-drop ALL --no-new-privs IMAGE[:TAG] COMMAND
dck run -d --readonly IMAGE[:TAG] COMMAND
```

Add only the capabilities that the application requires:

```bash
--cap-add NET_ADMIN
```

Use `--network none` for a workload that does not need networking, or `--network host` only when sharing the host network is intentional. Containers with `--network none` or `--network host` start without any interface wait; bridge-mode containers wait at most five seconds for the veth interface to come up.

## 14. Update application code

With a bind mount, edit the host files and restart the container:

```bash
nano /data/alfheimguide/main.py
dck restart alfheimguide
dck logs --tail 100 alfheimguide
```

For a new dependency, update `requirements.txt` and restart if your startup command installs dependencies. For production, prefer a built image instead of installing packages on every boot.

## 15. Application recipes: bots, databases, and game servers

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
| `on-failure` | Restart only for a non-zero exit while its monitor is alive; not adopted after detached CLI exits | Stay stopped | Not bootstrapped |
| `always` | Restart | Stay stopped when stopped explicitly | Start automatically |
| `unless-stopped` | Restart | Stay stopped until an explicit `dck start` | Start automatically unless it was manually stopped |

`dck run --restart always` and `dck run --restart unless-stopped` install the systemd bootstrap service when run as root. `--restart-delay` affects automatic recovery after a process exit; it does not delay the initial boot. To install bootstrap manually:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

The bootstrap service starts eligible containers after the host boots. It does not make `on-failure` a boot-autostart policy. For a persistent detached service, use `always` or `unless-stopped`; `on-failure` is only reliable while the process that owns its monitor remains alive.

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

## 16. Troubleshooting

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

### `Failed to download binary: all methods failed` during `dck update`

The updater used to time out after ten seconds, which was too short for multi-megabyte binaries on slow links. Install the release manually:

```bash
curl -fsSL --connect-timeout 10 -o /tmp/dck-new \
  https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
sudo systemctl restart dck-bootstrap
```

### Container stays `running` after its process exited

Older releases treated defunct (zombie) container processes as alive, so the supervisor never noticed the exit — the container stayed `running`, crash-loop restarts barely fired, and resources were not cleaned up. Update dck and verify:

```bash
dck version
dck ps -a
dck inspect NAME | grep -E '"status"|"pid"'
```

### A `--network none` container hangs before its command starts

For `--network none`, no `eth0` address is expected, so dck skips the interface wait. For `--network host`, the host interface is already available; bridge mode waits only briefly for the veth interface. If startup still hangs, update dck and inspect the container logs.

## 17. Data locations

For root, the default dck data directory is `/root/.dck`. The exact location is also shown by `dck info`:

```text
/root/.dck/
├── images/       downloaded image root filesystems
├── containers/   container state JSON
├── overlay/      writable container layers
├── logs/         dck stdout/stderr logs
├── volumes/      named volumes
├── cache/        cached image layers
├── consoles/     attach sockets
└── backups/      scheduled container archives
```

Set `DCK_DATA_DIR` before running dck to use another state location. Application bind mounts such as `/data/alfheimguide` are separate from this internal state.
