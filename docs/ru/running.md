# Запуск контейнеров в dck

Это практическое руководство по ежедневной работе: установка dck, загрузка образа, запуск приложения, подключение постоянных файлов, `.env`, просмотр логов, обновление кода и решение типичных ошибок.

> dck запускает контейнеры на Linux. Команды ниже рассчитаны на Bash и выполняются от `root` либо с необходимыми привилегиями.

## 1. Требования

На Linux-хосте должны быть доступны:

- `unshare`, `nsenter`, `mount`, `ip`, `iptables`, `pgrep`;
- PID-, mount-, UTS-, IPC- и network-пространства имён;
- OverlayFS;
- cgroups v2 для лимитов ресурсов;
- `curl` для установки и операций с registry.

Проверка перед установкой:

```bash
command -v unshare nsenter mount ip iptables pgrep curl
grep overlay /proc/filesystems
uname -a
```

## 2. Установка и обновление dck

Установка через APT на Debian/Ubuntu:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/scripts/install-apt.sh | sudo bash
```

Универсальная установка для Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

Проверка:

```bash
dck version
dck info
```

Обновление установленной версии:

```bash
dck update --check
dck update
```

После обновления проверьте бинарник и запустите временный контейнер:

```bash
dck version
dck run --rm alpine:latest echo "DCK UPDATE OK"
```

## 3. Загрузка и запуск образа

Образ можно передать позиционно. Флаг `--image` тоже поддерживается, а `--images` — нет.

```bash
dck pull alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

Тег указывается через двоеточие. Это разные ссылки:

```text
python3.12                         репозиторий library/python3.12
python:3.12                        образ python, тег 3.12
nanozoo/python3.12:3.12--d46ab4d  репозиторий и явный тег
```

Если у репозитория нет тега `latest`, укажите тег из результата поиска:

```bash
dck search nanozoo/python3.12
dck pull nanozoo/python3.12:3.12--d46ab4d
```

## 4. Запуск постоянного сервиса

Общий вид команды:

`--restart-delay` принимает значения длительности: `10s`, `30s` или `1m`.

```bash
dck run -d \
  -n ИМЯ_ПРИЛОЖЕНИЯ \
  -p ПОРТ_ХОСТА:ПОРТ_КОНТЕЙНЕРА \
  --restart unless-stopped \
  --restart-delay 1m \
  ОБРАЗ[:ТЕГ] \
  КОМАНДА [АРГУМЕНТЫ...]
```

Пример веб-сервера:

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

Доступные политики перезапуска: `no`, `always`, `on-failure`, `unless-stopped`. Добавьте `--restart-delay 1m` (или короткое значение `10s`), чтобы задать задержку перед автоматическим запуском после неожиданного завершения процесса. Намеренный `dck stop` эту задержку не отменяет и контейнер не запускает.

## 5. Bind mount и именованные тома

Формат volume: `источник:путь_в_контейнере`. Путь внутри контейнера должен быть абсолютным:

```bash
--vol /data/myapp:/app
--vol myapp_data:/var/lib/myapp
```

Создание именованного тома:

```bash
dck volume create app-data
dck volume ls
dck volume inspect app-data
```

Для кода приложения используйте отдельный каталог вне защищённых системных путей:

```bash
mkdir -p /data/myapp
cp -a /путь/к/myapp/. /data/myapp/
dck run -d \
  -n myapp \
  --vol /data/myapp:/app \
  --workdir /app \
  --restart unless-stopped \
  ОБРАЗ[:ТЕГ] КОМАНДА
```

В целях безопасности dck запрещает bind source, которые указывают на чувствительные каталоги хоста: `/`, `/root`, `/etc`, `/proc`, `/sys` и другие системные пути. Поэтому каталог вроде `/root/myapp` может потребоваться перенести в `/data/myapp` или другой отдельный каталог.

> Исходный каталог на хосте должен существовать до `dck run`. Команда `--vol "$PWD:/app"` корректна только тогда, когда текущий каталог разрешён для bind mount.

## 6. Переменные окружения и `.env`

Передача отдельных переменных:

```bash
dck run -d -e APP_ENV=production -e PORT=8080 ОБРАЗ[:ТЕГ] КОМАНДА
```

Или файл с записями `KEY=VALUE`:

```bash
cat > .env <<'EOF'
APP_ENV=production
BOT_TOKEN=replace_me
EOF

chmod 600 .env
dck run -d --env-file .env ОБРАЗ[:ТЕГ] КОМАНДА
```

Не добавляйте `.env` в Git и не выводите секреты в публичные логи. Для проекта `/data/mybot`:

```bash
dck run -d \
  -n mybot \
  --env-file /data/mybot/.env \
  --vol /data/mybot:/bot \
  --workdir /bot \
  ОБРАЗ[:ТЕГ] КОМАНДА
```

## 7. Python-бот: полный пример

Пусть в `/data/alfheimguide` находятся `main.py`, `requirements.txt` и `.env`:

```bash
cd /data/alfheimguide
cp .env.example .env
chmod 600 .env
# Откройте .env и укажите нужный токен и настройки.
```

Запуск в фоне:

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

Проверка:

```bash
dck ps -a
dck logs --tail 100 alfheimguide
dck logs -f alfheimguide
```

Если установка зависимостей выполняется при каждом рестарте и это неудобно, установите их один раз в overlay контейнера или соберите отдельный образ через Dockerfile. Файлы проекта в bind mount остаются на хосте и переживают удаление контейнера.

## 8. Java- или Minecraft-сервер

Для собственного Java JAR в `/data/minecraft` положите туда JAR и данные сервера:

```bash
mkdir -p /data/minecraft
ls -lh /data/minecraft/server.jar
```

Запуск Eclipse Temurin Java 21:

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

Сервер должен слушать `0.0.0.0:25565`, а не только `127.0.0.1`.

Проверка:

```bash
dck ps -a
dck logs --tail 100 minecraft
ss -ltnp | grep 25565
```

Вариант с именованным томом:

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

## 9. Жизненный цикл контейнера

```bash
dck ps                 # работающие контейнеры
dck ps -a              # работающие и остановленные
dck stop ИМЯ           # остановить без удаления данных
dck start ИМЯ          # запустить существующий остановленный
dck restart ИМЯ       # остановить и запустить
dck rm ИМЯ             # удалить остановленный
dck rm -f ИМЯ          # принудительно удалить
```

`dck stop` сохраняет overlay контейнера и данные bind mount. `dck rm -f` удаляет overlay и внутренние state/log-файлы dck. Файлы bind mount и данные именованных томов при удалении контейнера не удаляются.

## 10. Логи и attach

Для root-установки dck лог stdout/stderr контейнера хранится здесь:

```text
/root/.dck/logs/<container-id>.log
```

Корень данных можно изменить через `DCK_DATA_DIR`:

```bash
export DCK_DATA_DIR=/data/dck-state
dck info
```

Обычные команды работы с логами:

```bash
dck logs ИМЯ
dck logs --tail 100 ИМЯ
dck logs -f ИМЯ
dck attach ИМЯ
```

`dck logs -f` показывает новые строки. `dck attach` подключается к главному процессу detached-контейнера. Для выхода из attach нажмите `Ctrl+C` — контейнер не остановится.

При новом запуске dck создаёт свежий stdout/stderr лог, поэтому вывод прошлых запусков не накапливается после `stop`/`start` или `restart`. Логи самого приложения — отдельная вещь: например, Minecraft `/data/logs/latest.log` хранится в bind mount или именованном томе и сохраняется между запусками.

## 11. Автоматические бэкапы

Для каждого контейнера можно включить собственное расписание. Архив содержит writable overlay и именованные тома. Чтобы данные были согласованными, dck ненадолго остановит работающий контейнер, создаст архив и запустит его снова.

```bash
dck backup enable minecraft --interval 6h --retention 14
dck backup status minecraft
dck backup list
dck backup disable minecraft
```

По умолчанию архивы сохраняются в `$DCK_DATA_DIR/backups/<container>/`. При необходимости укажите отдельный каталог:

```bash
dck backup enable minecraft \
  --interval 24h \
  --retention 7 \
  --dir /data/backups/minecraft
```

Один раз установите постоянный supervisor, чтобы расписание продолжало работать после выхода из терминала и после перезагрузки:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

Разовый бэкап можно создать командой `dck backup create ИМЯ`. Восстанавливать архив можно только в остановленный контейнер:

```bash
dck backup restore minecraft /data/backups/minecraft/minecraft-20260811-120000.tar.gz
```

Настройки авто-бэкапа хранятся в state контейнера и переживают `stop`, `start` и обновление dck. Retention удаляет самые старые архивы расписания после успешного бэкапа; файлы с другими именами он не трогает.

## 12. Проверка и команды внутри контейнера

```bash
dck exec ИМЯ команда аргументы...
dck exec -i -t ИМЯ /bin/sh
dck console ИМЯ
dck top ИМЯ
dck port ИМЯ
dck stats ИМЯ --no-stream
dck fs ls ИМЯ /путь
dck fs cat ИМЯ /путь/файла
dck cp ./локальный-файл ИМЯ:/путь/
dck cp ИМЯ:/путь/файла ./локальный-файл
```

`attach` подключается к существующему главному процессу; `exec` запускает новый процесс. Для shell используйте `console` или `exec -i -t`.

## 13. Лимиты ресурсов и безопасность

```bash
dck run -d --memory 512m --cpus 1 --disk 5G ОБРАЗ[:ТЕГ] КОМАНДА
dck run -d --user 1000:1000 --cap-drop ALL --no-new-privs ОБРАЗ[:ТЕГ] КОМАНДА
dck run -d --readonly ОБРАЗ[:ТЕГ] КОМАНДА
```

Добавляйте только требуемые приложению capabilities:

```bash
--cap-add NET_ADMIN
```

Используйте `--network none`, если приложению не нужна сеть, и `--network host` только осознанно — этот режим делит сетевое пространство хоста.

## 14. Обновление кода приложения

При bind mount измените файл на хосте и перезапустите контейнер:

```bash
nano /data/alfheimguide/main.py
dck restart alfheimguide
dck logs --tail 100 alfheimguide
```

Если добавили зависимость, обновите `requirements.txt` и перезапустите контейнер, если startup-команда устанавливает зависимости. Для production лучше собрать образ, чем устанавливать пакеты при каждом запуске.

## 15. Практические рецепты: боты, базы данных и игровые серверы

Все примеры используют одну и ту же форму команды:

```bash
dck run -d \
  -n ИМЯ \
  -p ПОРТ_ХОСТА:ПОРТ_КОНТЕЙНЕРА \
  --restart ПОЛИТИКА \
  --restart-delay 1m \
  --env-file "$PWD/.env" \
  --vol "$PWD:/app" \
  --workdir /app \
  ОБРАЗ[:ТЕГ] КОМАНДА
```

Все флаги dck размещайте до имени образа. Всё после образа и команды передаётся процессу внутри контейнера. Например, `-p 23323:23332` должен находиться до `python:3.12`, а не после `sh -c`.

### Политики перезапуска

| Политика | Процесс неожиданно завершился | `dck stop` | Перезагрузка хоста |
|---|---|---|---|
| `no` | Останется остановленным | Останется остановленным | Не запускается |
| `on-failure` | Перезапуск только при ненулевом коде, пока жив monitor; после выхода detached CLI supervisor не подхватывает контейнер | Останется остановленным | Не запускается автоматически |
| `always` | Перезапуск | Явно остановленный контейнер не запускается этим действием | Запускается автоматически |
| `unless-stopped` | Перезапуск | Останется остановленным до явного `dck start` | Запускается автоматически, если не был остановлен вручную |

`dck run --restart always` и `dck run --restart unless-stopped`, запущенные от root, автоматически устанавливают systemd-службу bootstrap. `--restart-delay` влияет на восстановление после завершения процесса и не задерживает первоначальный запуск после reboot. Установить bootstrap вручную можно так:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

Служба bootstrap запускает подходящие контейнеры после загрузки хоста. Политика `on-failure` не означает автозапуск после reboot. Для постоянного detached-сервиса используйте `always` или `unless-stopped`; `on-failure` надёжна только пока жив процесс, владеющий monitor.

### Python-бот с `.env`, volume, портом и автоматическим восстановлением

```bash
mkdir -p /data/bot
cd /data/bot
cp .env.example .env
chmod 600 .env
# Откройте .env и задайте BOT_TOKEN и другие секреты.

# Перезапуск после сбоя и запуск после перезагрузки.
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

Вариант без автоматического перезапуска и автозапуска после reboot — просто не указывайте `--restart`:

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

Для данных базы используйте именованный том, а не overlay контейнера:

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

Вариант без перезапуска и автозапуска:

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

### Minecraft Java-сервер

Собственный JAR и все сохранения можно хранить в bind mount:

```bash
mkdir -p /data/minecraft
# Скопируйте server.jar, eula.txt, server.properties и миры в /data/minecraft.

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

Вариант Minecraft без автоперезапуска:

```bash
dck run -d \
  -n minecraft-manual \
  -p 25566:25565 \
  --vol /data/minecraft:/data \
  --workdir /data \
  eclipse-temurin:21 \
  java -Xms1G -Xmx4G -jar server.jar nogui
```

Minecraft должен слушать `0.0.0.0:25565`. Его собственные логи и миры сохраняются в `/data/minecraft`, а stdout/stderr dck очищается при каждом новом запуске контейнера.

### Terraria

У разных образов Terraria отличаются переменные окружения и пути данных. Перед production-пуском проверьте документацию выбранного образа:

```bash
dck volume create terraria-data
dck run -d \
  -n terraria \
  -p 7777:7777 \
  --restart unless-stopped \
  --vol terraria-data:/config \
  terraria-server-image:latest
```

Без автоматического перезапуска:

```bash
dck run -d \
  -n terraria-manual \
  -p 7778:7777 \
  --vol terraria-data:/config \
  terraria-server-image:latest
```

Замените `terraria-server-image:latest` на выбранный образ и настройте его EULA/переменные.

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

### Source- или другой dedicated game server

Используйте внутренний порт и каталог данных из документации конкретного образа:

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

Для ручного запуска удалите `--restart always`. Если нужен перезапуск через минуту после сбоя, добавьте `--restart-delay 1m`. Всегда проверьте EULA, порты, команду запуска, каталог сохранений и необходимые переменные окружения выбранного образа.

### Проверка, остановка и восстановление

```bash
dck ps -a
dck logs --tail 100 minecraft
dck stats minecraft --no-stream
dck stop minecraft
dck start minecraft
dck restart minecraft
```

Сбой процесса обрабатывается настроенной политикой перезапуска. Ручной `dck stop` считается намеренной остановкой и не даёт `unless-stopped` запуститься снова до `dck start`. Чтобы полностью отключить автоматическое восстановление:

```bash
dck stop bot
dck set bot --restart no
dck start bot
```

## 16. Решение проблем

### `flag provided but not defined: -images`

Используйте `--image` или передайте образ позиционно:

```bash
dck run --image python:3.12 python --version
dck run python:3.12 python --version
```

### `container mount target must be absolute`

Путь внутри контейнера должен начинаться с `/`:

```bash
--vol /data/app:/app
```

Не `--vol /data/app:app`.

### `resolve bind source ... no such file or directory`

Сначала создайте каталог источника:

```bash
mkdir -p /data/app
```

### `bind source ... is a protected host path`

Перенесите проект из защищённого системного каталога:

```bash
mkdir -p /data/app
cp -a /root/app/. /data/app/
```

### После `\` появляются `-n: command not found`

В Bash обратный слэш должен быть последним символом строки. Нельзя оставлять пустые строки после `\`. При копировании через неудобный терминал используйте одну строку:

```bash
dck run -d -n app --vol /data/app:/app --workdir /app ОБРАЗ[:ТЕГ] КОМАНДА
```

### Контейнер имеет статус `created`, но не `running`

Посмотрите лог и удалите неудачно созданный контейнер перед повтором:

```bash
dck ps -a
dck logs ИМЯ
dck rm -f ИМЯ
```

### Приложение работает, но недоступно

Проверьте, что приложение слушает `0.0.0.0`, mapping порта указан правильно, а firewall хоста разрешает порт:

```bash
dck port ИМЯ
ss -ltnp
dck logs ИМЯ
```

## 17. Где хранятся данные

Для root стандартный каталог данных dck — `/root/.dck`:

```text
/root/.dck/
├── images/       скачанные rootfs образов
├── containers/   JSON-состояние контейнеров
├── overlay/      записываемые слои контейнеров
├── logs/         stdout/stderr dck
├── volumes/      именованные тома
├── cache/        кэш слоёв образов
└── consoles/     socket-файлы для attach
    backups/      архивы автоматических бэкапов
```

Чтобы использовать другое место для внутреннего состояния, задайте `DCK_DATA_DIR` до запуска dck. Bind mount проекта, например `/data/alfheimguide`, хранится отдельно от этого внутреннего каталога.
