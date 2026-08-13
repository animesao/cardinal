<!-- dck-version:start -->
**Documentation version:** `1.23.29`
**Project release:** `v1.23.29`
<!-- dck-version:end -->

# Команды и использование

dck — лёгкий container runtime. Нет демона, нет Docker. Просто контейнеры.
~5 MB статический бинарник, OCI образы, bridge-сеть, кластеризация, FaaS.

> Полный список команд, алиасов, префиксов и флагов находится в [полном справочнике CLI](commands.md). Практические рецепты — в [примерах команд](examples.md).

---

## Содержание

- [Руководство по запуску](running.md)
- [Развёртывание сайтов](websites.md)
- [Управление образами](#управление-образами)
  - [dck pull](#dck-pull---platform-osarch-образтег)
  - [dck search](#dck-search-термин)
  - [dck images](#dck-images)
  - [dck rmi](#dck-rmi-образтег)
  - [dck export](#dck-export-образ--o-файлtargz)
  - [dck import](#dck-import-файлtargz)
  - [dck build](#dck-build--t-имятег-опции-)
  - [dck commit](#dck-commit-контейнер-образтег)
  - [dck push](#dck-push-образтег)
  - [dck login / dck logout](#dck-login-registry--dck-logout-registry)
- [Жизненный цикл контейнера](#жизненный-цикл-контейнера)
- [Запуск контейнеров (`dck run`)](#dck-run)
- [Работа с контейнерами](#работа-с-контейнерами)
- [Exec & Attach](#exec--attach)
- [Логи и мониторинг](#логи-и-мониторинг)
- [Сеть](#сеть)
- [Просмотр файлов](#просмотр-файлов--dck-fs)
- [Хранилище и тома](#хранилище-и-тома)
- [Лимиты ресурсов](#лимиты-ресурсов)
- [Безопасность](#безопасность)
- [Переменные окружения](#переменные-окружения)
- [Проверки здоровья (Healthchecks)](#проверки-здоровья-healthchecks)
- [Стартовые скрипты](#стартовые-скрипты)
- [Проброс портов](#проброс-портов)
- [Динамическое управление портами](#динамическое-управление-портами)
- [dck.toml / Compose](#dcktoml--compose)
- [dck up / dck down](#dck-up--dck-down)
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

### `dck pull [--platform os/arch] <образ>[:тег]`

Скачать образ из registry (по умолчанию Docker Hub).

```bash
dck pull nginx
dck pull alpine:3.19
dck pull --platform linux/arm64 eclipse-temurin:21-jre
dck pull registry.example.com/myapp:v1.0
```

Приватные registry: `DOCKER_USERNAME` / `DOCKER_PASSWORD`, или `-u user -p pass` на push.

### `dck images`

Список локально сохранённых образов.

```bash
dck images
```

### `dck search <термин>`

Поиск образов на Docker Hub.

```bash
dck search nginx
dck search python
dck search alpine
dck search python:3.11          # фильтр по тегу
```

Показывает имя, описание, звёзды, загрузки и доступные теги. Можно фильтровать теги через `образ:тег`.

### `dck rmi <образ>[:тег]`

Удалить образ.

```bash
dck rmi nginx:alpine
```

### `dck verify <образ>[:тег]`

Проверить config и диджесты слоёв локального образа.

```bash
dck verify nginx:alpine
```

### `dck export <образ> -o <файл.tar.gz>`

Экспортировать образ в tar.gz (для бэкапа или переноса).

```bash
dck export myapp:v1 -o myapp-v1.tar.gz
```

### `dck import <файл.tar.gz>`

Импортировать образ из tar.gz.

```bash
dck import myapp-v1.tar.gz
```

### `dck build -t <имя>:<тег> [опции] .`

Собрать образ из Dockerfile.

```bash
dck build -t myapp:v1 .
dck build -t myapp:v1 --build-arg VERSION=1.0 -f Dockerfile.prod .
```

**Поддерживаемые инструкции Dockerfile:**
FROM, RUN, COPY, ADD, WORKDIR, ENV, CMD, ENTRYPOINT, EXPOSE, LABEL, USER,
VOLUME, SHELL, ARG, HEALTHCHECK, STOPSIGNAL, ONBUILD.

**Возможности:**
- ✅ Многоэтапная сборка (`FROM ... AS alias`, `COPY --from=`)
- ✅ Подстановка ARG (`$VAR` / `${VAR}` во всех инструкциях)
- ✅ HEALTHCHECK с `--start-period`
- `--no-cache` принимается для совместимости CLI; сейчас dck всё равно выполняет каждую инструкцию Dockerfile и не использует кэш результатов инструкций

### `dck commit <контейнер> <образ>[:тег]`

Создать образ из текущего состояния контейнера (со всеми изменениями в overlay).

```bash
dck commit myproject myproject-snapshot:v1
```

Сохраняет всё, что вы установили (пакеты, файлы, конфиги) в переиспользуемый образ.

### `dck push <образ>[:тег]`

Отправить образ в registry.

```bash
dck push myapp:v1
dck push registry.example.com/myapp:v1
```

Авторизация: `-u user -p pass` или `DOCKER_USERNAME` / `DOCKER_PASSWORD`.

### `dck login <registry>` / `dck logout <registry>`

Войти/выйти из registry для авторизованных pull/push.

```bash
dck login registry.example.com
dck logout registry.example.com
```

---

## Жизненный цикл контейнера

### `dck ps`

Список контейнеров.

```bash
dck ps           # только запущенные
dck ps -a        # все (включая остановленные)
```

### `dck run [опции] <образ> [команда]`

Создать и запустить контейнер. Главная команда.

```bash
# Одноразовая команда
dck run --rm alpine echo "hello"

# Веб-сервер в фоне
dck run -d -n web -p 80:80 nginx:alpine

# Интерактивный shell
dck run -i -t --rm alpine sh

# С лимитами ресурсов
dck run -d --memory 512m --cpus 1.5 node:20 node app.js

# С томом и переменными
dck run -d -v /data:/data -e DB_URL=postgres://... myapp

# С длинными флагами и авто-перезапуском
dck run -d --name myapp --ports 8080:80 --volume /app:/app --restart always --image nginx:alpine
```

**Важно:** dck использует пакет `flag` из Go, поэтому флаги нужно передавать раздельно:
- ✅ `dck run -i -t alpine sh` (правильно)
- ❌ `dck run -it alpine sh` (ошибка — используйте `-i -t`)

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

### `dck stop <контейнер>`

Остановить контейнер (SIGTERM, затем SIGKILL).

```bash
dck stop web
dck stop --all         # остановить все работающие контейнеры
```

### `dck start <контейнер>`

Запустить остановленный контейнер. Все данные в overlay сохраняются.

```bash
dck start web
```

### `dck restart <контейнер>`

Перезапустить контейнер (stop + start).

```bash
dck restart web
```

### `dck rm [-f] <контейнер>`

Удалить контейнер. `-f` принудительно удаляет работающий.

```bash
dck rm web
dck rm -f web         # удалить даже если запущен
```

**Важно:** При удалении контейнера стирается его overlay-слой — все изменения (установленные пакеты, файлы) пропадают.

### `dck set <контейнер> [опции]`

Изменить параметры контейнера без удаления (overlay сохраняется). Останавливает, меняет JSON и запускает заново.

```bash
dck set mc --memory 4g --cpus 2
dck set mc --restart always
dck set mc -e DIFFICULTY=hard
dck set mc --workdir /data-mc
```

### `dck rename <контейнер> <новое-имя>`

Переименовать контейнер.

```bash
dck rename web web-new
```

### `dck port <контейнер>`

Показать проброс портов контейнера.

```bash
dck port web
```

### `dck port add <контейнер> <хост>:<контейнер>[/proto]`

Добавить проброс порта на работающий контейнер без перезапуска. Применяет iptables DNAT правила мгновенно. Порты сохраняются в состоянии контейнера и восстанавливаются после перезапуска.

```bash
dck port add web 8080:80
dck port add web 53:53/udp
```

### `dck port remove <контейнер> <хост>[/proto]`

Удалить проброс порта.

```bash
dck port remove web 8080
dck port remove web 53/udp
```

### `dck port rm <контейнер> <хост>[/proto]`

Алиас для `dck port remove`.

```bash
dck port rm web 8080
```

### `dck top <контейнер>`

Показать процессы внутри контейнера.

```bash
dck top web
```

---

## Exec & Attach

### `dck exec [-i] [-t] <контейнер> <команда>`

Выполнить команду внутри работающего контейнера.

```bash
# Неинтерактивная команда
dck exec web nginx -s reload

# Интерактивный shell с TTY
dck exec -i -t myproject sh

# Интерактивный Python
dck exec -i -t myproject python3
```

Создаёт **новый процесс** внутри контейнера. Входит в неймспейсы контейнера (PID, mount, network, IPC)
и запускает команду прямо в корневой ФС контейнера (chroot не нужен — корень уже установлен через pivot_root).

### `dck attach <контейнер>`

Подключиться к **главному процессу** контейнера (работает только для контейнеров с `-d`).

```bash
dck run -d -i -t -n myproject alpine sh
dck attach myproject    # подключиться к sh
```

> **exec vs attach:** `attach` подключается к stdin/stdout главного процесса. `exec` запускает новую команду внутри контейнера. `console` — сокращение для `exec -i -t` с автоопределением shell.

`dck attach` **устойчив к Ctrl+C** — контейнер продолжает работать.

### `dck console <контейнер>`

Автоматически определить и запустить интерактивный shell внутри контейнера.
Эквивалент `dck exec -i -t <контейнер> sh`.

```bash
dck console myproject
```

---

## Логи и мониторинг

### `dck logs [-f] [--tail <n>] <контейнер>`

Показать stdout/stderr контейнера.

```bash
dck logs web            # вывод текущего запуска
dck logs -f web         # следить за новыми строками
dck logs --tail 20 web  # последние 20 строк
dck logs -f --tail 10 web  # последние 10 + следить
```

При каждом новом запуске контейнера dck создаёт свежий лог, поэтому вывод прошлых циклов `stop`/`start` или `restart` не дописывается бесконечно. Логи, которые создаёт само приложение (например, Minecraft `/data/logs/latest.log`), остаются в подключённом хранилище приложения.

Для root логи dck находятся в `/root/.dck/logs/<container-id>.log`; изменить каталог состояния можно через `DCK_DATA_DIR`. Практические примеры находятся в [руководстве по запуску](running.md).

### `dck backup create|list|restore|enable|disable|status|verify`

Создавайте разовые архивы или включайте постоянное расписание для конкретного контейнера. Авто-бэкап включает writable overlay и именованные тома, но не host bind mount — каталоги приложения на хосте нужно архивировать отдельно. Для согласованности dck ненадолго останавливает работающий контейнер, создаёт архив и запускает его снова. Первый автоматический архив создаётся после заданного интервала, а не сразу.

```bash
dck backup enable minecraft --interval 6h --retention 14
dck backup status minecraft
dck backup list
dck backup disable minecraft

# Свой каталог (защищённые системные пути запрещены)
dck backup enable minecraft --interval 24h --retention 7 --dir /data/backups/minecraft
```

Чтобы расписание продолжало работать после выхода из CLI, установите systemd supervisor:

```bash
dck bootstrap --install
```

`dck backup create ИМЯ -o file.tar.gz` остаётся разовым ручным бэкапом. Восстановление выполняется только в остановленный контейнер: `dck backup restore ИМЯ file.tar.gz`. Архив можно проверить по контрольной сумме: `dck backup verify ФАЙЛ.tar.gz`; если checksum-файла рядом нет, dck сообщит, что архив не проверен.

### `dck stats [контейнер]`

Использование CPU, памяти, I/O и PIDs в реальном времени. Через cgroups v2.

```bash
dck stats               # все контейнеры
dck stats web           # конкретный
```

### `dck info`

Информация о системе: версия ядра, storage driver, директория данных, CPU, память, диск.

```bash
dck info
```

---

## Сеть

### Режимы сети

| Режим | Описание |
|---|---|
| `bridge` (по умолч.) | Каждый контейнер получает IP `10.0.2.X` на bridge `dck0`. Хост: `10.0.2.1`. |
| `none` | Без сети (только loopback) |
| `host` | Общая сеть с хостом (для VPN, сниффинга) |
| `<имя>` | Пользовательский Linux bridge, созданный через `dck network create` | `--network appnet` |

```bash
dck run -d -n web -p 80:80 nginx:alpine       # bridge (по умолч.)
dck run -d --network none alpine sleep infinity
dck run -d --network host myvpn-container

dck network create --subnet 10.20.0.0/24 appnet
dck network ls
dck run -d --network appnet -n app alpine sleep infinity
dck network inspect appnet
dck network rm appnet   # только после удаления контейнеров сети
```

### Схема сети

```
Хост:        dck0  10.0.2.1/24
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
dck run -d --dns 1.1.1.1 --dns 8.8.8.8 nginx
```

---

## Динамическое управление портами

Позволяет добавлять и удалять пробросы портов на работающем контейнере без остановки.

```bash
# Добавить порт
dck port add web 8080:80

# Удалить порт
dck port remove web 8080

# Алиас для remove
dck port rm web 8080
```

Правила iptables DNAT применяются мгновенно. Порты сохраняются в состоянии контейнера (`~/.dck/containers/<id>.json`) и автоматически восстанавливаются при перезапуске контейнера (`dck start`).

---

## Хранилище и тома

### Синтаксис томов

```bash
# Bind mount (директория хоста)
-v /путь/на/хосте:/путь/в/контейнере
-v /путь/на/хосте:/путь/в/контейнере:ro     # только чтение
-v /путь/на/хосте:/путь/в/контейнере:shared # shared mount

# Именованный том (управляется dck)
-v myvolume:/путь/в/контейнере

# tmpfs (в памяти)
-v tmpfs:/путь/в/контейнере:size=1G,mode=0777

# NFS
-v nfs://сервер:/экспорт:/путь/в/контейнере:nfsopts=hard,intr
```

### Именованные тома

Тома хранятся в `~/.dck/volumes/`.

```bash
# Создать том
dck volume create mydata

# Список томов
dck volume ls

# Информация о томе
dck volume inspect mydata

# Удалить том
dck volume rm mydata

# Удалить неиспользуемые тома
dck volume prune
```

### Как работает хранилище

```
Хранилище: /root/.dck/

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
Они сохраняются между перезапусками (`dck stop` + `dck start`), но **удаляются**
при удалении контейнера (`dck rm`).

Чтобы сохранить изменения навсегда — используйте `dck commit`.

### Просмотр файлов — `dck fs`

Просмотр файлов контейнера без запуска shell. Работает на **запущенных** и **остановленных** контейнерах — overlay остаётся смонтированным после `stop`.

```bash
dck fs ls <контейнер> [путь]              # Список файлов
dck fs cat <контейнер> <путь>             # Содержимое файла
dck fs tree <контейнер> [путь]            # Дерево директорий
dck fs find [контейнер] [путь] [флаги]    # Поиск файлов
  --name <шаблон>     Фильтр по имени (подстрока, напр. "index")
  --grep <текст>      Поиск внутри файлов
  --type f|d          Только файлы или папки
  --max-depth <n>     Макс. глубина
```

Примеры:
```bash
dck fs ls web /etc/nginx
dck fs cat web /etc/nginx/conf.d/default.conf
dck fs tree mc-server /data --max-depth 2
dck fs find web --name "*.conf" --grep "server_name"
dck fs find --name "index"                              # искать во всех контейнерах
```

### Копирование файлов

```bash
# Из контейнера на хост
dck cp web:/etc/nginx/nginx.conf ./nginx.conf

# С хоста в контейнер
dck cp ./app.py web:/app/
```

---

## Лимиты ресурсов

### Память

```bash
dck run -d --memory 512m nginx    # 512 мегабайт
dck run -d --memory 1g nginx      # 1 гигабайт
dck run -d --memory 2g nginx      # 2 гигабайта
```

Через cgroups v2 memory controller. При превышении — OOM kill.

### CPU

```bash
dck run -d --cpus 1.5 nginx       # 1.5 ядра
dck run -d --cpus 2 nginx         # 2 ядра
```

Через CFS quota в cgroups v2.

### Диск

```bash
dck run -d --disk 1G nginx        # 1 GB
dck run -d --disk 10G nginx       # 10 GB
```

Создаёт sparse ext4 образ, который монтируется как overlay. Требует `mkfs.ext4`.

---

## Безопасность

### Пользователь

Запуск от непривилегированного пользователя:

```bash
dck run -d --user 1000 nginx
dck run -d --user 1000:1000 nginx   # UID:GID
```

### Capabilities

По умолчанию dck сохраняет безопасный Docker-совместимый набор capabilities, необходимый обычным образам (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `FSETID`, `KILL`, `SETGID`, `SETUID`, `SETPCAP`, `NET_BIND_SERVICE`, `NET_RAW`, `SYS_CHROOT`, `MKNOD`, `AUDIT_WRITE` и `SETFCAP`). Опасные capabilities, такие как `SYS_ADMIN` и `SYS_MODULE`, по-прежнему отключены. Поэтому стандартные образы вроде `nginx:alpine` могут нормально подготовить файловую систему.

```bash
# Добавить capability
dck run -d --cap-add NET_ADMIN nginx
dck run -d --cap-add NET_ADMIN --cap-add SYS_PTRACE nginx

# Отключить все capabilities (максимальное ограничение)
dck run -d --cap-drop ALL nginx

# Вернуть конкретные после --cap-drop ALL
dck run -d --cap-drop ALL --cap-add NET_BIND_SERVICE nginx
```

### Read-only rootfs

```bash
dck run -d --readonly nginx
```

Корневая ФС только для чтения. Запись в тома по-прежнему работает.

### Запрет привилегий

```bash
dck run -d --no-new-privs nginx
```

Запрещает получение новых привилегий (setuid, setgid, capability) всем процессам в контейнере.

### Sysctls

```bash
dck run -d --sysctl net.ipv4.ip_forward=1 nginx
```

---

## Переменные окружения

```bash
# Одна переменная
dck run -e MY_VAR=value nginx

# Несколько
dck run -e DB_HOST=localhost -e DB_PORT=5432 nginx

# Из файла
dck run --env-file .env nginx
```

**Формат .env файла:**
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=admin
```

### Авто-внедрённые DCK_* переменные

При запуске контейнера dck внедряет:

| Переменная | Описание |
|---|---|
| `DCK_CONTAINER_ID` | ID контейнера |
| `DCK_CONTAINER_NAME` | Имя контейнера |
| `DCK_IMAGE_NAME` | Имя образа (например `library/alpine`) |
| `DCK_IMAGE_TAG` | Тег образа (например `latest`) |
| `DCK_HOSTNAME` | Hostname контейнера |
| `DCK_MEMORY` | Лимит памяти в байтах |
| `DCK_CPU` | Лимит CPU в ядрах |
| `DCK_IP` | IP адрес контейнера |
| `DCK_RESTART` | Политика рестарта |
| `DCK_PORT_TCP_80` | Проброс портов |

Внутри контейнера доступны скрипты в `/dck/`:
- `/dck/info` — информация о контейнере
- `/dck/env` — переменные DCK_*
- `/dck/help` — справка

---

## Проверки здоровья (Healthchecks)

Запускает команду внутри контейнера через заданный интервал. После `retries` неудач контейнер убивается и перезапускается.

```bash
dck run -d \
  --healthcheck-cmd "curl -f http://localhost || exit 1" \
  --healthcheck-interval 30 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  nginx
```

Healthchecks можно также задавать в compose-файлах и dck.toml.

---

## Стартовые скрипты

`--startup` запускает кастомный скрипт вместо команды из образа:

```bash
# Скрипт строкой
dck run -d --startup "#!/bin/sh\necho 'Hello from startup'" alpine sleep infinity

# Из файла
dck run -d --startup @./myscript.sh ubuntu
```

Скрипт записывается в `/startup.sh` и выполняется через `/bin/sh`.
При наличии `--startup` он **заменяет** стандартный CMD/entrypoint.

---

## dck.toml / Compose

### Формат dck.toml

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

dck поддерживает стандартный формат Docker Compose YAML. Полная документация — в [compose.md](compose.md).

---

## dck up / dck down

### `dck up [имя] [-f <файл>]`

Создать и запустить контейнеры из compose-файла.

Автоопределение (по порядку):
1. `dck.toml`
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
dck up                    # автоопределение
dck up myapp              # только сервис "myapp"
dck up -f compose.prod.yaml
dck up                    # запустить конфигурацию
dck up -f compose.prod.yaml
dck up myapp              # только сервис "myapp"
dck up --generate         # создать dck.toml из существующих контейнеров
```

### `dck down [имя] [-f <файл>]`

Остановить и удалить контейнеры из compose-файла.

```bash
dck down                  # stop + remove
dck down myapp            # только "myapp"
dck down -f dck.toml
dck down -a               # удалить ВСЕ контейнеры
# Для удаления всех контейнеров без чтения конфигурации:
dck down -a
```

---

## dck serve

Запустить Docker-совместимый REST API сервер.

```bash
dck serve -p 2375  # по умолчанию только localhost; для внешнего bind нужен --token
```

Совместим с Docker-клиентами, Portainer, VS Code Dev Containers и CI.

---

## Автозапуск при загрузке

Detached-контейнеры с `--restart always` или `--restart unless-stopped` запускаются автоматически после перезагрузки. Persistent supervisor не подхватывает `on-failure` после завершения короткого detached-процесса CLI.

dck сам устанавливает systemd-сервис когда:
- `dck run --restart always <образ>`
- `dck set <контейнер> --restart always`
- `dck up` (если в конфиге есть restart: "always")

Также можно управлять вручную:

```bash
dck bootstrap --install      # установить systemd-сервис
dck bootstrap --remove       # удалить systemd-сервис
dck bootstrap                # запустить все restart=always контейнеры сейчас
```

Схема:
```
Загрузка → systemd → dck-bootstrap.service → dck supervisor
  └─ Для каждого detached-контейнера с restart=always или unless-stopped:
      1. Настройка overlayfs
      2. Запуск unshare с неймспейсами
      3. Настройка veth + iptables
```

---

## Кластеризация

dck поддерживает multi-node кластеризацию с управлением сервисами, DNS-обнаружением
и rolling updates. Полная документация — [cluster.md](cluster.md).

```bash
# Инициализировать кластер
dck cluster init --name prod --bind 0.0.0.0 --port 2375 --token '<strong-random-token>'

# Присоединиться
dck cluster join 10.0.0.1:2375

# Показать адрес для подключения других нод
dck cluster join-token

# Общая информация о кластере (имя, ноды, сервисы)
dck cluster info

# Список нод (с CPU, памятью, лейблами)
dck cluster node ls

# Детальная информация о ноде
dck cluster node inspect <id>

# Список нод (кратко)
dck cluster ls

# Запустить API-сервер (принимает запросы на реплики от других нод)
dck cluster serve -p 2375

# Или запустить API-сервер автоматически при init/join
dck cluster init --name prod --serve
dck cluster join 10.0.0.1:7946 --serve

# Покинуть кластер
dck cluster leave
```

---

## Управление сервисами

Сервисы позволяют запускать реплицированные контейнеры по кластеру.
Полная документация — [cluster.md](cluster.md).

```bash
dck service create --name web --replicas 3 --port 80:80 nginx:alpine
dck service ls
dck service scale web 5
dck service update web --image nginx:1.25
dck service rm web
```

---

## FaaS / Serverless

dck может запускать образы как serverless-функции с авто-масштабированием.
Полная документация — [faas.md](faas.md).

```bash
# Развернуть функцию
dck fn deploy --name hello --port 8080 --timeout 30 --idle 300 ghcr.io/myorg/hello-func

# Вызвать
dck fn call hello --data '{"name": "dck"}'

# Список
dck fn ls

# Удалить
dck fn rm hello
```

---

## Блюпринты

Блюпринты — предварительно настроенные шаблоны контейнеров из репозиториев.

```bash
# Список доступных
dck blueprint list

# Информация о блюпринте с примерами
dck blueprint info mysql-8
dck blueprint info minecraft-server

# Установить
dck blueprint install nginx-proxy

# Добавить свой репозиторий
dck blueprint repo add https://github.com/user/my-blueprints

# Список репозиториев
dck blueprint repo list

# Удалить репозиторий
dck blueprint repo remove my-blueprints
```

---

## События

Поток событий жизненного цикла контейнеров в JSON.

```bash
dck events                          # в реальном времени
dck events --since "2026-07-07 12:00:00"  # события с указанного времени
```

События: `start`, `stop`, `kill`, `oom`, `healthcheck_failed` и другие.

---

## Системные команды

### `dck system prune`

Удалить неиспользуемые контейнеры и образы.

```bash
dck system prune
```

### `dck update [--check]`

Проверить обновления и обновить dck.

```bash
dck update              # обновить
dck update --check      # только проверить
```

### `dck version`

Версия.

```bash
dck version
```

---

## Архитектура

```
dck run -d
  ├─ unshare --fork --pid --mount --net --uts --ipc dck init <id>
  │   └─ dck init → pivot_root в overlay → настройка /proc/lo/eth0 → exec CMD
  └─ dck console-serve <id>
      ├─ читает stdout pipe
      ├─ пишет в лог-файл
      ├─ слушает Unix сокет
      └─ рассылает всем attach-клиентам
```

### Ключевые концепции

| Понятие | Описание |
|---|---|
| **Образ (Image)** | Read-only rootfs (`python:3.11-slim`, `nginx:alpine`). Скачивается один раз через `dck pull`. |
| **Контейнер** | Образ + слой записи (overlay). Изменения живут в overlay, не в образе. |
| **Overlay** | Дифф-слой поверх образа. Сохраняется между перезапусками — пакеты остаются установленными. |
| **Том (Volume)** | Bind mount с хоста в контейнер. `-v /data/mybot:/bot` монтирует `/data/mybot` как `/bot`. |
| **Сеть** | Каждый контейнер получает IP `10.0.2.X` на bridge `dck0`. Хост: `10.0.2.1`. |

### Как это работает

1. `dck run` скачивает образ (если нет в кеше)
2. Создаёт overlay ФС (lower=rootfs образа, upper=слой контейнера, merged=корень контейнера)
3. Запускает `unshare` с неймспейсами PID, mount, net, UTS, IPC
4. Внутри неймспейса `dck init` делает `pivot_root` в overlay, монтирует /proc, настраивает сеть
5. Запускает команду контейнера (CMD или `--startup` скрипт)
6. Если в фоне — `dck console-serve` перехватывает stdout и раздаёт через Unix сокет для `dck attach`

---

## Решение проблем

### dck rm -f <контейнер> зависает

```bash
# Принудительно убить процесс
kill -9 $(grep -o '"pid":[0-9]*' /root/.dck/containers/*.json | grep -o '[0-9]*')

# Затем удалить
dck rm -f <контейнер>

# Ручная очистка если файлы состояния битые
rm -f /root/.dck/containers/<id>.json
```

### Overlay не монтируется

```bash
lsmod | grep overlay
modprobe overlay   # если не загружен
```

### Сеть не работает

```bash
# Проверить bridge
ip link show dck0

# Включить IP forwarding
sysctl net.ipv4.ip_forward

# Переустановить
dck system prune && dck pull alpine && dck run --rm alpine ping 8.8.8.8
```

### Проброс портов не работает

```bash
# Проверить iptables
iptables -t nat -L -n | grep dck

# UFW может блокировать — проверить
ufw status
```

### Rootless режим

dck поддерживает rootless-запуск на системах с `newuidmap`/`newgidmap`.
Rootless контейнеры используют userspace networking.

### Сравнение с Docker

| Возможность | dck | Docker |
|---|---|---|
| Демон | Нет демона | dockerd обязателен |
| Размер | ~5 MB | ~100+ MB |
| Неймспейсы | PID, Mount, Net, UTS, IPC | Все |
| Bridge сеть | dck0 (10.0.2.0/24) | docker0 |
| Проброс портов | iptables DNAT | iptables DNAT |
| Автозапуск | постоянный systemd supervisor | systemd dockerd |
| Формат образов | OCI/Docker V2 | OCI/Docker V2 |
