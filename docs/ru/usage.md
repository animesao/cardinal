<!-- cardinal-version:start -->
**Documentation version:** `2.0.4`
**Project release:** `v2.0.4`
<!-- cardinal-version:end -->

# Команды и использование

cardinal — лёгкий container runtime. Нет демона, нет Docker. Просто контейнеры.
~5 MB статический бинарник, OCI образы, bridge-сеть, кластеризация, FaaS.

> Полный список команд, алиасов, префиксов и флагов находится в [полном справочнике CLI](commands.md). Практические рецепты — в [примерах команд](examples.md).

---

## Содержание

- [Руководство по запуску](running.md)
- [Развёртывание сайтов](websites.md)
- [Управление образами](#управление-образами)
  - [cardinal pull](#cardinal-pull---platform-osarch-образтег)
  - [cardinal search](#cardinal-search-термин)
  - [cardinal images](#cardinal-images)
  - [cardinal rmi](#cardinal-rmi-образтег)
  - [cardinal export](#cardinal-export-образ--o-файлtargz)
  - [cardinal import](#cardinal-import-файлtargz)
  - [cardinal build](#cardinal-build--t-имятег-опции-)
  - [cardinal commit](#cardinal-commit-контейнер-образтег)
  - [cardinal push](#cardinal-push-образтег)
  - [cardinal login / cardinal logout](#cardinal-login-registry--cardinal-logout-registry)
- [Жизненный цикл контейнера](#жизненный-цикл-контейнера)
- [Запуск контейнеров (`cardinal run`)](#cardinal-run)
- [Работа с контейнерами](#работа-с-контейнерами)
- [Exec & Attach](#exec--attach)
- [Логи и мониторинг](#логи-и-мониторинг)
- [Сеть](#сеть)
- [Просмотр файлов](#просмотр-файлов--cardinal-fs)
- [Хранилище и тома](#хранилище-и-тома)
- [Лимиты ресурсов](#лимиты-ресурсов)
- [Безопасность](#безопасность)
- [Переменные окружения](#переменные-окружения)
- [Проверки здоровья (Healthchecks)](#проверки-здоровья-healthchecks)
- [Стартовые скрипты](#стартовые-скрипты)
- [Проброс портов](#проброс-портов)
- [Динамическое управление портами](#динамическое-управление-портами)
- [cardinal.toml / Compose](#cardinaltoml--compose)
- [cardinal up / cardinal down](#cardinal-up--cardinal-down)
- [Кластеризация](#кластеризация)
- [Управление сервисами](#управление-сервисами)
- [FaaS / Serverless](#faas--serverless)
- [Блюпринты](#блюпринты)
- [Сборка и экспорт образов](#сборка-и-экспорт-образов)
- [Регистры](#регистры)
- [Системные команды](#системные-команды)
- [События](#события)
- [Архитектура](#архитектура)
- [Решение проблем](#решение-проблем)

---

## Управление образами

### `cardinal pull [--platform os/arch] <образ>[:тег]`

Скачать образ из registry (по умолчанию Docker Hub).

```bash
cardinal pull nginx
cardinal pull alpine:3.19
cardinal pull --platform linux/arm64 eclipse-temurin:21-jre
cardinal pull registry.example.com/myapp:v1.0
```

Приватные registry: `DOCKER_USERNAME` / `DOCKER_PASSWORD`, или `-u user -p pass` на push.

### `cardinal images`

Список локально сохранённых образов.

```bash
cardinal images
```

### `cardinal search <термин>`

Поиск образов на Docker Hub.

```bash
cardinal search nginx
cardinal search python
cardinal search alpine
cardinal search python:3.11          # фильтр по тегу
```

Показывает имя, описание, звёзды, загрузки и доступные теги. Можно фильтровать теги через `образ:тег`.

### `cardinal rmi <образ>[:тег]`

Удалить образ.

```bash
cardinal rmi nginx:alpine
```

### `cardinal verify <образ>[:тег]`

Проверить config и диджесты слоёв локального образа.

```bash
cardinal verify nginx:alpine
```

### `cardinal export <образ> -o <файл.tar.gz>`

Экспортировать образ в tar.gz (для бэкапа или переноса).

```bash
cardinal export myapp:v1 -o myapp-v1.tar.gz
```

### `cardinal import <файл.tar.gz>`

Импортировать образ из tar.gz.

```bash
cardinal import myapp-v1.tar.gz
```

### `cardinal build -t <имя>:<тег> [опции] .`

Собрать образ из Dockerfile.

```bash
cardinal build -t myapp:v1 .
cardinal build -t myapp:v1 --build-arg VERSION=1.0 -f Dockerfile.prod .
```

**Поддерживаемые инструкции Dockerfile:**
FROM, RUN, COPY, ADD, WORKDIR, ENV, CMD, ENTRYPOINT, EXPOSE, LABEL, USER,
VOLUME, SHELL, ARG, HEALTHCHECK, STOPSIGNAL, ONBUILD.

**Возможности:**
- ✅ Многоэтапная сборка (`FROM ... AS alias`, `COPY --from=`)
- ✅ Подстановка ARG (`$VAR` / `${VAR}` во всех инструкциях)
- ✅ HEALTHCHECK с `--start-period`
- `--no-cache` принимается для совместимости CLI; сейчас cardinal всё равно выполняет каждую инструкцию Dockerfile и не использует кэш результатов инструкций

### `cardinal commit <контейнер> <образ>[:тег]`

Создать образ из текущего состояния контейнера (со всеми изменениями в overlay).

```bash
cardinal commit myproject myproject-snapshot:v1
```

Сохраняет всё, что вы установили (пакеты, файлы, конфиги) в переиспользуемый образ.

### `cardinal push <образ>[:тег]`

Отправить образ в registry.

```bash
cardinal push myapp:v1
cardinal push registry.example.com/myapp:v1
```

Авторизация: `-u user -p pass` или `DOCKER_USERNAME` / `DOCKER_PASSWORD`.

### `cardinal login <registry>` / `cardinal logout <registry>`

Войти/выйти из registry для авторизованных pull/push.

```bash
cardinal login registry.example.com
cardinal logout registry.example.com
```

---

## Жизненный цикл контейнера

### `cardinal ps`

Список контейнеров.

```bash
cardinal ps           # только запущенные
cardinal ps -a        # все (включая остановленные)
```

### `cardinal run [опции] <образ> [команда]`

Создать и запустить контейнер. Главная команда.

```bash
# Одноразовая команда
cardinal run --rm alpine echo "hello"

# Веб-сервер в фоне
cardinal run -d -n web -p 80:80 nginx:alpine

# Интерактивный shell
cardinal run -i -t --rm alpine sh

# С лимитами ресурсов
cardinal run -d --memory 512m --cpus 1.5 node:20 node app.js

# С томом и переменными
cardinal run -d -v /data:/data -e DB_URL=postgres://... myapp

# С длинными флагами и авто-перезапуском
cardinal run -d --name myapp --ports 8080:80 --volume /app:/app --restart always --image nginx:alpine
```

**Важно:** cardinal использует пакет `flag` из Go. Флаги можно передавать раздельно
(`-i -t`) или объединённой формой (`-it`, `-dit`) — сокращения нормализуются
автоматически перед парсингом:
- ✅ `cardinal run -i -t alpine sh`
- ✅ `cardinal run -it alpine sh` (сокращение, нормализуется в `-i -t`)

#### Флаги запуска

| Флаг | Описание | Пример |
|---|---|---|
| `-d` | Фоновый режим (detach) | `-d` |
| `-n <имя>` | Имя контейнера | `-n myapp` |
| `-p H:C[/proto]` | Проброс порта `хост:контейнер/tcp\|udp` | `-p 8080:80` |
| `--ports H:C` | Проброс порта (алиас `-p`) | `--ports 8080:80` |
| `-v S:D` | Монтирование тома `источник:назначение` | `-v /data:/data` |
| `--volume S:D` | Монтирование тома (алиас `-v`) | `--volume /data:/data` |
| `--vol S:D` | Монтирование тома (алиас `-v`) | `--vol myvol:/data` |
| `-e K=V` | Переменная окружения | `-e DB_HOST=localhost` |
| `--env-file <файл>` | Файл с переменными окружения | `--env-file .env` |
| `-i` | Интерактивный режим (держать stdin открытым) | `-i` |
| `-t` | Выделить TTY (псевдотерминал) | `-t` |
| `--rm` | Удалить контейнер при выходе | `--rm` |
| `--restart <политика>` | `no`, `always`, `on-failure`, `unless-stopped`; для detached-контейнеров после reboot supervisor обслуживает `always`/`unless-stopped` | `--restart always` |
| `--restart-delay <длительность>` | Задержка восстановления после сбоя, например `10s` или `1m`; не задерживает первоначальный запуск | `--restart-delay 1m` |
| `--restart-max-attempts <n>` | Бюджет crash-loop: остановить авто-перезапуск после N сбоев в течение окна (по умолчанию 5) | `--restart-max-attempts 5` |
| `--restart-window <длительность>` | Окно бюджета crash-loop | `--restart-window 10m` |
| `--memory <лимит>` | Лимит памяти | `--memory 2g` |
| `--ram <лимит>` | Лимит памяти (алиас `--memory`) | `--ram 1g` |
| `--cpus <число>` | Лимит CPU | `--cpus 1.5` |
| `--cpu <число>` | Лимит CPU (алиас `--cpus`) | `--cpu 2` |
| `--disk <лимит>` | Лимит диска (создаёт ext4 образ) | `--disk 10G` |
| `--workdir <дир>` | Рабочая директория внутри контейнера | `--workdir /app` |
| `-h <имя>` | Hostname контейнера | `-h myserver` |
| `--entrypoint <cmd>` | Переопределить entrypoint | `--entrypoint /bin/bash` |
| `--image <образ>` | Образ контейнера (вместо позиционного аргумента) | `--image nginx:alpine` |
| `--cmd <cmd>` | Команда контейнера (вместо позиционных аргументов) | `--cmd "python app.py"` |
| `--command <cmd>` | Команда контейнера (алиас `--cmd`) | `--command "java -jar server.jar"` |
| `--cap-add <cap>` | Добавить capability | `--cap-add NET_ADMIN` |
| `--cap-drop <cap>` | Убрать capability | `--cap-drop ALL` |
| `--user <uid>` | Запуск от UID или `UID:GID` | `--user 1000:1000` |
| `--readonly` | Read-only корневая ФС | `--readonly` |
| `--no-new-privs` | Запретить повышение привилегий | `--no-new-privs` |
| `--sysctl <k=v>` | Sysctl параметр | `--sysctl net.ipv4.ip_forward=1` |
| `--ulimit <опция>` | Ulimit: `name=soft:hard` | `--ulimit nofile=1024:2048` |
| `-l, --label <k=v>` | Метка контейнера | `-l env=prod` |
| `--dns <ip>` | DNS сервер (можно повторять) | `--dns 8.8.8.8` |
| `--network <режим>` | Сеть: `bridge` (по умолч.), `none`, `host` или имя пользовательской сети | `--network appnet` |
| `--startup <s>` | Стартовый скрипт (строка или `@файл`) | `--startup @setup.sh` |
| `--healthcheck-cmd <cmd>` | Команда проверки здоровья | `--healthcheck-cmd "curl -f http://localhost"` |
| `--healthcheck-interval <s>` | Интервал проверки (секунды) | `--healthcheck-interval 30` |
| `--healthcheck-retries <n>` | Количество попыток | `--healthcheck-retries 5` |
| `--healthcheck-timeout <s>` | Таймаут проверки (секунды) | `--healthcheck-timeout 10` |

### `cardinal stop <контейнер>`

Остановить контейнер (SIGTERM, затем SIGKILL).

```bash
cardinal stop web
cardinal stop --all         # остановить все работающие контейнеры
```

### `cardinal start <контейнер>`

Запустить остановленный контейнер. Все данные в overlay сохраняются.

```bash
cardinal start web
```

### `cardinal restart <контейнер>`

Перезапустить контейнер (stop + start).

```bash
cardinal restart web
```

### `cardinal rm [-f] <контейнер>`

Удалить контейнер. `-f` принудительно удаляет работающий.

```bash
cardinal rm web
cardinal rm -f web         # удалить даже если запущен
```

**Важно:** При удалении контейнера стирается его overlay-слой — все изменения (установленные пакеты, файлы) пропадают.

### `cardinal set <контейнер> [опции]`

Изменить параметры контейнера без удаления (overlay сохраняется). Останавливает, меняет JSON и запускает заново.

```bash
cardinal set mc --memory 4g --cpus 2
cardinal set mc --restart always
cardinal set mc -e DIFFICULTY=hard
cardinal set mc --workdir /data-mc
```

### `cardinal rename <контейнер> <новое-имя>`

Переименовать контейнер.

```bash
cardinal rename web web-new
```

### `cardinal port <контейнер>`

Показать проброс портов контейнера.

```bash
cardinal port web
```

### `cardinal port add <контейнер> <хост>:<контейнер>[/proto]`

Добавить проброс порта на работающий контейнер без перезапуска. Применяет iptables DNAT правила мгновенно. Порты сохраняются в состоянии контейнера и восстанавливаются после перезапуска.

```bash
cardinal port add web 8080:80
cardinal port add web 53:53/udp
```

### `cardinal port remove <контейнер> <хост>[/proto]`

Удалить проброс порта.

```bash
cardinal port remove web 8080
cardinal port remove web 53/udp
```

### `cardinal port rm <контейнер> <хост>[/proto]`

Алиас для `cardinal port remove`.

```bash
cardinal port rm web 8080
```

### `cardinal top <контейнер>`

Показать процессы внутри контейнера.

```bash
cardinal top web
```

---

## Exec & Attach

### `cardinal exec [-i] [-t] <контейнер> <команда>`

Выполнить команду внутри работающего контейнера.

```bash
# Неинтерактивная команда
cardinal exec web nginx -s reload

# Интерактивный shell с TTY
cardinal exec -i -t myproject sh

# Интерактивный Python
cardinal exec -i -t myproject python3
```

Создаёт **новый процесс** внутри контейнера. Входит в неймспейсы контейнера (PID, mount, network, IPC)
и запускает команду прямо в корневой ФС контейнера (chroot не нужен — корень уже установлен через pivot_root).

### `cardinal attach <контейнер>`

Подключиться к **главному процессу** контейнера (работает только для контейнеров с `-d`).

```bash
cardinal run -d -i -t -n myproject alpine sh
cardinal attach myproject    # подключиться к sh
```

> **exec vs attach:** `attach` подключается к stdin/stdout главного процесса. `exec` запускает новую команду внутри контейнера. `console` — сокращение для `exec -i -t` с автоопределением shell.

`cardinal attach` **устойчив к Ctrl+C** — контейнер продолжает работать.

### `cardinal console <контейнер>`

Автоматически определить и запустить интерактивный shell внутри контейнера.
Эквивалент `cardinal exec -i -t <контейнер> sh`.

```bash
cardinal console myproject
```

---

## Логи и мониторинг

### `cardinal logs [-f] [--tail <n>] <контейнер>`

Показать stdout/stderr контейнера.

```bash
cardinal logs web            # вывод текущего запуска
cardinal logs -f web         # следить за новыми строками
cardinal logs --tail 20 web  # последние 20 строк
cardinal logs -f --tail 10 web  # последние 10 + следить
```

При каждом новом запуске контейнера cardinal создаёт свежий лог, поэтому вывод прошлых циклов `stop`/`start` или `restart` не дописывается бесконечно. Логи, которые создаёт само приложение (например, Minecraft `/data/logs/latest.log`), остаются в подключённом хранилище приложения.

Для root логи cardinal находятся в `/root/.cardinal/logs/<container-id>.log`; изменить каталог состояния можно через `CARDINAL_DATA_DIR`. Практические примеры находятся в [руководстве по запуску](running.md).

### `cardinal backup create|list|restore|enable|disable|status|verify`

Создавайте разовые архивы или включайте постоянное расписание для конкретного контейнера. Авто-бэкап включает writable overlay и именованные тома, но не host bind mount — каталоги приложения на хосте нужно архивировать отдельно. Для согласованности cardinal ненадолго останавливает работающий контейнер, создаёт архив и запускает его снова. Первый автоматический архив создаётся после заданного интервала, а не сразу.

```bash
cardinal backup enable minecraft --interval 6h --retention 14
cardinal backup status minecraft
cardinal backup list
cardinal backup disable minecraft

# Свой каталог (защищённые системные пути запрещены)
cardinal backup enable minecraft --interval 24h --retention 7 --dir /data/backups/minecraft
```

Чтобы расписание продолжало работать после выхода из CLI, установите systemd supervisor:

```bash
cardinal bootstrap --install
```

`cardinal backup create ИМЯ -o file.tar.gz` остаётся разовым ручным бэкапом. Восстановление выполняется только в остановленный контейнер: `cardinal backup restore ИМЯ file.tar.gz`. Архив можно проверить по контрольной сумме: `cardinal backup verify ФАЙЛ.tar.gz`; если checksum-файла рядом нет, cardinal сообщит, что архив не проверен.

### `cardinal stats [контейнер]`

Использование CPU, памяти, I/O и PIDs в реальном времени. Через cgroups v2.

```bash
cardinal stats               # все контейнеры
cardinal stats web           # конкретный
```

### `cardinal info`

Информация о системе: версия ядра, storage driver, директория данных, CPU, память, диск.

```bash
cardinal info
```

---

## Сеть

### Режимы сети

| Режим | Описание |
|---|---|
| `bridge` (по умолч.) | Каждый контейнер получает IP `10.0.2.X` на bridge `cardinal0`. Хост: `10.0.2.1`. |
| `none` | Без сети (только loopback) |
| `host` | Общая сеть с хостом (для VPN, сниффинга) |
| `<имя>` | Пользовательский Linux bridge, созданный через `cardinal network create` | `--network appnet` |

```bash
cardinal run -d -n web -p 80:80 nginx:alpine       # bridge (по умолч.)
cardinal run -d --network none alpine sleep infinity
cardinal run -d --network host myvpn-container

cardinal network create --subnet 10.20.0.0/24 appnet
cardinal network ls
cardinal run -d --network appnet -n app alpine sleep infinity
cardinal network inspect appnet
cardinal network rm appnet   # только после удаления контейнеров сети
```

### Схема сети

```
Хост:        cardinal0  10.0.2.1/24
Контейнер A: eth0  10.0.2.2
Контейнер B: eth0  10.0.2.3

A → хост:      ping 10.0.2.1      (хост — шлюз)
хост → A:      ping 10.0.2.2      (есть маршрут)
A → B:         ping 10.0.2.3      (через bridge)
A → порт B:    curl 10.0.2.1:8080 (DNAT: порт_хоста → порт_контейнера)
```

### Проброс портов

```bash
# TCP (по умолчанию)
-p 8080:80
-p 8080:80/tcp

# UDP
-p 53:53/udp

# Несколько портов
-p 80:80 -p 443:443
```

Проброс портов использует iptables DNAT с авто-настройкой UFW.

### Свой DNS

```bash
cardinal run -d --dns 1.1.1.1 --dns 8.8.8.8 nginx
```

---

## Динамическое управление портами

Позволяет добавлять и удалять пробросы портов на работающем контейнере без остановки.

```bash
# Добавить порт
cardinal port add web 8080:80

# Удалить порт
cardinal port remove web 8080

# Алиас для remove
cardinal port rm web 8080
```

Правила iptables DNAT применяются мгновенно. Порты сохраняются в состоянии контейнера (`~/.cardinal/containers/<id>.json`) и автоматически восстанавливаются при перезапуске контейнера (`cardinal start`).

---

## Хранилище и тома

### Синтаксис томов

```bash
# Bind mount (директория хоста)
-v /путь/на/хосте:/путь/в/контейнере
-v /путь/на/хосте:/путь/в/контейнере:ro     # только чтение
-v /путь/на/хосте:/путь/в/контейнере:shared # shared mount

# Именованный том (управляется cardinal)
-v myvolume:/путь/в/контейнере

# tmpfs (в памяти)
-v tmpfs:/путь/в/контейнере:size=1G,mode=0777

# NFS
-v nfs://сервер:/экспорт:/путь/в/контейнере:nfsopts=hard,intr
```

### Именованные тома

Тома хранятся в `~/.cardinal/volumes/`.

```bash
# Создать том
cardinal volume create mydata

# Список томов
cardinal volume ls

# Информация о томе
cardinal volume inspect mydata

# Удалить том
cardinal volume rm mydata

# Удалить неиспользуемые тома
cardinal volume prune
```

### Как работает хранилище

```
Хранилище: /root/.cardinal/

images/        OCI rootfs для каждого тега (только чтение)
containers/    JSON-файлы состояния
overlay/       upper/work/merged для каждого контейнера (слой записи)
volumes/       Именованные тома
logs/          stdout/stderr контейнера (новый файл при каждом запуске)
volumes/       Именованные тома
cache/         Кэш слоёв образов
consoles/      Unix сокеты для attach
backups/       Архивы автоматических бэкапов
```

**Overlay:** Каждый контейнер получает слой поверх read-only образа.
Изменения (установленные пакеты, файлы, правки) живут в overlay.
Они сохраняются между перезапусками (`cardinal stop` + `cardinal start`), но **удаляются**
при удалении контейнера (`cardinal rm`).

Чтобы сохранить изменения навсегда — используйте `cardinal commit`.

### Просмотр файлов — `cardinal fs`

Просмотр файлов контейнера без запуска shell. Работает на **запущенных** и **остановленных** контейнерах — overlay остаётся смонтированным после `stop`.

```bash
cardinal fs ls <контейнер> [путь]              # Список файлов
cardinal fs cat <контейнер> <путь>             # Содержимое файла
cardinal fs tree <контейнер> [путь]            # Дерево директорий
cardinal fs find [контейнер] [путь] [флаги]    # Поиск файлов
  --name <шаблон>     Фильтр по имени (подстрока, напр. "index")
  --grep <текст>      Поиск внутри файлов
  --type f|d          Только файлы или папки
  --max-depth <n>     Макс. глубина
```

Примеры:
```bash
cardinal fs ls web /etc/nginx
cardinal fs cat web /etc/nginx/conf.d/default.conf
cardinal fs tree mc-server /data --max-depth 2
cardinal fs find web --name "*.conf" --grep "server_name"
cardinal fs find --name "index"                              # искать во всех контейнерах
```

### Копирование файлов

```bash
# Из контейнера на хост
cardinal cp web:/etc/nginx/nginx.conf ./nginx.conf

# С хоста в контейнер
cardinal cp ./app.py web:/app/
```

---

## Лимиты ресурсов

### Память

```bash
cardinal run -d --memory 512m nginx    # 512 мегабайт
cardinal run -d --memory 1g nginx      # 1 гигабайт
cardinal run -d --memory 2g nginx      # 2 гигабайта
```

Через cgroups v2 memory controller. При превышении — OOM kill.

### CPU

```bash
cardinal run -d --cpus 1.5 nginx       # 1.5 ядра
cardinal run -d --cpus 2 nginx         # 2 ядра
```

Через CFS quota в cgroups v2.

### Диск

```bash
cardinal run -d --disk 1G nginx        # 1 GB
cardinal run -d --disk 10G nginx       # 10 GB
```

Создаёт sparse ext4 образ, который монтируется как overlay. Требует `mkfs.ext4`.

---

## Безопасность

### Пользователь

Запуск от непривилегированного пользователя:

```bash
cardinal run -d --user 1000 nginx
cardinal run -d --user 1000:1000 nginx   # UID:GID
```

### Capabilities

По умолчанию cardinal сохраняет безопасный Docker-совместимый набор capabilities, необходимый обычным образам (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `FSETID`, `KILL`, `SETGID`, `SETUID`, `SETPCAP`, `NET_BIND_SERVICE`, `NET_RAW`, `SYS_CHROOT`, `MKNOD`, `AUDIT_WRITE` и `SETFCAP`). Опасные capabilities, такие как `SYS_ADMIN` и `SYS_MODULE`, по-прежнему отключены. Поэтому стандартные образы вроде `nginx:alpine` могут нормально подготовить файловую систему.

```bash
# Добавить capability
cardinal run -d --cap-add NET_ADMIN nginx
cardinal run -d --cap-add NET_ADMIN --cap-add SYS_PTRACE nginx

# Отключить все capabilities (максимальное ограничение)
cardinal run -d --cap-drop ALL nginx

# Вернуть конкретные после --cap-drop ALL
cardinal run -d --cap-drop ALL --cap-add NET_BIND_SERVICE nginx
```

### Read-only rootfs

```bash
cardinal run -d --readonly nginx
```

Корневая ФС только для чтения. Запись в тома по-прежнему работает.

### Запрет привилегий

```bash
cardinal run -d --no-new-privs nginx
```

Запрещает получение новых привилегий (setuid, setgid, capability) всем процессам в контейнере.

### Sysctls

```bash
cardinal run -d --sysctl net.ipv4.ip_forward=1 nginx
```

### Профиль Seccomp

cardinal применяет профиль seccomp по умолчанию, который блокирует 30+ опасных syscalls, включая `mount`, `ptrace`, `reboot`, `kexec_load`, `bpf` и `init_module`.

```bash
# Использовать профиль seccomp по умолчанию (автоматически)
cardinal run -d nginx

# Использовать кастомный профиль seccomp
cardinal run -d --seccomp-profile /путь/к/профилю.json nginx
```

### Профиль AppArmor

cardinal применяет профиль AppArmor по умолчанию (`cardinal-container`), который ограничивает доступ к чувствительным путям хоста и ограничивает возможности контейнера.

```bash
# Использовать профиль AppArmor по умолчанию (автоматически)
cardinal run -d nginx

# Использовать кастомный профиль AppArmor
cardinal run -d --apparmor-profile мой-профиль nginx
```

### Сетевая изоляция

Изолировать контейнер от всех других контейнеров для предотвращения lateral movement:

```bash
cardinal run -d --isolated nginx

# Разрешить конкретную коммуникацию
cardinal run -d --isolated --network appnet nginx
```

### Аудит-логирование

Включить аудит-логирование для записи всех событий жизненного цикла контейнера:

```bash
cardinal run -d --audit-log nginx

# События логируются в ~/.cardinal/audit/audit-YYYY-MM-DD.log
cat ~/.cardinal/audit/audit-$(date +%Y-%m-%d).log
```

### Шифрование бэкапов

Шифровать архивы бэкапов с помощью AES-256-GCM:

```bash
# Сгенерировать ключ шифрования
cardinal backup generate-key

# Установить ключ через переменную окружения
export CARDINAL_BACKUP_KEY="ваш-hex-ключ"

# Создать зашифрованный бэкап
cardinal backup create nginx -e

# Создать зашифрованный бэкап с кастомным путём
cardinal backup create nginx -o /data/backups/nginx.enc -e
```

---

## Переменные окружения

```bash
# Одна переменная
cardinal run -e MY_VAR=value nginx

# Несколько
cardinal run -e DB_HOST=localhost -e DB_PORT=5432 nginx

# Из файла
cardinal run --env-file .env nginx
```

**Формат .env файла:**
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=admin
```

### Авто-внедрённые CARDINAL_* переменные

При запуске контейнера cardinal внедряет:

| Переменная | Описание |
|---|---|
| `CARDINAL_CONTAINER_ID` | ID контейнера |
| `CARDINAL_CONTAINER_NAME` | Имя контейнера |
| `CARDINAL_IMAGE_NAME` | Имя образа (например `library/alpine`) |
| `CARDINAL_IMAGE_TAG` | Тег образа (например `latest`) |
| `CARDINAL_HOSTNAME` | Hostname контейнера |
| `CARDINAL_MEMORY` | Лимит памяти в байтах |
| `CARDINAL_CPU` | Лимит CPU в ядрах |
| `CARDINAL_IP` | IP адрес контейнера |
| `CARDINAL_RESTART` | Политика рестарта |
| `CARDINAL_PORT_TCP_80` | Проброс портов |

Внутри контейнера доступны скрипты в `/cardinal/`:
- `/cardinal/info` — информация о контейнере
- `/cardinal/env` — переменные CARDINAL_*
- `/cardinal/help` — справка

---

## Проверки здоровья (Healthchecks)

Запускает команду внутри контейнера через заданный интервал. После `retries` неудач контейнер убивается и перезапускается.

```bash
cardinal run -d \
  --healthcheck-cmd "curl -f http://localhost || exit 1" \
  --healthcheck-interval 30 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  nginx
```

Healthchecks можно также задавать в compose-файлах и cardinal.toml.

---

## Стартовые скрипты

`--startup` запускает кастомный скрипт вместо команды из образа:

```bash
# Скрипт строкой
cardinal run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Из файла
cardinal run -d --startup @./myscript.sh ubuntu
```

Скрипт записывается в `/startup.sh` и выполняется через `/bin/sh`.
При наличии `--startup` он **заменяет** стандартный CMD/entrypoint.

---

## cardinal.toml / Compose

### Формат cardinal.toml

Определите контейнеры в TOML-файле, запускайте всё одной командой.

```toml
[container.web]
image = "nginx:alpine"
ports = ["80:80", "443:80"]
volumes = ["./html:/usr/share/nginx/html"]
restart = "always"

[container.db]
image = "postgres:16"
ports = ["5432:5432"]
env = { POSTGRES_PASSWORD = "secret", POSTGRES_DB = "myapp" }
volumes = ["pg_data:/var/lib/postgresql/data"]
restart = "always"
```

### compose.yaml / docker-compose.yaml

cardinal поддерживает стандартный формат Docker Compose YAML. Полная документация — в [compose.md](compose.md).

---

## cardinal up / cardinal down

### `cardinal up [имя] [-f <файл>]`

Создать и запустить контейнеры из compose-файла.

Автоопределение (по порядку):
1. `cardinal.toml`
2. `compose.yaml`
3. `compose.yml`
4. `docker-compose.yaml`
5. `docker-compose.yml`

`depends_on` учитывается — контейнеры запускаются в порядке зависимостей.
Поддерживаются `service_started` (по умолчанию), `service_healthy` (ждёт healthcheck),
`service_completed_successfully`.

```toml
[container.db]
image = "postgres:16"
healthcheck = { cmd = "pg_isready -U postgres", interval = 10, retries = 5 }

[container.app]
image = "myapp:latest"
depends_on = { db = "service_healthy" }
```

```bash
cardinal up                    # автоопределение
cardinal up myapp              # только сервис "myapp"
cardinal up -f compose.prod.yaml
cardinal up                    # запустить конфигурацию
cardinal up -f compose.prod.yaml
cardinal up myapp              # только сервис "myapp"
cardinal up --generate         # создать cardinal.toml из существующих контейнеров
```

### `cardinal down [имя] [-f <файл>]`

Остановить и удалить контейнеры из compose-файла.

```bash
cardinal down                  # stop + remove
cardinal down myapp            # только "myapp"
cardinal down -f cardinal.toml
cardinal down -a               # удалить ВСЕ контейнеры
# Для удаления всех контейнеров без чтения конфигурации:
cardinal down -a
```

---

## cardinal serve

Запустить Docker-совместимый REST API сервер.

```bash
cardinal serve -p 2375  # по умолчанию только localhost; для внешнего bind нужен --token
```

Совместим с Docker-клиентами, Portainer, VS Code Dev Containers и CI.

---

## Автозапуск при загрузке

Detached-контейнеры с `--restart always` или `--restart unless-stopped` запускаются автоматически после перезагрузки. Persistent supervisor не подхватывает `on-failure` после завершения короткого detached-процесса CLI.

cardinal сам устанавливает systemd-сервис когда:
- `cardinal run --restart always <образ>`
- `cardinal set <контейнер> --restart always`
- `cardinal up` (если в конфиге есть restart: "always")

Также можно управлять вручную:

```bash
cardinal bootstrap --install      # установить systemd-сервис
cardinal bootstrap --remove       # удалить systemd-сервис
cardinal bootstrap                # запустить все restart=always контейнеры сейчас
```

Схема:
```
Загрузка → systemd → cardinal-bootstrap.service → cardinal supervisor
  └─ Для каждого detached-контейнера с restart=always или unless-stopped:
      1. Настройка overlayfs
      2. Запуск unshare с неймспейсами
      3. Настройка veth + iptables
```

---

## Кластеризация

cardinal поддерживает multi-node кластеризацию с управлением сервисами, DNS-обнаружением
и rolling updates. Полная документация — [cluster.md](cluster.md).

```bash
# Инициализировать кластер
cardinal cluster init --name prod --bind 0.0.0.0 --port 2375 --token '<strong-random-token>'

# Присоединиться
cardinal cluster join 10.0.0.1:2375

# Показать адрес для подключения других нод
cardinal cluster join-token

# Общая информация о кластере (имя, ноды, сервисы)
cardinal cluster info

# Список нод (с CPU, памятью, лейблами)
cardinal cluster node ls

# Детальная информация о ноде
cardinal cluster node inspect <id>

# Список нод (кратко)
cardinal cluster ls

# Запустить API-сервер (принимает запросы на реплики от других нод)
cardinal cluster serve -p 2375

# Или запустить API-сервер автоматически при init/join
cardinal cluster init --name prod --serve
cardinal cluster join 10.0.0.1:7946 --serve

# Покинуть кластер
cardinal cluster leave
```

---

## Управление сервисами

Сервисы позволяют запускать реплицированные контейнеры по кластеру.
Полная документация — [cluster.md](cluster.md).

```bash
cardinal service create --name web --replicas 3 --port 80:80 nginx:alpine
cardinal service ls
cardinal service scale web 5
cardinal service update web --image nginx:1.25
cardinal service rm web
```

---

## FaaS / Serverless

cardinal может запускать образы как serverless-функции с авто-масштабированием.
Полная документация — [faas.md](faas.md).

```bash
# Развернуть функцию
cardinal fn deploy --name hello --port 8080 --timeout 30 --idle 300 ghcr.io/myorg/hello-func

# Вызвать
cardinal fn call hello --data '{"name": "cardinal"}'

# Список
cardinal fn ls

# Удалить
cardinal fn rm hello
```

---

## Блюпринты

Блюпринты — предварительно настроенные шаблоны контейнеров из репозиториев.

```bash
# Список доступных
cardinal blueprint list

# Информация о блюпринте с примерами
cardinal blueprint info mysql-8
cardinal blueprint info minecraft-server

# Установить
cardinal blueprint install nginx-proxy

# Добавить свой репозиторий
cardinal blueprint repo add https://github.com/user/my-blueprints

# Список репозиториев
cardinal blueprint repo list

# Удалить репозиторий
cardinal blueprint repo remove my-blueprints
```

---

## События

Поток событий жизненного цикла контейнеров в JSON.

```bash
cardinal events                          # в реальном времени
cardinal events --since "2026-07-07 12:00:00"  # события с указанного времени
```

События: `start`, `stop`, `kill`, `oom`, `healthcheck_failed` и другие.

---

## Системные команды

### `cardinal system prune`

Удалить неиспользуемые контейнеры и образы.

```bash
cardinal system prune
```

### `cardinal update [--check]`

Проверить обновления и обновить cardinal.

```bash
cardinal update              # обновить
cardinal update --check      # только проверить
```

### `cardinal version`

Версия.

```bash
cardinal version
```

---

## Архитектура

```
cardinal run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc cardinal init <id>
  │   └─ cardinal init → pivot_root в overlay → настройка /proc/lo/eth0 → exec CMD
  └─ cardinal console-serve <id>
      ├─ читает stdout pipe
      ├─ пишет в лог-файл
      ├─ слушает Unix сокет
      └─ рассылает всем attach-клиентам
```

### Ключевые концепции

| Понятие | Описание |
|---|---|
| **Образ (Image)** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Скачивается один раз через `cardinal pull`. |
| **Контейнер** | Образ + слой записи (overlay). Изменения живут в overlay, не в образе. |
| **Overlay** | Дифф-слой поверх образа. Сохраняется между перезапусками — пакеты остаются установленными. |
| **Том (Volume)** | Bind mount с хоста в контейнер. `-v /data/mybot:/bot` монтирует `/data/mybot` как `/bot`. |
| **Сеть** | Каждый контейнер получает IP `10.0.2.X` на bridge `cardinal0`. Хост: `10.0.2.1`. |

### Как это работает

1. `cardinal run` скачивает образ (если нет в кеше)
2. Создаёт overlay ФС (lower=rootfs образа, upper=слой контейнера, merged=корень контейнера)
3. Запускает `unshare` с неймспейсами PID, mount, net, UTS, IPC
4. Внутри неймспейса `cardinal init` делает `pivot_root` в overlay, монтирует /proc, настраивает сеть
5. Запускает команду контейнера (CMD или `--startup` скрипт)
6. Если в фоне — `cardinal console-serve` перехватывает stdout и раздаёт через Unix сокет для `cardinal attach`

---

## Решение проблем

### cardinal rm -f <контейнер> зависает

```bash
# Принудительно убить процесс
kill -9 $(grep -o '"pid":[0-9]*' /root/.cardinal/containers/*.json | grep -o '[0-9]*')

# Затем удалить
cardinal rm -f <контейнер>

# Ручная очистка если файлы состояния битые
rm -f /root/.cardinal/containers/<id>.json
```

### Overlay не монтируется

```bash
lsmod | grep overlay
modprobe overlay   # если не загружен
```

### Сеть не работает

```bash
# Проверить bridge
ip link show cardinal0

# Включить IP forwarding
sysctl net.ipv4.ip_forward

# Переустановить
cardinal system prune && cardinal pull alpine && cardinal run --rm alpine ping 8.8.8.8
```

### Проброс портов не работает

```bash
# Проверить iptables
iptables -t nat -L -n | grep cardinal

# UFW может блокировать — проверить
ufw status
```

### Rootless режим

cardinal поддерживает rootless-запуск на системах с `newuidmap`/`newgidmap`.
Rootless контейнеры используют userspace networking.

### Сравнение с Docker

| Возможность | cardinal | Docker |
|---|---|---|
| Демон | Нет демона | dockerd обязателен |
| Размер | ~5 MB | ~100+ MB |
| Неймспейсы | PID, Mount, Net, UTS, IPC | Все |
| Bridge сеть | cardinal0 (10.0.2.0/24) | docker0 |
| Проброс портов | iptables DNAT | iptables DNAT |
| Автозапуск | постоянный systemd supervisor | systemd dockerd |
| Формат образов | OCI/Docker V2 | OCI/Docker V2 |
