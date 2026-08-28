<!-- cardinal-version:start -->
**Documentation version:** `2.0.4`
**Project release:** `v2.0.4`
<!-- cardinal-version:end -->

# Полный справочник CLI cardinal

Полный справочник Linux-бинарника `cardinal`: дерево команд, позиционные аргументы, короткие и длинные флаги, алиасы, значения по умолчанию и правила безопасности.

> **Платформа:** cardinal работает на Linux и требует namespaces, OverlayFS, cgroups v2, `unshare`, `nsenter`, `mount`, `ip`, `iptables` и `pgrep`.

## 1. Синтаксис и префиксы

```text
cardinal КОМАНДА [ПОДКОМАНДА] [ОПЦИИ] [ПОЗИЦИОННЫЕ-АРГУМЕНТЫ]
```

| Форма | Значение | Пример |
|---|---|---|
| `COMMAND` | Верхнеуровневая команда | `run`, `logs`, `volume` |
| `SUBCOMMAND` | Операция внутри команды | `volume create`, `backup enable` |
| `<value>` | Обязательное значение | `<container>`, `<image>` |
| `[value]` | Необязательное значение | `[path]`, `[service]` |
| `-x` | Короткий флаг | `-d`, `-p 8080:80` |
| `--option` | Длинный флаг | `--restart unless-stopped` |
| `--option=value` | Длинный флаг со значением через `=` | `--memory=2g` |
| `--` | Конец опций; всё после него — аргументы команды | `cardinal run alpine sh -c -- 'echo -n hi'` |

Булевы короткие флаги можно указывать раздельно (`-i -t`) или объединённой формой (`-it`, `-dit`) — cardinal нормализует сокращения вроде `-it`/`-dit` (для `run`) и `-it` (для `exec`) перед парсингом. Значения с пробелами, `$`, `*`, `:` и shell-синтаксисом заключайте в кавычки.

### Основные алиасы

- `ls` и `list` — алиасы списковых подкоманд там, где указано ниже.
- `-p` / `--ports` — порты в `run`.
- `-v` / `--volume` / `--vol` — монтирование в `run`.
- `--memory` / `--ram` и `--cpus` / `--cpu` — алиасы ограничений ресурсов в `run` и `set`.
- `--cmd` / `--command` — алиасы команды в `run`.
- `-l` / `--label` — алиасы метки.
- `version`, `--version`, `-v` — версия; `help`, `--help`, `-h` — справка.
- В подкомандах удаления обычно доступны `rm` и `remove`.

## 2. Образы и registry

### `cardinal pull [--platform os/arch] IMAGE[:TAG]`

Скачать образ из Docker Hub или настроенного registry. Если tag не указан, используется `latest`. `--platform` выбирает платформу манифеста, например `linux/amd64` или `linux/arm64`.

```bash
cardinal pull alpine
cardinal pull alpine:3.20
cardinal pull --platform linux/arm64 eclipse-temurin:21
```

### `cardinal push IMAGE[:TAG] [-u USER] [-p PASSWORD]`

Отправить локальный образ в registry. Флаги учётных данных — `-u` и `-p`; для автоматизации лучше использовать `cardinal login` или переменные окружения.

```bash
cardinal push myapp:v1
cardinal push -u registry-user -p 'secret' registry.example.com/team/myapp:v1
```

### `cardinal images`

Показать локальные образы.

```bash
cardinal images
```

### `cardinal search TERM`

Искать образы на Docker Hub. После двоеточия можно указать фильтр тегов.

```bash
cardinal search nginx
cardinal search python:3.12
```

### `cardinal rmi IMAGE[:TAG]`

Удалить локальный образ. По умолчанию используется tag `latest`.

```bash
cardinal rmi alpine:3.20
```

### `cardinal verify IMAGE[:TAG]`

Проверить config и диджесты слоёв локального образа по сохранённым манифестам.

```bash
cardinal verify alpine:3.20
```

### `cardinal commit CONTAINER IMAGE[:TAG]`

Создать образ из текущего изменяемого состояния контейнера.

```bash
cardinal commit web registry.example.com/team/web:snapshot
```

### `cardinal build -t NAME[:TAG] [OPTIONS] [CONTEXT]`

Собрать образ по Dockerfile. Контекст по умолчанию — `.`, tag обязателен.

| Флаг | Описание |
|---|---|
| `-t NAME[:TAG]` | Обязательное имя и tag образа |
| `-f FILE` | Путь к Dockerfile; по умолчанию `<context>/Dockerfile` |
| `--no-cache` | Принимается для совместимости; сейчас результаты отдельных инструкций не переиспользуются |
| `--build-arg KEY=VALUE` | Повторяемая переменная времени сборки |
| `--quiet` | Скрыть вывод сборки |
| `--cpu N` | Ограничение CPU сборки |
| `--memory BYTES` | Ограничение памяти сборки в байтах |

```bash
cardinal build -t myapp:dev .
cardinal build -t myapp:prod -f Dockerfile.prod --build-arg VERSION=1.0 ./src
```

### `cardinal export IMAGE[:TAG] [-o FILE.tar.gz]`

Экспортировать образ в архив. `-o` задаёт путь результата.

```bash
cardinal export myapp:v1 -o /data/images/myapp-v1.tar.gz
```

### `cardinal import FILE.tar.gz [FILE.tar.gz ...]`

Импортировать один или несколько архивов образов.

```bash
cardinal import /data/images/myapp-v1.tar.gz
```

### `cardinal login REGISTRY [-u USER] [-p PASSWORD] [--password-stdin]`

Сохранить данные registry. Если данные не указаны, cardinal запросит их интерактивно. `--password-stdin` читает пароль из stdin.

```bash
cardinal login registry.example.com
echo "$REGISTRY_PASSWORD" | cardinal login registry.example.com -u "$REGISTRY_USER" --password-stdin
```

### `cardinal logout REGISTRY [REGISTRY ...]`

Удалить сохранённые данные registry.

```bash
cardinal logout registry.example.com
```

## 3. Жизненный цикл контейнера

### `cardinal run [OPTIONS] IMAGE [COMMAND ...]`

Создать и запустить контейнер. Образ можно передать через `--image`, а команду — через `--cmd` или `--command`. `--cmd` разбирается как shell-подобная строка; позиционные аргументы команды сохраняются как отдельные аргументы.

| Флаг | Описание |
|---|---|
| `-d` | Запуск в фоне |
| `-n NAME` / `--name NAME` | Имя контейнера |
| `-i` | Оставить stdin интерактивным |
| `-t` | Выделить TTY |
| `--rm` | Удалить контейнер после завершения процесса |
| `-h HOSTNAME` | Hostname контейнера |
| `--restart POLICY` | `no`, `always`, `on-failure` или `unless-stopped` |
| `--restart-delay DURATION` | Задержка перезапуска после сбоя, например `10s` или `1m` |
| `--restart-max-attempts N` | Бюджет crash-loop: автоматический перезапуск блокируется после N сбоев в течение окна (по умолчанию 5) |
| `--restart-window DURATION` | Окно бюджета crash-loop, например `10m`, `1h` |
| `-e KEY=VALUE` | Повторяемая переменная окружения |
| `--env-file FILE` | Загрузить строки `KEY=VALUE` или `export KEY=VALUE` |
| `-p HOST:CONTAINER[/PROTO]` | Проброс порта; можно передать несколько через запятую |
| `--ports` | Алиас `-p` |
| `-v SRC:DST[:MODE]` | Bind mount или именованный volume; режимы `:ro`/`:rw`, propagation `:shared`/`:rslave`, `nocopy`, а также спецификации `tmpfs:` и `nfs://` |
| `--volume`, `--vol` | Алиасы `-v` |
| `--memory`, `--ram LIMIT` | Лимит памяти, например `512m`, `1g` |
| `--cpus`, `--cpu N` | Лимит CPU, например `0.5`, `2` |
| `--disk LIMIT` | Лимит диска, например `1G`, `2T` |
| `--workdir DIR` | Рабочий каталог в контейнере |
| `--image IMAGE` | Образ вместо позиционного аргумента |
| `--cmd`, `--command COMMAND` | Команда вместо позиционных аргументов |
| `--entrypoint COMMAND` | Переопределить entrypoint образа |
| `--network MODE` | `bridge` (по умолчанию), `none`, `host` или имя пользовательской сети |
| `-l`, `--label KEY=VALUE` | Повторяемая метка контейнера |
| `--cap-add CAP` | Добавить Linux capability; можно повторять |
| `--cap-drop CAP` | Убрать capability; можно повторять |
| `--user USER` | Имя пользователя или `UID:GID` |
| `--readonly` | Read-only корневая файловая система |
| `--no-new-privs` | Запретить получение новых привилегий |
| `--sysctl KEY=VALUE` | Повторяемая настройка sysctl |
| `--ulimit NAME=SOFT:HARD` | Повторяемый ulimit |
| `--dns IP` | Повторяемый DNS-сервер |
| `--startup SCRIPT` | Inline-скрипт или `@FILE`; заменяет обычную команду/entrypoint |
| `--healthcheck-cmd COMMAND` | Команда healthcheck |
| `--healthcheck-interval SECONDS` | Интервал healthcheck |
| `--healthcheck-retries N` | Число последовательных ошибок до перезапуска |
| `--healthcheck-timeout SECONDS` | Таймаут healthcheck |
| `--seccomp-profile FILE` | Путь к JSON-профилю seccomp (по умолчанию: встроенный профиль) |
| `--apparmor-profile NAME` | Имя профиля AppArmor |
| `--isolated` | Изолировать контейнер от других (сетевая сегментация) |
| `--encrypted-backup` | Шифровать архивы бэкапов AES-256-GCM |
| `--audit-log` | Включить аудит-логирование для событий контейнера |

Примеры:

```bash
cardinal run --rm alpine echo hello
cardinal run -d -n web -p 8080:80 nginx:alpine
cardinal run -d --name app --image python:3.12 --cmd 'python /app/main.py'
cardinal run -i -t --rm alpine sh
cardinal run -d --restart unless-stopped --restart-delay 1m --memory 4g --cpus 2 myapp:latest
```

Политики автоматического перезапуска обслуживает `cardinal-bootstrap.service`. Контейнер `unless-stopped`, остановленный вручную, не запускается при boot recovery; `always` запускается после перезагрузки. Сервис устанавливается автоматически, если это возможно для root, либо явно командой `cardinal bootstrap --install`.

Частые быстрые падения защищены: когда исчерпан бюджет crash-loop (по умолчанию 5 перезапусков, `--restart-max-attempts` и `--restart-window`), автоматический перезапуск блокируется, и контейнер остаётся остановленным до явного `cardinal start`.

Host bind source должен существовать и не должен находиться в защищённых системных путях `/root`, `/etc`, `/var`, `/usr`, `/opt` или `/run`. Используйте каталог данных `/data/myapp` либо именованный volume:

```bash
mkdir -p /data/myapp
cardinal run -d -v /data/myapp:/app myapp:latest
```

### `cardinal ps [-a]`

Показать контейнеры. `-a` добавляет остановленные и созданные контейнеры.

```bash
cardinal ps
cardinal ps -a
```

### `cardinal inspect [--sensitive] CONTAINER [CONTAINER ...]`

Вывести состояние контейнера в JSON. Чувствительные поля скрыты, если не указать `--sensitive`.

```bash
cardinal inspect web
cardinal inspect --sensitive web
```

### `cardinal start CONTAINER`

Запустить остановленные контейнеры, сохраняя overlay и volumes.

### `cardinal stop [--all] CONTAINER`

Остановить контейнеры. `--all` останавливает все работающие контейнеры.

### `cardinal restart CONTAINER`

Перезапустить контейнер: stop + start.

### `cardinal rm [-f|-r] CONTAINER`

Удалить контейнер. `-f` принудительно удаляет работающий контейнер (`-r` — алиас для `-f`). Изменяемый overlay удаляется, именованные volumes сохраняются.

### `cardinal rename CONTAINER NEW_NAME`

Переименовать контейнер.

### `cardinal set CONTAINER [OPTIONS]`

Изменить параметры без удаления контейнера. Если контейнер работал, cardinal остановит и снова запустит его.

| Флаг | Описание |
|---|---|
| `--memory`, `--ram LIMIT` | Лимит памяти |
| `--cpus`, `--cpu N` | Лимит CPU |
| `--disk LIMIT` | Лимит диска |
| `--restart POLICY` | Политика перезапуска |
| `--restart-delay DURATION` | Задержка восстановления |
| `--restart-max-attempts N` | Бюджет перезапусков crash-loop |
| `--restart-window DURATION` | Окно бюджета crash-loop |
| `--workdir DIR` | Рабочий каталог |
| `-e KEY=VALUE` | Добавить переменную окружения |
| `--entrypoint COMMAND` | Переопределить entrypoint |
| `--user USER` | Пользователь или UID:GID |
| `--readonly` | Read-only rootfs |
| `--no-new-privs` | Запретить повышение привилегий |
| `-h HOSTNAME` | Hostname |
| `--network MODE` | Режим сети |
| `--startup SCRIPT` | Inline-скрипт или `@FILE`; выполняется перед командой контейнера |

```bash
cardinal set minecraft --restart unless-stopped --restart-delay 1m
cardinal set web --memory 2g --cpus 2
```

## 4. Сетевые команды

### `cardinal network create [--subnet CIDR] NAME`

Создать пользовательскую Linux bridge-сеть. Если `--subnet` не указан, cardinal выберет свободную private-сеть `/24`.

```bash
cardinal network create --subnet 10.20.0.0/24 appnet
cardinal network ls
cardinal network inspect appnet
cardinal run -d --network appnet alpine sleep infinity
cardinal network rm appnet
```

`network rm` откажет, пока сеть используется и IP-адреса заняты. Сначала удалите контейнеры сети. Пользовательский bridge требует root или `CAP_NET_ADMIN`, а также `ip`/`iptables`.

### `cardinal network ls|list`

Показать пользовательские bridge-сети. Встроенная сеть `cardinal0` сюда не входит.

### `cardinal network inspect NAME`

Показать ID, driver, subnet, gateway, bridge-интерфейс и число занятых IP.

### `cardinal network rm|remove NAME`

Удалить неиспользуемую пользовательскую сеть и её firewall-правила.

## 5. Логи, мониторинг, выполнение и файлы

### `cardinal logs [-f] [--tail N] [--previous] [--all] CONTAINER`

Показать stdout/stderr контейнера. `-f` следит за текущим выводом, `--tail` ограничивает текущий вывод, `--previous` показывает предыдущий повернутый запуск, `--all` показывает текущий и повернутые логи. При каждом новом запуске cardinal создаёт свежий файл лога.

```bash
cardinal logs web
cardinal logs --tail 100 web
cardinal logs -f web
cardinal logs --previous web
cardinal logs --all web
```

### `cardinal attach CONTAINER`

Подключиться к главному процессу через Unix console socket. Контейнер должен работать. `Ctrl+C` безопасен: отсоединение не останавливает контейнер.

### `cardinal exec [-i] [-t] CONTAINER COMMAND [ARGS ...]`

Запустить новый процесс внутри работающего контейнера.

```bash
cardinal exec web nginx -s reload
cardinal exec -i -t web /bin/sh
```

### `cardinal console CONTAINER`

Открыть интерактивную оболочку: сначала Bash, затем `/bin/sh`.

### `cardinal console-serve ...`

Внутренний сервер консоли для attach; обычно вручную не запускается.

### `cardinal cp SRC DST`

Копировать между хостом и контейнером. Для endpoint контейнера используется `CONTAINER:/path`; копирование контейнер-контейнер не поддерживается.

```bash
cardinal cp ./config.yml web:/etc/app/config.yml
cardinal cp web:/etc/app/config.yml ./backup/
```

### `cardinal fs ls|cat|tree|find ...`

Просматривать merged filesystem работающего или остановленного контейнера.

```text
cardinal fs ls CONTAINER [PATH]
cardinal fs cat CONTAINER PATH
cardinal fs tree CONTAINER [PATH]
cardinal fs find CONTAINER [PATH] [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
cardinal fs find [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
```

Последняя форма ищет во всех контейнерах. `--name` фильтрует по подстроке имени, `--grep` ищет содержимое файлов, `--type` выбирает файлы или каталоги, `--max-depth` ограничивает глубину.

### `cardinal stats [CONTAINER] [--no-stream]`

Показать статистику cgroups v2: CPU, память, I/O и процессы. `--no-stream` выводит один снимок и завершает работу.

### `cardinal top CONTAINER`

Показать процессы внутри контейнера.

### `cardinal info`

Показать сведения о хосте, runtime, storage, CPU, памяти, диске и контейнерах.

### `cardinal events [--since TIME]`

Поток событий контейнеров в JSON. `--since` принимает RFC3339 или `YYYY-MM-DD HH:MM:SS`.

## 6. Порты, volumes и backup

### `cardinal port CONTAINER`

Показать проброшенные порты.

### `cardinal port add CONTAINER HOST:CONTAINER[/PROTO]`

Добавить TCP (по умолчанию) или UDP mapping без пересоздания контейнера.

### `cardinal port remove|rm CONTAINER HOST[/PROTO]`

Удалить dynamic mapping. `rm` — алиас `remove`.

### `cardinal volume create [OPTIONS] [NAME]`

Создать именованный local volume. Если имя не указано, cardinal сгенерирует его.

| Флаг | Описание |
|---|---|
| `-d DRIVER` | Драйвер; по умолчанию `local` |
| `-l`, `--label KEY=VALUE` | Повторяемая метка |

```bash
cardinal volume create app-data
cardinal volume create -l env=prod app-data
```

### `cardinal volume ls|list`

Список именованных volumes.

### `cardinal volume inspect NAME [NAME ...]`

Показать драйвер, mountpoint, дату создания и labels.

### `cardinal volume rm|remove NAME [NAME ...]`

Удалить volumes. Операция необратима.

### `cardinal volume prune`

Удалить local volumes, на которые не ссылается ни один контейнер.

### `cardinal backup COMMAND`

| Команда | Синтаксис | Описание |
|---|---|---|
| `create` | `cardinal backup create CONTAINER [-o FILE.tar.gz]` | Разовый backup; контейнер должен быть остановлен |
| `list` / `ls` | `cardinal backup list` | Список архивов в каталоге cardinal backup |
| `restore` | `cardinal backup restore CONTAINER FILE.tar.gz` | Восстановление в остановленный подходящий контейнер |
| `enable` | `cardinal backup enable CONTAINER [OPTIONS]` | Включить расписание |
| `disable` | `cardinal backup disable CONTAINER` | Выключить расписание |
| `status` | `cardinal backup status CONTAINER` | Показать расписание и результат |
| `verify` | `cardinal backup verify FILE.tar.gz` | Проверить архив бэкапа по контрольной сумме |

Флаги `backup enable`:

- `--interval DURATION` — по умолчанию `24h`.
- `--retention N` — по умолчанию `7`, допустимо `1..1000`.
- `--dir PATH` — каталог назначения; защищённые host paths и symlink-компоненты отклоняются.

Backup содержит writable overlay и именованные volumes, но не host bind mounts. Плановый backup ненадолго останавливает работающий контейнер, создаёт согласованный архив и запускает контейнер снова. Включение расписания не создаёт архив немедленно; первый архив появится после указанного интервала. До создания первого архива `backup status` может показывать время инициализации расписания, а не время готового архива. Для продолжения после выхода CLI установите supervisor: `cardinal bootstrap --install`.

Полное руководство по автоматическим бэкапам, ручным бэкапам, восстановлению, скачиванию на локальный компьютер и граничным случаям см. в [Руководстве по бэкапам](backups.md).

## 7. Compose-подобная конфигурация

### `cardinal up [-f FILE] [SERVICE]`

Загрузить `cardinal.toml` или указанный файл, скачать образы, учесть `depends_on` и создать/запустить контейнеры. `--generate` создаёт конфигурацию из существующих именованных контейнеров.

| Флаг | Описание |
|---|---|
| `-f FILE` | Путь к конфигурации |
| `--generate` | Создать конфигурацию; по умолчанию `cardinal.toml` |

```bash
cardinal up
cardinal up -f production.toml api
cardinal up --generate -f generated.toml
```

### `cardinal down [-f FILE] [-a] [SERVICE]`

Удалить контейнеры из конфигурации. `-f` выбирает файл, позиционный service ограничивает действие, `-a` удаляет все контейнеры без чтения конфигурации.

```bash
cardinal down
cardinal down -f production.toml api
cardinal down -a
```

## 8. API, cluster, services и functions

### `cardinal serve [-p PORT] [-H HOST] [-d] [--token TOKEN] [--tls-cert FILE --tls-key FILE]`

Запустить REST API. По умолчанию `127.0.0.1:2375`; `CARDINAL_HOST` может переопределить host/port, `CARDINAL_TOKEN` — передать token. Внешний bind требует token. Для HTTPS передайте одновременно сертификат и приватный ключ; Bearer token всё равно обязателен для внешнего bind.

```bash
cardinal serve
cardinal serve -H 0.0.0.0 -p 2375 --token "$CARDINAL_TOKEN" --tls-cert /etc/cardinal/server.crt --tls-key /etc/cardinal/server.key -d
```

### `cardinal doctor [--strict]` и `cardinal security check [--strict]`

Запустить read-only проверки хоста и runtime. Команды проверяют права каталогов, Linux helpers, namespaces, cgroups, OverlayFS, rootless prerequisites и настройки API. Они не устанавливают пакеты и не запускают/останавливают контейнеры. Код выхода ненулевой при ошибке; `--strict` также считает предупреждения ошибками.

```bash
cardinal doctor
cardinal doctor --strict
cardinal security check
```

```bash
cardinal serve
cardinal serve -H 0.0.0.0 -p 2375 --token "$CARDINAL_TOKEN" -d
```

### `cardinal cluster COMMAND`

| Команда | Синтаксис/флаги |
|---|---|
| `init` | `cardinal cluster init [--name NAME] [--bind ADDR] [--port PORT] [--api-port PORT] [--serve] [--token TOKEN]` |
| `join` | `cardinal cluster join PEER [--bind ADDR] [--port PORT] [--serve] [--token TOKEN]` |
| `join-token` | `cardinal cluster join-token` |
| `leave` | `cardinal cluster leave` |
| `info` | `cardinal cluster info` |
| `ls` / `list` | `cardinal cluster ls` |
| `node ls` / `node list` | `cardinal cluster node ls` |
| `node inspect` | `cardinal cluster node inspect ID` |
| `serve` | `cardinal cluster serve [-p PORT] [-H HOST] [--token TOKEN]` |

Если `--token` не передан, используется `CARDINAL_TOKEN`. Не открывайте API/cluster ports наружу без authentication и firewall.

### `cardinal service COMMAND`

| Команда | Синтаксис |
|---|---|
| `create` | `cardinal service create --name NAME [--replicas N] [-p PORT[:TARGET]] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `cardinal service ls` |
| `rm` / `remove` | `cardinal service rm NAME [NAME ...]` |
| `scale` | `cardinal service scale NAME REPLICAS` |
| `update` | `cardinal service update NAME --image NEW_IMAGE` |

### `cardinal fn COMMAND`

| Команда | Синтаксис |
|---|---|
| `deploy` | `cardinal fn deploy --name NAME [--port N] [--handler PATH] [--timeout SEC] [--idle SEC] [--memory LIMIT] [--cpus N] [--warm N] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `cardinal fn ls` |
| `rm` / `remove` | `cardinal fn rm NAME [NAME ...]` |
| `call` | `cardinal fn call NAME [--data PAYLOAD]` |

Defaults `fn deploy`: port `8080`, handler `/handler`, timeout `30` секунд, idle timeout `300` секунд, warm replicas `0`. В `fn call` флаг `-d` — алиас `--data`.

### `cardinal blueprint COMMAND`

| Команда | Синтаксис |
|---|---|
| `list` / `ls` | `cardinal blueprint list` |
| `info` / `show` | `cardinal blueprint info NAME` |
| `install` / `i` | `cardinal blueprint install NAME [-n NAME] [-d] [--memory LIMIT] [--cpus N] [-e KEY=VALUE] [-y]` |
| `repo list` / `repo ls` | `cardinal blueprint repo list` |
| `repo add` | `cardinal blueprint repo add URL [--name NAME] [--branch BRANCH]` |
| `repo remove` / `repo rm` | `cardinal blueprint repo remove NAME\|URL\|INDEX` |

`blueprint info` принимает полное имя или совпадающий prefix. `-y` пропускает подтверждения установки.

## 9. Системные команды

### `cardinal system prune`

Удалить неиспользуемые контейнеры и образы по правилам runtime.

### `cardinal update [--check]`

Проверить новую версию. Без `--check` (или `-c`) cardinal спросит подтверждение и скачает/заменит бинарник. При наличии checksum обновление проверяется. На скачивание даётся до пяти минут; при ошибке каждый метод загрузки сообщает собственную причину. Если автоматическое скачивание не удалось, установите релиз вручную (см. [Руководство по запуску](running.md), раздел 2).

### `cardinal bootstrap [--install] [--remove]`

Установить/запустить или удалить systemd unit `cardinal-bootstrap.service`. `-i` — алиас `--install`, `-r` — алиас `--remove`. Без action-флага выполняется разовый проход запуска контейнеров.

```bash
cardinal bootstrap --install
systemctl status cardinal-bootstrap
cardinal bootstrap --remove
```

### `cardinal supervisor`

Запустить постоянный supervisor перезапусков и scheduled backups. Обычно запускается systemd; не запускайте второй экземпляр вручную.

### `cardinal version`, `cardinal --version`, `cardinal -v`

Показать установленную версию.

### `cardinal help`, `cardinal --help`, `cardinal -h`

Показать встроенный обзор команд. Отдельные команды выводят usage, если не хватает аргументов.

### Внутренние команды

`cardinal init <container-id> <merged-path>` подготавливает namespace контейнера и вызывается runtime. `cardinal console-serve` обслуживает attach-подключения. Обычно пользователю их запускать не нужно.

## 10. Данные и переменные окружения

- `CARDINAL_DATA_DIR` меняет каталог состояния runtime; для root по умолчанию `/root/.cardinal`.
- `CARDINAL_TOKEN` передаёт authentication для API/cluster, если token-флаг не указан.
- `CARDINAL_HOST` может переопределить host и port REST API.
- State контейнеров, overlay, логи, образы, named volumes, sockets и backups находятся внутри каталога cardinal data.
- Bind mount не копируется в backup контейнера. Host source нужно архивировать отдельно.

Практические рецепты находятся в [Примерах команд](examples.md), установка и troubleshooting — в [Руководстве по запуску](running.md).

## Дополнение командной строки (shell completion)

cardinal использует [spf13/cobra](https://github.com/spf13/cobra), который автоматически
формирует подкоманду `completion` для bash, zsh, fish и PowerShell.

```bash
# bash
cardinal completion bash | sudo tee /etc/bash_completion.d/cardinal > /dev/null
# для текущего пользователя
cardinal completion bash > ~/.local/share/bash-completion/completions/cardinal

# zsh (файл _cardinal нужно положить в любую директорию из $fpath)
cardinal completion zsh > "${fpath[1]}/_cardinal"

# fish
cardinal completion fish > ~/.config/fish/completions/cardinal.fish

# PowerShell
cardinal completion powershell | Out-String | Invoke-Expression
```

После этого работает:

- автодополнение по `Tab` для всех 60+ команд (`cardinal <TAB>` ⇒ `pull  push  run  ...`);
- дополнение флагов для конкретной команды (`cardinal run --<TAB>` ⇒ `--restart-max-attempts --restart-delay ...`);
- дополнение путей файлов для `-v` и `--seccomp-profile`.

## Глобальные флаги

`--json`, `--quiet`, `--log-level` принимаются всеми командами.

| Флаг | По умолчанию | Описание |
|---|---|---|
| `--log-level debug\|info\|warn\|error` | `info` | Минимальный уровень лога, который попадает в stderr. |
| `--json` | off | Логи в формате JSON-lines (для агрегаторов). |
| `--quiet` | off | Эквивалентно `--log-level error`; подавляет информационные сообщения. |
