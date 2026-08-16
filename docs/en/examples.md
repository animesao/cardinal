<!-- dck-version:start -->
**Documentation version:** `1.24.16`
**Project release:** `v1.24.16`
<!-- dck-version:end -->

# dck Command Examples

Practical recipes for Linux hosts. Replace image names, passwords, paths, and public ports with your own values.

> dck intentionally rejects protected host bind sources such as `/root`, `/etc`, `/var`, `/usr`, `/opt`, and `/run`. Use `/data/<project>` or a named volume. Create host directories before using them.

## 1. First smoke test

```bash
dck version
dck pull alpine:latest
dck run --rm alpine:latest echo "DCK OK"
dck ps -a
```

Expected output from the one-shot test includes:

```text
DCK OK
```

## 2. Long-running test container

```bash
dck run -d \
  -n dck-test \
  --restart unless-stopped \
  --restart-delay 10s \
  alpine:latest \
  sh -c 'i=0; while true; do i=$((i+1)); echo "tick $i $(date)"; sleep 5; done'

dck ps
dck logs --tail 20 dck-test
dck logs -f dck-test
```

Press `Ctrl+C` to stop following logs; it does not stop the container.

```bash
dck stop dck-test
dck ps -a
dck rm dck-test
```

## 3. Restart after a crash

This process exits every three seconds. dck should start it again after the configured delay.

```bash
dck run -d \
  -n restart-test \
  --restart always \
  --restart-delay 10s \
  alpine:latest \
  sh -c 'echo process-started; sleep 3; exit 1'

sleep 15
dck ps -a
dck logs --all restart-test
```

Remove the test when finished:

```bash
dck rm -f restart-test
```

For a container that should stay stopped after an intentional `dck stop`, use `unless-stopped` instead of `always`.

## 4. Environment variables and `.env`

```bash
mkdir -p /data/env-test
cat > /data/env-test/.env <<'EOF'
APP_ENV=production
APP_NAME=dck-demo
EOF

dck run --rm \
  --env-file /data/env-test/.env \
  alpine:latest \
  sh -c 'printf "%s/%s\\n" "$APP_NAME" "$APP_ENV"'
```

`-e KEY=VALUE` can be repeated and is useful for a small number of values. Do not put registry or application secrets directly into shell history; use a protected env file or a secret-management workflow.

## 5. Named volumes

Named volumes are stored by dck and survive container removal.

```bash
dck volume create demo-data
dck run --rm --vol demo-data:/data alpine:latest \
  sh -c 'echo saved > /data/message.txt'
dck run --rm --vol demo-data:/data alpine:latest \
  cat /data/message.txt
dck volume inspect demo-data
dck volume rm demo-data
```

A bind mount shares an existing host directory instead:

```bash
mkdir -p /data/demo-app
dck run --rm -v /data/demo-app:/app alpine:latest \
  sh -c 'echo host-visible > /app/message.txt'
cat /data/demo-app/message.txt
```

## 6. Web server

```bash
mkdir -p /data/site
printf '<h1>Hello from dck</h1>\n' > /data/site/index.html

dck run -d \
  -n web \
  -p 8080:80 \
  --restart unless-stopped \
  --vol /data/site:/usr/share/nginx/html \
  nginx:alpine

curl http://127.0.0.1:8080
dck port web
dck logs --tail 50 web
```

The host source is `/data/site`; `/usr/share/nginx/html` is the path inside the nginx container.

## 7. Python application or bot

```bash
mkdir -p /data/python-app
cd /data/python-app
cat > .env <<'EOF'
APP_ENV=production
EOF
cat > requirements.txt <<'EOF'
flask==3.0.0
EOF
cat > app.py <<'EOF'
from flask import Flask
app = Flask(__name__)

@app.get('/')
def index():
    return 'Hello from dck\n'

app.run(host='0.0.0.0', port=5000)
EOF

dck run -d \
  -n flask \
  -p 5000:5000 \
  --restart unless-stopped \
  --env-file "$PWD/.env" \
  --vol "$PWD:/app" \
  --workdir /app \
  python:3.12 \
  sh -c 'python -m pip install --no-cache-dir -r requirements.txt && exec python app.py'

curl http://127.0.0.1:5000
dck logs -f flask
```

For a Discord or Telegram bot, replace `app.py` and `requirements.txt`, pass its token through `--env-file`, and keep the same `--restart unless-stopped` policy.

## 8. PostgreSQL, MySQL, and Redis

Use named volumes for database data. These examples assume the images' standard entrypoints.

### PostgreSQL

```bash
dck run -d \
  -n postgres \
  -p 5432:5432 \
  --restart unless-stopped \
  --vol postgres-data:/var/lib/postgresql/data \
  -e POSTGRES_DB=app \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD='change-this-password' \
  postgres:16
```

### MySQL

```bash
dck run -d \
  -n mysql \
  -p 3306:3306 \
  --restart unless-stopped \
  --vol mysql-data:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD='change-this-password' \
  -e MYSQL_DATABASE=app \
  mysql:8
```

### Redis

```bash
dck run -d \
  -n redis \
  -p 6379:6379 \
  --restart unless-stopped \
  --vol redis-data:/data \
  redis:7 redis-server --appendonly yes
```

Check state and logs:

```bash
dck ps
dck logs --tail 100 postgres
dck exec postgres pg_isready -U app -d app
dck exec redis redis-cli ping
```

## 9. Minecraft Java server using `$PWD`

`$PWD` is useful only when the current directory is an allowed host directory. Do not run this from `/root/test`; copy the server to `/data/minecraft` first.

```bash
mkdir -p /data/minecraft
# Copy server.jar and existing worlds/plugins into /data/minecraft.
cd /data/minecraft
ls -lh server.jar
printf 'eula=true\n' > eula.txt

dck run -d \
  -n minecraft \
  -h minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --restart-delay 1m \
  --vol "$PWD:/data" \
  --workdir /data \
  --memory 4g \
  --cpus 4 \
  eclipse-temurin:21 \
  java -Xms1G -Xmx4G -jar server.jar nogui
```

Monitor the server:

```bash
dck ps -a
dck logs --tail 100 minecraft
dck logs -f minecraft
ss -ltnp | grep 25565
```

Look for `Done (...)! For help, type "help"`. Connect to `VPS_PUBLIC_IP:25565`. Minecraft's own files under `/data` are on the bind mount and are not included in dck's container backup; back up `/data/minecraft` separately.

## 10. Java/Paper server with a start script

```bash
cd /data/minecraft
cat > start.sh <<'EOF'
#!/bin/sh
set -eu
cd /data
exec java -Xms1G -Xmx4G -jar server.jar nogui
EOF
chmod +x start.sh

dck run -d \
  -n minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --restart-delay 1m \
  --vol "$PWD:/data" \
  --workdir /data \
  eclipse-temurin:21 \
  ./start.sh
```

Avoid using Paper's `/restart` command with a missing host-side `start.sh`; let dck restart the main process. If the server exits, `--restart-delay 1m` performs recovery after one minute.

## 11. Backups

Automatic backups archive the writable overlay and named volumes. They do not archive host bind mounts. For the full backup guide — including automatic backups, manual backups, restoration, downloading to your local machine, bind-mount workarounds, edge cases, and best practices — see [Backups Guide](backups.md).

```bash
dck run -d \
  -n backup-test \
  --restart unless-stopped \
  --vol backup-data:/data \
  alpine:latest \
  sh -c 'echo backup-data > /data/test.txt; sleep 3600'

dck bootstrap --install
dck backup enable backup-test --interval 1h --retention 7
dck backup status backup-test
dck backup list
```

Create a manual backup of a stopped container:

```bash
dck stop backup-test
dck backup create backup-test -o /data/backups/backup-test/manual.tar.gz
dck backup restore backup-test /data/backups/backup-test/manual.tar.gz
dck start backup-test
```

Verify an archive against its checksum sidecar (without one, dck reports the archive as unverified):

```bash
dck backup verify /data/backups/backup-test/manual.tar.gz
```

Clean up:

```bash
dck backup disable backup-test
dck rm -f backup-test
dck volume rm backup-data
```

## 12. Inspect, execute, copy, and browse

```bash
dck inspect web
dck inspect --sensitive web
dck exec -i -t web /bin/sh
dck console web
dck top web
dck stats --no-stream web
dck fs ls web /
dck fs tree web /etc/nginx
dck fs find web /etc --name '.conf' --type f
dck fs cat web /etc/hostname
dck cp ./local.conf web:/etc/app/config.conf
dck cp web:/etc/app/config.conf /data/config-backup/
```

## 13. Dynamic ports

```bash
dck port web
dck port add web 8081:80
dck port add web 5353:53/udp
dck port remove web 8081
dck port rm web 5353/udp
```

## 14. `dck.toml` multi-container stack

```bash
mkdir -p /data/stack
cd /data/stack
cat > dck.toml <<'EOF'
[container.db]
image = "postgres:16"
restart = "unless-stopped"
volumes = ["stack-db:/var/lib/postgresql/data"]
env = { POSTGRES_DB = "app", POSTGRES_USER = "app", POSTGRES_PASSWORD = "change-me" }

[container.web]
image = "nginx:alpine"
restart = "unless-stopped"
ports = ["8080:80"]
volumes = ["/data/stack/site:/usr/share/nginx/html"]
depends_on = { db = "service_started" }
EOF
mkdir -p /data/stack/site
echo 'stack is up' > /data/stack/site/index.html

dck up -f dck.toml
dck ps
dck down -f dck.toml
```

Generate a starting config from existing named containers:

```bash
dck up --generate -f generated.toml
```

## 15. Registry, image transfer, and build

```bash
dck login registry.example.com
dck build -t registry.example.com/team/app:1.0 .
dck push registry.example.com/team/app:1.0
dck export registry.example.com/team/app:1.0 -o /data/app-1.0.tar.gz
dck import /data/app-1.0.tar.gz
dck verify registry.example.com/team/app:1.0
dck logout registry.example.com
```

## 16. Cluster, services, and functions

Single-node cluster initialization:

```bash
export DCK_TOKEN='replace-with-a-long-random-token'
dck cluster init --name production --bind 10.0.2.1 --port 7946 --api-port 2375 --token "$DCK_TOKEN"
dck cluster info
dck cluster node ls
dck cluster join-token
```

Replicated service:

```bash
dck service create --name api --replicas 3 --port 8080:80 nginx:alpine
dck service ls
dck service scale api 5
dck service update api --image nginx:1.27-alpine
dck service rm api
```

Serverless function:

```bash
# Replace the image below with your function image exposing /handler on port 8080.
dck fn deploy --name hello --port 8080 --timeout 30 --idle 300 --warm 1 registry.example.com/team/hello-function:latest
dck fn ls
dck fn call hello --data '{"name":"dck"}'
dck fn rm hello
```

## 17. Maintenance and boot recovery

```bash
dck info
dck events --since "2026-01-01T00:00:00Z"
dck system prune
dck update --check
dck bootstrap --install
systemctl status dck-bootstrap
journalctl -u dck-bootstrap -f
```

Use `dck bootstrap --remove` only when you no longer want boot recovery and scheduled backups.

## 18. Cleanup checklist

```bash
dck ps -a
dck stop minecraft
# Back up bind-mounted files separately before deleting their host directory.
dck rm -f minecraft
dck volume ls
dck volume prune
```

For every command's complete syntax, see [CLI Command Reference](commands.md). For installation and troubleshooting, see [Running dck](running.md).
