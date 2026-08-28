<!-- cardinal-version:start -->
**Documentation version:** `2.0.5`
**Project release:** `v2.0.5`
<!-- cardinal-version:end -->

# Compose / Развёртывание

## `cardinal up [имя] [-f <файл>]`

Создать и запустить контейнеры из compose-файла.

Автоопределение (по приоритету):
1. `cardinal.toml`
2. `compose.yaml`
3. `compose.yml`
4. `docker-compose.yaml`
5. `docker-compose.yml`

```
cardinal up                    # автоопределение
cardinal up myapp              # запустить только "myapp"
cardinal up -f docker-compose.prod.yaml
cardinal up myapp              # запустить только сервис "myapp"
cardinal bootstrap --install  # отдельно установить recovery при загрузке
```

## `cardinal down [имя] [-f <файл>]`

Остановить и удалить контейнеры из compose-файла.

```
cardinal down                  # остановить + удалить
cardinal down myapp            # только "myapp"
cardinal down -f cardinal.toml
cardinal down -a               # удалить ВСЕ контейнеры без чтения config
# Именованные volumes удаляйте отдельно и осторожно:
cardinal volume prune
```

## compose.yaml справочник

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

### Поддерживаемые поля compose

| Поле | Статус | Примечания |
|---|---|---|
| `services.<name>.image` | ✅ | |
| `services.<name>.build` | ✅ | Путь или inline-сборка |
| `services.<name>.ports` | ✅ | `HOST:CONTAINER`, `HOST:CONTAINER/PROTO` |
| `services.<name>.environment` | ✅ | Map или список |
| `services.<name>.env_file` | ✅ | |
| `services.<name>.volumes` | ✅ | Bind, named, tmpfs |
| `services.<name>.command` | ✅ | |
| `services.<name>.working_dir` | ✅ | |
| `services.<name>.user` | ✅ | |
| `services.<name>.restart` | ✅ | |
| `services.<name>.depends_on` | ✅ | Простой или с condition |
| `services.<name>.healthcheck` | ✅ | Полная поддержка |
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
| `volumes` | ✅ | Именованные тома |
| `networks` | ✅ | Bridge-сети |
| `services.<name>.deploy` | ✅ | replicas, resources, restart_policy, update_config, placement |
| `services.<name>.secrets` | ✅ | Инжекция секретов как файлов (source, target, uid, gid, mode) |
| `services.<name>.configs` | ✅ | Инжекция конфигов как файлов (source, target, uid, gid, mode) |

### Пример: full-stack приложение

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

## Secrets (Секреты)

Определяются на верхнем уровне и используются в сервисах:

```yaml
secrets:
  db_password:
    file: ./secrets/db_password.txt
  api_key:
    file: ./secrets/api_key.txt

services:
  app:
    image: myapp:latest
    secrets:
      - db_password
      - source: db_password
        target: /app/config/db_pass
        uid: "1000"
        gid: "1000"
        mode: 0600
```

Секреты монтируются как файлы:
- По умолчанию: `/run/secrets/<имя>`
- Права по умолчанию: `0444`

## Configs (Конфиги)

Работают как секреты, но монтируются в `/`:

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

Конфиги монтируются как файлы:
- По умолчанию: `/<имя>`
- Права по умолчанию: `0444`

## Примеры из реальной жизни

### Пример 1: Бот + Сайт + БД (разные директории)

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
cardinal up           # запустить все 3 сервиса
cardinal up bot       # запустить только бота
```

---

### Пример 2: Всё в одной папке проекта

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

### Пример 3: Абсолютные пути внутри `/data`

```
/data/
├── bot/
│   └── main.py
├── api/
│   └── server.js
/data/
├── cardinal/
│   └── compose.yaml
├── secrets/
│   ├── db_pass.txt
│   └── bot_token.txt
└── mysql/
```

**compose.yaml (внутри `/data/cardinal/`):**

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
cd /data/cardinal
cardinal up
# Recovery при загрузке устанавливается отдельно:
cardinal bootstrap --install
```

---

## Советы по написанию compose-файлов

### Правила указания путей

| Синтаксис volume | Что происходит |
|---|---|
| `./site:/app` | Относительно compose.yaml. Папка `site/` рядом с compose.yaml |
| `/data/site:/app` | Абсолютный путь — работает откуда угодно |
| `site-data:/app` | Именованный том — управляется cardinal (`~/.cardinal/volumes/site-data/`) |

### Связь между контейнерами

Контейнеры видят друг друга по **имени сервиса**:

```yaml
services:
  db:    # → hostname "db"
  site:  # → hostname "site"
  bot:   # → hostname "bot"
```

Внутри контейнера: `ping db`, `curl site:3000`, `psql -h db -U user`

### depends_on

```yaml
depends_on:
  db:
    condition: service_healthy   # ждать пока healthcheck пройдёт
  redis:
    condition: service_started   # просто дождаться запуска
```

### Secrets vs Configs

| Особенность | Secrets | Configs |
|---|---|---|
| Путь по умолчанию | `/run/secrets/<name>` | `/<name>` |
| Права по умолчанию | `0444` | `0444` |
| Для чего | Пароли, токены, ключи | Конфиги, nginx conf, SSL-сертификаты |

### Автостарт при загрузке

```bash
cardinal up                    # запустить настроенные сервисы
# recovery при загрузке устанавливается отдельно:
cardinal bootstrap --install  # установить systemd-сервис
```

После перезагрузки: `systemctl status cardinal-bootstrap` проверит, что все контейнеры с `restart: always` запущены.

## Формат cardinal.toml

Для обратной совместимости с существующими проектами cardinal:

```toml
containers = ["web", "api"]

[web]
image = "nginx:alpine"
ports = ["80:80"]
volumes = ["./www:/usr/share/nginx/html"]
environment = ["NGINX_HOST=example.com"]
restart = "unless-stopped"
```
