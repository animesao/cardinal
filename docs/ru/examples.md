<!-- dck-version:start -->
**Documentation version:** `1.24.13`
**Project release:** `v1.24.13`
<!-- dck-version:end -->

# Примеры команд dck

Практические рецепты для Linux. Замените имена образов, пароли, пути и публичные порты на свои.

> dck специально отклоняет защищённые host bind sources: `/root`, `/etc`, `/var`, `/usr`, `/opt`, `/run`. Используйте `/data/<project>` или именованный volume. Host-каталог нужно создать заранее.

## 1. Быстрый smoke test

```bash
dck version
dck pull alpine:latest
dck run --rm alpine:latest echo "DCK OK"
dck ps -a
```

В результате разового теста должна появиться строка:

```text
DCK OK
```

## 2. Долгоживущий тестовый контейнер

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

Нажмите `Ctrl+C`, чтобы выйти из просмотра логов; контейнер при этом не остановится.

```bash
dck stop dck-test
dck ps -a
dck rm dck-test
```

## 3. Проверка перезапуска после сбоя

Этот процесс завершается через три секунды. dck должен запустить его снова после заданной задержки.

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

После проверки удалите контейнер:

```bash
dck rm -f restart-test
```

Если контейнер должен оставаться остановленным после ручного `dck stop`, используйте `unless-stopped`, а не `always`.

## 4. Переменные окружения и `.env`

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

`-e KEY=VALUE` можно повторять и использовать для небольшого числа значений. Секреты registry и приложения не стоит помещать прямо в историю shell; используйте защищённый env-файл или отдельное хранилище секретов.

## 5. Именованные volumes

Именованные volumes хранятся в dck и переживают удаление контейнера.

```bash
dck volume create demo-data
dck run --rm --vol demo-data:/data alpine:latest \
  sh -c 'echo saved > /data/message.txt'
dck run --rm --vol demo-data:/data alpine:latest \
  cat /data/message.txt
dck volume inspect demo-data
dck volume rm demo-data
```

Bind mount подключает существующий каталог хоста:

```bash
mkdir -p /data/demo-app
dck run --rm -v /data/demo-app:/app alpine:latest \
  sh -c 'echo host-visible > /app/message.txt'
cat /data/demo-app/message.txt
```

## 6. Web-сервер

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

Источник на хосте — `/data/site`; `/usr/share/nginx/html` — путь внутри nginx-контейнера.

## 7. Python-приложение или бот

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

Для Discord или Telegram-бота замените `app.py` и `requirements.txt`, передайте token через `--env-file` и оставьте политику `--restart unless-stopped`.

## 8. PostgreSQL, MySQL и Redis

Для данных баз используйте именованные volumes. Примеры предполагают стандартные entrypoints образов.

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

Проверка состояния и логов:

```bash
dck ps
dck logs --tail 100 postgres
dck exec postgres pg_isready -U app -d app
dck exec redis redis-cli ping
```

## 9. Minecraft Java через `$PWD`

`$PWD` удобен только внутри разрешённого каталога хоста. Не запускайте это из `/root/test`; сначала скопируйте сервер в `/data/minecraft`.

```bash
mkdir -p /data/minecraft
# Скопируйте server.jar и существующие миры/plugins в /data/minecraft.
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

Проверка Minecraft:

```bash
dck ps -a
dck logs --tail 100 minecraft
dck logs -f minecraft
ss -ltnp | grep 25565
```

Ищите в логах `Done (...)! For help, type "help"`. Подключение: `PUBLIC_IP_VPS:25565`. Файлы Minecraft в `/data` находятся на bind mount и не входят в backup контейнера dck; каталог `/data/minecraft` нужно архивировать отдельно.

## 10. Java/Paper через start-скрипт

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

Не используйте Paper-команду `/restart`, если host-side `start.sh` отсутствует. Пусть dck перезапускает главный процесс. Если сервер завершится, `--restart-delay 1m` восстановит его через минуту.

## 11. Backup

Automatic backup архивирует writable overlay и именованные volumes, но не host bind mounts. Полное руководство по бэкапам — автоматические, ручные, восстановление, скачивание на ПК, обход bind-монтов, граничные случаи и лучшие практики — см. в [Руководстве по бэкапам](backups.md).

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

Разовый backup остановленного контейнера:

```bash
dck stop backup-test
dck backup create backup-test -o /data/backups/backup-test/manual.tar.gz
dck backup restore backup-test /data/backups/backup-test/manual.tar.gz
dck start backup-test
```

Проверка архива по контрольной сумме (если checksum-файла рядом нет, dck сообщит, что архив не проверен):

```bash
dck backup verify /data/backups/backup-test/manual.tar.gz
```

Очистка:

```bash
dck backup disable backup-test
dck rm -f backup-test
dck volume rm backup-data
```

## 12. Inspect, exec, copy и filesystem

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

## 13. Динамические порты

```bash
dck port web
dck port add web 8081:80
dck port add web 5353:53/udp
dck port remove web 8081
dck port rm web 5353/udp
```

## 14. Multi-container stack через `dck.toml`

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

Создать исходную конфигурацию из существующих именованных контейнеров:

```bash
dck up --generate -f generated.toml
```

## 15. Registry, перенос и build

```bash
dck login registry.example.com
dck build -t registry.example.com/team/app:1.0 .
dck push registry.example.com/team/app:1.0
dck export registry.example.com/team/app:1.0 -o /data/app-1.0.tar.gz
dck import /data/app-1.0.tar.gz
dck verify registry.example.com/team/app:1.0
dck logout registry.example.com
```

## 16. Cluster, services и functions

Инициализация одновузлового cluster:

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
# Замените образ на свой function image с HTTP handler /handler на порту 8080.
dck fn deploy --name hello --port 8080 --timeout 30 --idle 300 --warm 1 registry.example.com/team/hello-function:latest
dck fn ls
dck fn call hello --data '{"name":"dck"}'
dck fn rm hello
```

## 17. Обслуживание и recovery после reboot

```bash
dck info
dck events --since "2026-01-01T00:00:00Z"
dck system prune
dck update --check
dck bootstrap --install
systemctl status dck-bootstrap
journalctl -u dck-bootstrap -f
```

Используйте `dck bootstrap --remove`, только если больше не нужны запуск после reboot и scheduled backups.

## 18. Чек-лист очистки

```bash
dck ps -a
dck stop minecraft
# Перед удалением host directory отдельно сохраните bind-mounted файлы.
dck rm -f minecraft
dck volume ls
dck volume prune
```

Полный синтаксис каждой команды — в [Справочнике CLI](commands.md), установка и troubleshooting — в [Руководстве по запуску](running.md).
