<!-- dck-version:start -->
**Documentation version:** `1.60.9`
**Project release:** `v1.60.9`
<!-- dck-version:end -->

# Compose / Deployment

## `dck up [name] [-f <file>]`

Create and start containers from a compose file.

Auto-detection order:
1. `dck.toml`
2. `compose.yaml`
3. `compose.yml`
4. `docker-compose.yaml`
5. `docker-compose.yml`

```
dck up                    # auto-detect
dck up myapp              # start only the "myapp" service
dck up -f docker-compose.prod.yaml
dck up myapp              # start only the "myapp" service
dck bootstrap --install  # install boot recovery separately
```

## `dck down [name] [-f <file>]`

Stop and remove containers from a compose file.

```
dck down                  # stop + remove
dck down myapp            # stop + remove only "myapp"
dck down -f dck.toml
dck down -a               # remove ALL containers (ignore config)
# Remove named volumes separately, only when you are sure:
dck volume prune
```

## compose.yaml reference

```yaml
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./www:/usr/share/nginx/html
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
    environment:
      - NGINX_HOST=example.com
    restart: unless-stopped
    depends_on:
      - api
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 30s
      timeout: 10s
      retries: 3
    dns:
      - 8.8.8.8
    networks:
      - frontend

  api:
    image: myapi:latest
    ports:
      - "3000:3000"
    environment:
      DB_HOST: db
      DB_NAME: ${DB_NAME:-test}
    env_file:
      - .env.prod
    volumes:
      - api-data:/app/data
    restart: always
    depends_on:
      db:
        condition: service_healthy
    ulimits:
      nofile:
        soft: 1024
        hard: 2048
    cap_add:
      - NET_ADMIN
    cap_drop:
      - ALL
    command: ["node", "server.js"]
    working_dir: /app
    networks:
      - backend

  db:
    image: postgres:16
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD_FILE: /run/secrets/db_pass
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    ports:
      - "5432"
    restart: always
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - backend

volumes:
  api-data:
    driver: local
  pgdata:

networks:
  frontend:
  backend:
    internal: true
```

### Supported compose fields

| Field | Status | Notes |
|---|---|---|
| `services.<name>.image` | ✅ | |
| `services.<name>.build` | ✅ | Path or inline build |
| `services.<name>.ports` | ✅ | `HOST:CONTAINER`, `HOST:CONTAINER/PROTO` |
| `services.<name>.environment` | ✅ | Map or list |
| `services.<name>.env_file` | ✅ | |
| `services.<name>.volumes` | ✅ | Bind, named, tmpfs |
| `services.<name>.command` | ✅ | |
| `services.<name>.working_dir` | ✅ | |
| `services.<name>.user` | ✅ | |
| `services.<name>.restart` | ✅ | |
| `services.<name>.depends_on` | ✅ | Simple or with condition |
| `services.<name>.healthcheck` | ✅ | Full support |
| `services.<name>.dns` | ✅ | |
| `services.<name>.dns_search` | ✅ | |
| `services.<name>.cap_add` | ✅ | |
| `services.<name>.cap_drop` | ✅ | |
| `services.<name>.ulimits` | ✅ | |
| `services.<name>.sysctls` | ✅ | |
| `services.<name>.labels` | ✅ | |
| `services.<name>.networks` | ✅ | |
| `services.<name>.entrypoint` | ✅ | |
| `services.<name>.expose` | ✅ | |
| `services.<name>.stop_signal` | ✅ | |
| `volumes` | ✅ | Named volumes |
| `networks` | ✅ | Bridge networks |
| `services.<name>.deploy` | ✅ | replicas, resources, restart_policy, update_config, placement |
| `services.<name>.secrets` | ✅ | File-based secret injection (source, target, uid, gid, mode) |
| `services.<name>.configs` | ✅ | File-based config injection (source, target, uid, gid, mode) |

### Example: full-stack app

**compose.yaml:**

```yaml
services:
  frontend:
    image: node:20
    working_dir: /app
    volumes:
      - ./frontend:/app
    ports:
      - "5173:5173"
    command: npm run dev
    environment:
      - VITE_API_URL=http://localhost:3000

  backend:
    build: ./backend
    ports:
      - "3000:3000"
    environment:
      DB_URL: postgres://user:pass@db:5432/myapp
      REDIS_URL: redis://redis:6379
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_started
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: myapp
    healthcheck:
      test: pg_isready -U user -d myapp
      interval: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    healthcheck:
      test: redis-cli ping
      interval: 5s

volumes:
  pgdata:
```

## Secrets

Define secrets at the top level and reference them in services:

```yaml
secrets:
  db_password:
    file: ./secrets/db_password.txt
  api_key:
    file: ./secrets/api_key.txt
    external: false

services:
  app:
    image: myapp:latest
    secrets:
      - db_password
      - api_key
      # Extended syntax:
      - source: db_password
        target: /app/config/db_pass
        uid: "1000"
        gid: "1000"
        mode: 0600
```

Secrets are mounted as files inside the container:
- Default target: `/run/secrets/<name>`
- Default permissions: `0444`

## Configs

Configs work identically to secrets but mount to `/` by default:

```yaml
configs:
  nginx_conf:
    file: ./nginx.conf
  app_config:
    file: ./config/app.yml

services:
  web:
    image: nginx:alpine
    configs:
      - nginx_conf
      - source: app_config
        target: /app/config.yml
        mode: 0644
```

Configs are mounted as files inside the container:
- Default target: `/<name>`
- Default permissions: `0444`

## Real-world examples

### Example 1: Bot + Site + DB (different directories)

```
/data/
├── mybot/
│   ├── main.py
│   └── requirements.txt
├── mysite/
│   ├── package.json
│   └── index.js
└── compose.yaml
```

**compose.yaml:**

```yaml
services:
  db:
    image: mysql:8
    restart: always
    volumes:
      - mysql_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: myapp
      MYSQL_USER: bot
      MYSQL_PASSWORD: botpass
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      retries: 5

  bot:
    image: python:3.12-slim
    restart: always
    working_dir: /app
    volumes:
      - /data/mybot:/app
    command: sh -c "pip install -r requirements.txt && python main.py"
    environment:
      DB_HOST: db
      DB_USER: bot
      DB_PASSWORD: botpass
      DB_NAME: myapp
    depends_on:
      db:
        condition: service_healthy
    labels:
      - "app=telegram-bot"
      - "env=production"

  site:
    image: node:20-alpine
    restart: always
    working_dir: /app
    volumes:
      - /data/mysite:/app
    ports:
      - "3000:3000"
    command: sh -c "npm install && node index.js"
    environment:
      DB_HOST: db
      DB_USER: bot
      DB_PASSWORD: botpass
      DB_NAME: myapp
    depends_on:
      db:
        condition: service_healthy

volumes:
  mysql_data:
```

```
cd /data
dck up           # start all 3 services
dck up bot       # start only the bot
```

---

### Example 2: Everything in one project folder

```
/home/user/myproject/
├── compose.yaml
├── .env
├── bots/
│   ├── __init__.py
│   └── main.py
├── site/
│   ├── public/
│   ├── index.js
│   ├── package.json
│   └── Dockerfile
├── nginx/
│   └── default.conf
└── scripts/
    └── init.sql
```

**.env:**

```
DB_PASSWORD=secret123
DOMAIN=example.com
```

**compose.yaml:**

```yaml
services:
  nginx:
    image: nginx:alpine
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/default.conf:/etc/nginx/conf.d/default.conf:ro
      - ./site/public:/var/www/html:ro
    depends_on:
      - site

  site:
    build: ./site
    restart: always
    working_dir: /app
    volumes:
      - ./site:/app
      - /app/node_modules
    ports:
      - "3000:3000"
    command: node index.js
    environment:
      DB_HOST: db
      DB_PASS: ${DB_PASSWORD}
    depends_on:
      db:
        condition: service_healthy

  bot:
    image: python:3.12-slim
    restart: always
    working_dir: /app
    volumes:
      - ./bots:/app
    command: sh -c "pip install -r requirements.txt && python main.py"
    environment:
      DB_HOST: db
      DB_PASS: ${DB_PASSWORD}
      BOT_TOKEN: ${BOT_TOKEN}
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    restart: always
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    healthcheck:
      test: pg_isready -U postgres
      interval: 5s
      retries: 10

volumes:
  pgdata:
```

---

### Example 3: Absolute paths under `/data`

```
/data/
├── bot/
│   └── main.py
├── api/
│   └── server.js
/data/
├── dck/
│   └── compose.yaml
├── secrets/
│   ├── db_pass.txt
│   └── bot_token.txt
└── mysql/
```

**compose.yaml (inside `/data/dck/`):**

```yaml
services:
  db:
    image: mysql:8
    restart: always
    volumes:
      - /data/mysql:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/db_pass
      MYSQL_DATABASE: myapp
    secrets:
      - db_pass

  bot:
    image: python:3.12-slim
    restart: always
    working_dir: /app
    volumes:
      - /data/bot:/app
    command: sh -c "pip install -r /app/requirements.txt && python /app/main.py"
    secrets:
      - source: db_pass
        target: /app/db_pass.txt
        mode: 0600
      - source: bot_token
        target: /app/token.txt
        mode: 0600
    depends_on:
      db:
        condition: service_healthy

  api:
    image: node:20-alpine
    restart: always
    working_dir: /app
    volumes:
      - /data/api:/app
    ports:
      - "4000:4000"
    command: node /app/server.js
    environment:
      DB_HOST: db
    secrets:
      - db_pass
    depends_on:
      db:
        condition: service_healthy

secrets:
  db_pass:
    file: /data/secrets/db_pass.txt
  bot_token:
    file: /data/secrets/bot_token.txt
```

```
cd /data/dck
dck up
# Install boot recovery separately:
dck bootstrap --install
```

---

## Tips for writing compose files

### Directory structure rules

| Volume syntax | What happens |
|---|---|
| `./site:/app` | Relative to compose.yaml location. `site/` folder next to compose.yaml |
| `/data/site:/app` | Absolute path — works from anywhere |
| `site-data:/app` | Named volume — managed by dck (`~/.dck/volumes/site-data/`) |

### Container-to-container communication

Each container can reach others by **service name**:

```yaml
services:
  db:    # → hostname "db"
  site:  # → hostname "site"
  bot:   # → hostname "bot"
```

Inside container: `ping db`, `curl site:3000`, `psql -h db -U user`

### depends_on

```yaml
depends_on:
  db:
    condition: service_healthy   # wait until healthcheck passes
  redis:
    condition: service_started   # just wait for start
```

### Secrets vs Configs

| Feature | Secrets | Configs |
|---|---|---|
| Default path | `/run/secrets/<name>` | `/<name>` |
| Default permissions | `0444` | `0444` |
| Use for | Passwords, tokens, keys | Config files, nginx conf, SSL certs |

### Auto-start on boot

```bash
dck up                    # start the configured services
# install boot recovery separately:
dck bootstrap --install  # install systemd service for existing containers
```

After reboot: `systemctl status dck-bootstrap` checks all containers with `restart: always` are running.

## dck.toml format

For backwards compatibility with existing dck projects:

```toml
containers = ["web", "api"]

[web]
image = "nginx:alpine"
ports = ["80:80"]
volumes = ["./www:/usr/share/nginx/html"]
environment = ["NGINX_HOST=example.com"]
restart = "unless-stopped"
```
