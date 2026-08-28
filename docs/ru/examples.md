<!-- cardinal-version:start -->
**Documentation version:** `2.0.5`
**Project release:** `v2.0.5`
<!-- cardinal-version:end -->

# Примеры команд cardinal

Практические рецепты для Linux. Замените имена образов, пароли, пути и публичные порты на свои.

> cardinal специально отклоняет защищённые host bind sources: `/root`, `/etc`, `/var`, `/usr`, `/opt`, `/run`. Используйте `/data/<project>` или именованный volume. Host-каталог нужно создать заранее.

## 1. Быстрый smoke test

```bash
cardinal version
cardinal pull alpine:latest
cardinal run --rm alpine:latest echo "CARDINAL OK"
cardinal ps -a
```

В результате разового теста должна появиться строка:

```text
CARDINAL OK
```

## 2. Долгоживущий тестовый контейнер

```bash
cardinal run -d \
  -n cardinal-test \
  --restart unless-stopped \
  --restart-delay 10s \
  alpine:latest \
  sh -c 'i=0; while true; do i=$((i+1)); echo "tick $i $(date)"; sleep 5; done'

cardinal ps
cardinal logs --tail 20 cardinal-test
cardinal logs -f cardinal-test
```

Нажмите `Ctrl+C`, чтобы выйти из просмотра логов; контейнер при этом не остановится.

```bash
cardinal stop cardinal-test
cardinal ps -a
cardinal rm cardinal-test
```

## 3. Проверка перезапуска после сбоя

Этот процесс завершается через три секунды. cardinal должен запустить его снова после заданной задержки.

```bash
cardinal run -d \
  -n restart-test \
  --restart always \
  --restart-delay 10s \
  alpine:latest \
  sh -c 'echo process-started; sleep 3; exit 1'

sleep 15
cardinal ps -a
cardinal logs --all restart-test
```

После проверки удалите контейнер:

```bash
cardinal rm -f restart-test
```

Если контейнер должен оставаться остановленным после ручного `cardinal stop`, используйте `unless-stopped`, а не `always`.

## 4. Переменные окружения и `.env`

```bash
mkdir -p /data/env-test
cat > /data/env-test/.env <<'EOF'
APP_ENV=production
APP_NAME=cardinal-demo
EOF

cardinal run --rm \
  --env-file /data/env-test/.env \
  alpine:latest \
  sh -c 'printf "%s/%s\\n" "$APP_NAME" "$APP_ENV"'
```

`-e KEY=VALUE` можно повторять и использовать для небольшого числа значений. Секреты registry и приложения не стоит помещать прямо в историю shell; используйте защищённый env-файл или отдельное хранилище секретов.

## 5. Именованные volumes

Именованные volumes хранятся в cardinal и переживают удаление контейнера.

```bash
cardinal volume create demo-data
cardinal run --rm --vol demo-data:/data alpine:latest \
  sh -c 'echo saved > /data/message.txt'
cardinal run --rm --vol demo-data:/data alpine:latest \
  cat /data/message.txt
cardinal volume inspect demo-data
cardinal volume rm demo-data
```

Bind mount подключает существующий каталог хоста:

```bash
mkdir -p /data/demo-app
cardinal run --rm -v /data/demo-app:/app alpine:latest \
  sh -c 'echo host-visible > /app/message.txt'
cat /data/demo-app/message.txt
```

## 6. Web-сервер

```bash
mkdir -p /data/site
printf '<h1>Hello from cardinal</h1>\n' > /data/site/index.html

cardinal run -d \
  -n web \
  -p 8080:80 \
  --restart unless-stopped \
  --vol /data/site:/usr/share/nginx/html \
  nginx:alpine

curl http://127.0.0.1:8080
cardinal port web
cardinal logs --tail 50 web
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
    return 'Hello from cardinal\n'

app.run(host='0.0.0.0', port=5000)
EOF

cardinal run -d \
  -n flask \
  -p 5000:5000 \
  --restart unless-stopped \
  --env-file "$PWD/.env" \
  --vol "$PWD:/app" \
  --workdir /app \
  python:3.12 \
  sh -c 'python -m pip install --no-cache-dir -r requirements.txt && exec python app.py'

curl http://127.0.0.1:5000
cardinal logs -f flask
```

Для Discord или Telegram-бота замените `app.py` и `requirements.txt`, передайте token через `--env-file` и оставьте политику `--restart unless-stopped`.

## 8. PostgreSQL, MySQL и Redis

Для данных баз используйте именованные volumes. Примеры предполагают стандартные entrypoints образов.

### PostgreSQL

```bash
cardinal run -d \
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
cardinal run -d \
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
cardinal run -d \
  -n redis \
  -p 6379:6379 \
  --restart unless-stopped \
  --vol redis-data:/data \
  redis:7 redis-server --appendonly yes
```

Проверка состояния и логов:

```bash
cardinal ps
cardinal logs --tail 100 postgres
cardinal exec postgres pg_isready -U app -d app
cardinal exec redis redis-cli ping
```

## 9. Minecraft Java через `$PWD`

`$PWD` удобен только внутри разрешённого каталога хоста. Не запускайте это из `/root/test`; сначала скопируйте сервер в `/data/minecraft`.

```bash
mkdir -p /data/minecraft
# Скопируйте server.jar и существующие миры/plugins в /data/minecraft.
cd /data/minecraft
ls -lh server.jar
printf 'eula=true\n' > eula.txt

cardinal run -d \
  -n minecraft \
  -h minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --restart-delay 1m \
  --vol "$PWD:/data" \
  --workdir /data \
  --network host \
  --memory 4g \
  --cpus 4 \
  eclipse-temurin:21 \
  java -Xms1G -Xmx4G -jar server.jar nogui
```

Проверка Minecraft:

```bash
cardinal ps -a
cardinal logs --tail 100 minecraft
cardinal logs -f minecraft
ss -ltnp | grep 25565
```

Ищите в логах `Done (...)! For help, type "help"`. Подключение: `PUBLIC_IP_VPS:25565`. Файлы Minecraft в `/data` находятся на bind mount и не входят в backup контейнера cardinal; каталог `/data/minecraft` нужно архивировать отдельно.

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

cardinal run -d \
  -n minecraft \
  -p 25565:25565 \
  --restart unless-stopped \
  --restart-delay 1m \
  --vol "$PWD:/data" \
  --workdir /data \
  eclipse-temurin:21 \
  ./start.sh
```

Не используйте Paper-команду `/restart`, если host-side `start.sh` отсутствует. Пусть cardinal перезапускает главный процесс. Если сервер завершится, `--restart-delay 1m` восстановит его через минуту.

## 11. Backup

Automatic backup архивирует writable overlay и именованные volumes, но не host bind mounts. Полное руководство по бэкапам — автоматические, ручные, восстановление, скачивание на ПК, обход bind-монтов, граничные случаи и лучшие практики — см. в [Руководстве по бэкапам](backups.md).

```bash
cardinal run -d \
  -n backup-test \
  --restart unless-stopped \
  --vol backup-data:/data \
  alpine:latest \
  sh -c 'echo backup-data > /data/test.txt; sleep 3600'

cardinal bootstrap --install
cardinal backup enable backup-test --interval 1h --retention 7
cardinal backup status backup-test
cardinal backup list
```

Разовый backup остановленного контейнера:

```bash
cardinal stop backup-test
cardinal backup create backup-test -o /data/backups/backup-test/manual.tar.gz
cardinal backup restore backup-test /data/backups/backup-test/manual.tar.gz
cardinal start backup-test
```

Проверка архива по контрольной сумме (если checksum-файла рядом нет, cardinal сообщит, что архив не проверен):

```bash
cardinal backup verify /data/backups/backup-test/manual.tar.gz
```

Очистка:

```bash
cardinal backup disable backup-test
cardinal rm -f backup-test
cardinal volume rm backup-data
```

## 12. Inspect, exec, copy и filesystem

```bash
cardinal inspect web
cardinal inspect --sensitive web
cardinal exec -i -t web /bin/sh
cardinal console web
cardinal top web
cardinal stats --no-stream web
cardinal fs ls web /
cardinal fs tree web /etc/nginx
cardinal fs find web /etc --name '.conf' --type f
cardinal fs cat web /etc/hostname
cardinal cp ./local.conf web:/etc/app/config.conf
cardinal cp web:/etc/app/config.conf /data/config-backup/
```

## 13. Динамические порты

```bash
cardinal port web
cardinal port add web 8081:80
cardinal port add web 5353:53/udp
cardinal port remove web 8081
cardinal port rm web 5353/udp
```

## 14. Multi-container stack через `cardinal.toml`

```bash
mkdir -p /data/stack
cd /data/stack
cat > cardinal.toml <<'EOF'
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

cardinal up -f cardinal.toml
cardinal ps
cardinal down -f cardinal.toml
```

Создать исходную конфигурацию из существующих именованных контейнеров:

```bash
cardinal up --generate -f generated.toml
```

## 15. Registry, перенос и build

```bash
cardinal login registry.example.com
cardinal build -t registry.example.com/team/app:1.0 .
cardinal push registry.example.com/team/app:1.0
cardinal export registry.example.com/team/app:1.0 -o /data/app-1.0.tar.gz
cardinal import /data/app-1.0.tar.gz
cardinal verify registry.example.com/team/app:1.0
cardinal logout registry.example.com
```

## 16. Cluster, services и functions

Инициализация одновузлового cluster:

```bash
export CARDINAL_TOKEN='replace-with-a-long-random-token'
cardinal cluster init --name production --bind 10.0.2.1 --port 7946 --api-port 2375 --token "$CARDINAL_TOKEN"
cardinal cluster info
cardinal cluster node ls
cardinal cluster join-token
```

Replicated service:

```bash
cardinal service create --name api --replicas 3 --port 8080:80 nginx:alpine
cardinal service ls
cardinal service scale api 5
cardinal service update api --image nginx:1.27-alpine
cardinal service rm api
```

Serverless function:

```bash
# Замените образ на свой function image с HTTP handler /handler на порту 8080.
cardinal fn deploy --name hello --port 8080 --timeout 30 --idle 300 --warm 1 registry.example.com/team/hello-function:latest
cardinal fn ls
cardinal fn call hello --data '{"name":"cardinal"}'
cardinal fn rm hello
```

## 17. Запуск через `--image` / `--cmd` / `--workdir`

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

### Minecraft Paper:

```bash
cardinal run -d --restart always \
  -n mc-paper -p 25565:25565 \
  -v $PWD:/data --memory 4G --cpus 4 \
  -workdir /data \
  -image eclipse-temurin:21-jdk \
  -cmd "java -Xmx3500M -jar paper-1.21.11-116.jar nogui"
```

### Discord-бот:

```bash
cardinal run -d --restart always \
  -n discord-bot \
  -v /data/bot:/bot --workdir /bot \
  -e BOT_TOKEN=your_token \
  -image python:3.12 \
  -cmd "sh -c 'pip install -r /bot/requirements.txt && exec python /bot/bot.py'"
```

### Telegram-бот:

```bash
cardinal run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot --workdir /bot \
  -e BOT_TOKEN=your_token \
  -image python:3.12 \
  -cmd "sh -c 'pip install -r /bot/requirements.txt && exec python /bot/bot.py'"
```

### PostgreSQL:

```bash
cardinal run -d --restart always \
  -n postgres -p 5432:5432 \
  -v pg_data:/var/lib/postgresql/data \
  -e POSTGRES_DB=myapp -e POSTGRES_PASSWORD=secret \
  -image postgres:16
```

### Redis:

```bash
cardinal run -d --restart always \
  -n redis -p 6379:6379 \
  -v redis_data:/data \
  -image redis:7 \
  -cmd "redis-server --appendonly yes"
```

### Nginx (статический сайт):

```bash
cardinal run -d --restart always \
  -n web -p 8080:80 \
  -v /data/site:/usr/share/nginx/html \
  -network host \
  -image nginx:alpine
```

> **Сетевой доступ:** если приложению нужен интернет (DNS), добавьте `-network host`. Без этого bridge-контейнеры не резолвят внешние хосты.

## 18. Обслуживание и recovery после reboot

```bash
cardinal info
cardinal events --since "2026-01-01T00:00:00Z"
cardinal system prune
cardinal update --check
cardinal bootstrap --install
systemctl status cardinal-bootstrap
journalctl -u cardinal-bootstrap -f
```

Используйте `cardinal bootstrap --remove`, только если больше не нужны запуск после reboot и scheduled backups.

## 18. Чек-лист очистки

```bash
cardinal ps -a
cardinal stop minecraft
# Перед удалением host directory отдельно сохраните bind-mounted файлы.
cardinal rm -f minecraft
cardinal volume ls
cardinal volume prune
```

Полный синтаксис каждой команды — в [Справочнике CLI](commands.md), установка и troubleshooting — в [Руководстве по запуску](running.md).
