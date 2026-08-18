<!-- dck-version:start -->
**Documentation version:** `1.60.1`
**Project release:** `v1.60.1`
<!-- dck-version:end -->

# Compose Examples — 15 Real-World Configurations

Every example is a complete `compose.yaml`. Use them as-is or mix and match.

## Quick Start with .env

Services that use `${VAR}` need a `.env` file in the same directory:

```bash
cat > .env << 'EOF'
DB_PASSWORD=mysecretpass
MYSQL_ROOT_PASS=rootpass
MYSQL_APP_PASS=apppass
REDIS_PASS=redispass
GITHUB_TOKEN=ghp_xxx
EOF
```

---

## 1. Minecraft Server (Paper)

Includes stale lock cleanup so the server always starts fresh.

```yaml
services:
  mc:
    image: eclipse-temurin:21-jdk
    working_dir: /server
    volumes:
      - /data/mc-server:/server
    command: >
      sh -c "
        rm -f world/session.lock world_nether/session.lock world_the_end/session.lock
        echo eula=true > eula.txt
        exec java -Xms512M -Xmx4G -jar paper-*.jar nogui
      "
    restart: always
    ports:
      - "25565:25565"
    dns:
      - 8.8.8.8
```

- `rm -f *.lock` — cleans stale Paper locks from previous crashes
- `working_dir` — working directory inside container
- `volumes: /data/mc-server:/server` — mount host folder to container
- `restart: always` — restart on crash or reboot
- `dns` — fixes DNS for downloading mods/plugins

---

## 2. Web Server (Nginx + static files)

```yaml
services:
  web:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./html:/usr/share/nginx/html:ro
      - ./nginx-conf:/etc/nginx/conf.d:ro
      - ./ssl:/etc/nginx/ssl:ro
    labels:
      - "app=website"
      - "env=production"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 30s
      timeout: 10s
      retries: 3
```

- `:ro` — read-only mount (security)
- **Use directories, not files** — dck mounts directories only (e.g. `./nginx-conf/` not `./nginx.conf`)
- `labels` — metadata for filtering with `dck ps --filter`
- `healthcheck` — check if nginx responds every 30s

---

## 3. Full-Stack App (Node.js + Python + Postgres)

```yaml
services:
  db:
    image: postgres:16-alpine
    restart: always
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init-sql:/docker-entrypoint-initdb.d
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD: ${DB_PASSWORD:-secret}
    env_file:
      - .env
    healthcheck:
      test: pg_isready -U postgres
      interval: 5s
      retries: 10
    networks:
      - backend

  api:
    build: ./api
    restart: always
    working_dir: /app
    volumes:
      - ./api:/app
      - /app/node_modules
    ports:
      - "3000:3000"
    environment:
      DB_HOST: db
    depends_on:
      db:
        condition: service_healthy
    ulimits:
      nofile:
        soft: 65536
        hard: 65536
    networks:
      - backend

  bot:
    image: python:3.12-slim
    restart: always
    working_dir: /app
    volumes:
      - ./bots:/app
    command: sh -c "pip install -r requirements.txt && exec python main.py"
    environment:
      DB_HOST: db
    secrets:
      - bot_token
    depends_on:
      db:
        condition: service_healthy
    networks:
      - backend

volumes:
  pgdata:

secrets:
  bot_token:
    file: ./secrets/bot_token.txt

networks:
  backend:
    internal: true
```

- **Use directories for mounts** — `./init-sql/` instead of `./init.sql` (dck limitation)
- `volumes: /app/node_modules` — unnamed volume, prevents node_modules from being overwritten by bind mount
- `env_file` — load env vars from file
- `secrets` — inject sensitive files as `/run/secrets/<name>`
- `networks: backend` — isolates backend services
- `ulimits` — set file descriptor limits

---

## 4. Database Cluster (MySQL + Redis + Adminer)

```yaml
services:
  mysql:
    image: mysql:8
    restart: always
    volumes:
      - mysql_data:/var/lib/mysql
      - ./mysql-conf:/etc/mysql/conf.d
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASS}
      MYSQL_DATABASE: myapp
      MYSQL_USER: app
      MYSQL_PASSWORD: ${MYSQL_APP_PASS}
    command: >
      --character-set-server=utf8mb4
      --collation-server=utf8mb4_unicode_ci
      --max_connections=200
    cap_add:
      - SYS_NICE
    healthcheck:
      test: mysqladmin ping -h localhost -u root -p${MYSQL_ROOT_PASS}
      interval: 10s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: always
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --requirepass ${REDIS_PASS}
    healthcheck:
      test: redis-cli -a ${REDIS_PASS} ping
      interval: 10s

  adminer:
    image: adminer:latest
    restart: always
    ports:
      - "8080:8080"
    environment:
      ADMINER_DEFAULT_SERVER: mysql

volumes:
  mysql_data:
  redis_data:
```

- **Required `.env` file**: create `.env` with `MYSQL_ROOT_PASS`, `MYSQL_APP_PASS`, `REDIS_PASS`
- Use `./mysql-conf/` directory (copy `my.cnf` inside) — dck doesn't mount single files
- `cap_add: SYS_NICE` — needed for MySQL to adjust thread priority
- Adminer — web UI at port 8080

---

## 5. Reverse Proxy (Nginx + multiple services)

```yaml
services:
  nginx:
    image: nginx:alpine
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx-conf:/etc/nginx/conf.d:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - app1
      - app2

  app1:
    image: node:20-alpine
    working_dir: /app
    volumes:
      - ./app1:/app
    command: node server.js
    environment:
      - PORT=3000
    expose:
      - "3000"
    restart: always

  app2:
    image: python:3.12-slim
    working_dir: /app
    volumes:
      - ./app2:/app
    command: python app.py
    environment:
      - PORT=5000
    expose:
      - "5000"
    restart: always
```

- `expose` — port accessible to linked services only (not to host)
- Nginx resolves `app1:3000`, `app2:5000`
- Place `app1.conf` inside `./nginx-conf/` directory

**nginx-conf/default.conf:**
```nginx
upstream app1 { server app1:3000; }
upstream app2 { server app2:5000; }
server {
    listen 80;
    location /app1 { proxy_pass http://app1; }
    location /app2 { proxy_pass http://app2; }
}
```

---

## 6. Cron / Scheduled Tasks

```yaml
services:
  cron:
    image: alpine:3.19
    restart: always
    volumes:
      - ./scripts:/scripts
    command: >
      sh -c "
        echo '* * * * * cd /scripts && sh task.sh' > /var/spool/cron/crontabs/root &&
        crond -f -l 2
      "
    environment:
      - TZ=Europe/Moscow
    stop_grace_period: 5s
```

- `stop_grace_period` — time to wait before force-killing (default 10s)
- `TZ` — set timezone for cron
- Runs `task.sh` every minute

---

## 7. File Server (nginx + uploads)

```yaml
services:
  storage:
    image: nginx:alpine
    restart: always
    ports:
      - "9000:80"
    volumes:
      - /data/storage:/usr/share/nginx/html:ro
      - /data/uploads:/uploads:rw
    tmpfs:
      - /var/cache/nginx
      - /var/run
```

**Simplified for dck** — omits `user: "1000:1000"` + `read_only` because nginx needs to write cache. For hardened security use a custom nginx.conf with temp paths.

- `tmpfs` — temporary in-memory filesystem for logs/cache
- **Note:** `security_opt` values must be quoted YAML strings: `"no-new-privileges:true"`
- **Note:** `read_only: true` + `user: "1000:1000"` breaks nginx (can't create `/var/cache/nginx/client_temp`)

---

## 8. CI Runner (self-hosted, reference only)

> **Not a ready-to-run dck recipe.** The common `summerwind/actions-runner`
> setup expects Docker-in-Docker or access to a host Docker socket. dck blocks
> protected host bind sources such as `/var/run/docker.sock`, so use a
> dedicated host-side runner integration or an image that does not require
> Docker socket access. The snippet below documents the container-side
> settings only.

```yaml
services:
  runner:
    image: summerwind/actions-runner:latest
    restart: always
    volumes:
      - runner_data:/home/runner
    environment:
      - RUNNER_NAME=my-runner
      - RUNNER_REPO=https://github.com/user/repo
      - RUNNER_TOKEN=${GITHUB_TOKEN}
      - RUNNER_LABELS=dck,production
    working_dir: /home/runner
    privileged: true
    dns:
      - 8.8.8.8

volumes:
  runner_data:
```

- `privileged: true` alone does not provide Docker-in-Docker.
- For dck, treat this as a configuration reference and use a host-side runner integration or a runtime-specific image that does not require `/var/run/docker.sock`.

---

## 9. Full-Stack with Configs (Nginx + API + DB + Redis)

```yaml
services:
  nginx:
    image: nginx:alpine
    restart: always
    ports:
      - "443:443"
    volumes:
      - ./site/public:/var/www/html:ro
    configs:
      - source: nginx_conf
        target: /etc/nginx/conf.d/default.conf
    secrets:
      - ssl_cert
      - ssl_key
    depends_on:
      - api

  api:
    build: ./api
    restart: always
    ports:
      - "3000:3000"
    environment:
      DB_HOST: db
      REDIS_HOST: redis
      NODE_ENV: production
    secrets:
      - db_password
    deploy:
      replicas: 2
      resources:
        limits:
          memory: "512m"
          cpus: "1.0"
        reservations:
          memory: "256m"
          cpus: "0.5"
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3

  db:
    image: postgres:16-alpine
    restart: always
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password
    healthcheck:
      test: pg_isready -U postgres
      interval: 5s
      retries: 10
    deploy:
      resources:
        limits:
          memory: "1g"
          cpus: "2.0"

  redis:
    image: redis:7-alpine
    restart: always
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    deploy:
      resources:
        limits:
          memory: "256m"

volumes:
  pgdata:
  redis_data:

configs:
  nginx_conf:
    file: ./nginx/default.conf

secrets:
  ssl_cert:
    file: /etc/ssl/certs/domain.crt
  ssl_key:
    file: /etc/ssl/private/domain.key
    mode: 0600
  db_password:
    file: ./secrets/db_pass.txt
```

- `configs` — inject config files (default path: `/<name>`)
- `secrets` — inject sensitive files (default path: `/run/secrets/<name>`)
- `deploy.resources.limits` — hard limits (container can't exceed)
- `deploy.resources.reservations` — guaranteed resources
- `replicas: 2` — for clustering mode
- **Secrets are loaded from host paths** — ensure `./secrets/db_pass.txt` exists

---

## 10. Development Environment

```yaml
services:
  frontend:
    image: node:20
    working_dir: /app
    volumes:
      - ./frontend:/app
      - /app/node_modules
    ports:
      - "5173:5173"
    command: npm run dev
    environment:
      - VITE_API_URL=http://localhost:3000
    stdin_open: true
    tty: true

  backend:
    build: ./backend
    working_dir: /app
    volumes:
      - ./backend:/app
      - /app/node_modules
    ports:
      - "3000:3000"
    command: npx nodemon server.js
    environment:
      - DB_HOST=db
      - DEBUG=true
    depends_on:
      - db

  db:
    image: postgres:16-alpine
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: devpass
    ports:
      - "5432:5432"

volumes:
  pgdata:
```

- `stdin_open: true` + `tty: true` — interactive mode (like `-it`)
- `npx nodemon` — auto-restart on file changes
- `.env` file can override variables

---

## 11. Single-Command Application

```yaml
services:
  backup:
    image: alpine:3.19
    volumes:
      - /data/shared:/data:ro
      - /data/backups:/backups
    command: >
      sh -c "tar czf /backups/data-$(date +%Y%m%d).tar.gz /data"
    restart: "no"
```

- `restart: "no"` — run once and stop
- Useful for backups, migrations, batch jobs

---

## 12. Private Registry Mirror

```yaml
services:
  registry:
    image: registry:2
    restart: always
    ports:
      - "5000:5000"
    volumes:
      - registry_data:/var/lib/registry
    environment:
      - REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io
      - REGISTRY_STORAGE_DELETE_ENABLED=true
    healthcheck:
      test: curl -f http://localhost:5000/v2/
      interval: 30s

volumes:
  registry_data:
```

---

## 13. Prometheus + Grafana Monitoring

**Important:** dck mounts volumes as directories only. Use a directory config mount instead of a file.

```yaml
services:
  prometheus:
    image: prom/prometheus
    restart: always
    volumes:
      - ./prometheus-conf:/etc/prometheus:ro
      - prom_data:/prometheus
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana
    restart: always
    volumes:
      - grafana_data:/var/lib/grafana
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

volumes:
  prom_data:
  grafana_data:
```

Create `./prometheus-conf/prometheus.yml` on the host:

```bash
mkdir -p prometheus-conf
cat > prometheus-conf/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]
EOF
```

- **Directory mount** `./prometheus-conf:/etc/prometheus:ro` works (file mount `./prometheus.yml:/etc/prometheus/prometheus.yml` does not in dck)
- Grafana accessible at `http://host:3001`, admin/admin or `GF_SECURITY_ADMIN_PASSWORD`

---

## 14. WordPress + MySQL

A classic blog setup with persistent data.

```yaml
services:
  db:
    image: mysql:8
    restart: always
    volumes:
      - wp_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASS}
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wpuser
      MYSQL_PASSWORD: ${MYSQL_APP_PASS}
    cap_add:
      - SYS_NICE

  wordpress:
    image: wordpress:6-apache
    restart: always
    ports:
      - "8080:80"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wpuser
      WORDPRESS_DB_PASSWORD: ${MYSQL_APP_PASS}
      WORDPRESS_DB_NAME: wordpress
    volumes:
      - wp_html:/var/www/html
    depends_on:
      - db

volumes:
  wp_data:
  wp_html:
```

---

## 15. Elasticsearch + Kibana

Log aggregation stack.

```yaml
services:
  elasticsearch:
    image: elasticsearch:8.15.0
    restart: always
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - ES_JAVA_OPTS=-Xms512m -Xmx512m
    ports:
      - "9200:9200"
    volumes:
      - es_data:/usr/share/elasticsearch/data
    ulimits:
      memlock:
        soft: -1
        hard: -1
      nofile:
        soft: 65536
        hard: 65536

  kibana:
    image: kibana:8.15.0
    restart: always
    ports:
      - "5601:5601"
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    depends_on:
      - elasticsearch

volumes:
  es_data:
```

- `ulimits memlock: -1` — required for Elasticsearch mmap
- Kibana at `http://host:5601`

---

## Important dck Gotchas

| Issue | Why | Fix |
|---|---|---|
| **File bind mount** fails | dck only mounts directories (bind mount `./file` → `mount bind exit 32`) | Use a directory: `./dir/` instead of `./file` |
| **Cgroup** warning | `/sys/fs/cgroup/dck` is read-only on VPS/LXC | Non-fatal, container runs without resource limits |
| **`session.lock`** Minecraft crash | Paper leaves stale locks on host volume | Add `rm -f world/*/session.lock` to start script |
| **`security_opt`** YAML error | `no-new-privileges:true` has two colons | Quote it: `"no-new-privileges:true"` |
| **`.env`** not loaded | `${VAR}` in environment expands from `.env` file | Create `.env` in same directory as `compose.yaml` |
| **`ip link delete`** warning on stop | Cleanup of veth pair fails on some kernels | Non-fatal, interface cleans up on host reboot |

## Quick Reference: All compose fields

| Field | Description | Example |
|---|---|---|
| `image` | Container image | `nginx:alpine` |
| `build` | Build from Dockerfile (path or object) | `./api` or `context: ./api dockerfile: Dockerfile.prod` |
| `command` | Override CMD | `node app.js` |
| `entrypoint` | Override ENTRYPOINT | `["/bin/sh", "-c"]` |
| `ports` | Port mapping | `"80:80"`, `"443:443/tcp"` |
| `expose` | Port available to linked services | `"3000"` |
| `volumes` | File/directory mounts | `./src:/app:ro`, `data:/var/lib/data` |
| `environment` | Env vars | `KEY=val` or `{ KEY: val }` |
| `env_file` | Load from file | `.env.prod` |
| `restart` | Restart policy | `no`, `always`, `on-failure`, `unless-stopped` |
| `dns` | DNS servers | `8.8.8.8` |
| `dns_search` | DNS search domains | `example.com` |
| `cap_add` | Add Linux capabilities | `NET_ADMIN`, `SYS_PTRACE` |
| `cap_drop` | Remove capabilities | `ALL` |
| `user` | Run as user | `1000:1000`, `www-data` |
| `working_dir` | Working directory | `/app` |
| `hostname` | Container hostname | `myserver` |
| `labels` | Metadata | `app=myapp`, `env=prod` |
| `healthcheck` | Health check | `test: curl -f http://localhost` |
| `depends_on` | Startup order | `db: condition: service_healthy` |
| `networks` | Networks to join | `frontend`, `backend` |
| `network_mode` | Network mode | `bridge`, `host`, `none` |
| `sysctls` | Kernel parameters | `net.core.somaxconn=1024` |
| `ulimits` | Resource limits | `nofile: 1024:2048` |
| `secrets` | Sensitive files | `- db_password` |
| `configs` | Config files | `- nginx_conf` |
| `deploy` | Deploy configuration | `replicas: 3`, `resources: limits: memory: 512m` |
| `stdin_open` | Keep stdin open (`-i`) | `true` |
| `tty` | Allocate TTY (`-t`) | `true` |
| `read_only` | Read-only rootfs | `true` |
| `tmpfs` | In-memory filesystem | `/tmp:size=100M` |
| `privileged` | Full container privileges | `true` (use with caution) |
| `stop_signal` | Signal to stop | `SIGTERM`, `SIGQUIT` |
| `stop_grace_period` | Grace period before kill | `10s` |
| `extra_hosts` | Extra /etc/hosts entries | `"host.docker.internal:host-gateway"` |
