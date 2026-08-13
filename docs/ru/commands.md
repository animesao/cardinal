<!-- dck-version:start -->
**Documentation version:** `1.23.29`
**Project release:** `v1.23.29`
<!-- dck-version:end -->

# Полный справочник CLI dck

Полный справочник Linux-бинарника `dck`: дерево команд, позиционные аргументы, короткие и длинные флаги, алиасы, значения по умолчанию и правила безопасности.

> **Платформа:** dck работает на Linux и требует namespaces, OverlayFS, cgroups v2, `unshare`, `nsenter`, `mount`, `ip`, `iptables` и `pgrep`.

## 1. Синтаксис и префиксы

```text
dck КОМАНДА [ПОДКОМАНДА] [ОПЦИИ] [ПОЗИЦИОННЫЕ-АРГУМЕНТЫ]
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
| `--` | Конец опций; всё после него — аргументы команды | `dck run alpine sh -c -- 'echo -n hi'` |

Булевы короткие флаги указываются раздельно: используйте `-i -t`, а не объединённую форму `-it`. Значения с пробелами, `$`, `*`, `:` и shell-синтаксисом заключайте в кавычки.

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

### `dck pull [--platform os/arch] IMAGE[:TAG]`

Скачать образ из Docker Hub или настроенного registry. Если tag не указан, используется `latest`. `--platform` выбирает платформу манифеста, например `linux/amd64` или `linux/arm64`.

```bash
dck pull alpine
dck pull alpine:3.20
dck pull --platform linux/arm64 eclipse-temurin:21
```

### `dck push IMAGE[:TAG] [-u USER] [-p PASSWORD]`

Отправить локальный образ в registry. Флаги учётных данных — `-u` и `-p`; для автоматизации лучше использовать `dck login` или переменные окружения.

```bash
dck push myapp:v1
dck push -u registry-user -p 'secret' registry.example.com/team/myapp:v1
```

### `dck images`

Показать локальные образы.

```bash
dck images
```

### `dck search TERM`

Искать образы на Docker Hub. После двоеточия можно указать фильтр тегов.

```bash
dck search nginx
dck search python:3.12
```

### `dck rmi IMAGE[:TAG]`

Удалить локальный образ. По умолчанию используется tag `latest`.

```bash
dck rmi alpine:3.20
```

### `dck verify IMAGE[:TAG]`

Проверить config и диджесты слоёв локального образа по сохранённым манифестам.

```bash
dck verify alpine:3.20
```

### `dck commit CONTAINER IMAGE[:TAG]`

Создать образ из текущего изменяемого состояния контейнера.

```bash
dck commit web registry.example.com/team/web:snapshot
```

### `dck build -t NAME[:TAG] [OPTIONS] [CONTEXT]`

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
dck build -t myapp:dev .
dck build -t myapp:prod -f Dockerfile.prod --build-arg VERSION=1.0 ./src
```

### `dck export IMAGE[:TAG] [-o FILE.tar.gz]`

Экспортировать образ в архив. `-o` задаёт путь результата.

```bash
dck export myapp:v1 -o /data/images/myapp-v1.tar.gz
```

### `dck import FILE.tar.gz [FILE.tar.gz ...]`

Импортировать один или несколько архивов образов.

```bash
dck import /data/images/myapp-v1.tar.gz
```

### `dck login REGISTRY [-u USER] [-p PASSWORD] [--password-stdin]`

Сохранить данные registry. Если данные не указаны, dck запросит их интерактивно. `--password-stdin` читает пароль из stdin.

```bash
dck login registry.example.com
echo "$REGISTRY_PASSWORD" | dck login registry.example.com -u "$REGISTRY_USER" --password-stdin
```

### `dck logout REGISTRY [REGISTRY ...]`

Удалить сохранённые данные registry.

```bash
dck logout registry.example.com
```

## 3. Жизненный цикл контейнера

### `dck run [OPTIONS] IMAGE [COMMAND ...]`

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

Примеры:

```bash
dck run --rm alpine echo hello
dck run -d -n web -p 8080:80 nginx:alpine
dck run -d --name app --image python:3.12 --cmd 'python /app/main.py'
dck run -i -t --rm alpine sh
dck run -d --restart unless-stopped --restart-delay 1m --memory 4g --cpus 2 myapp:latest
```

Политики автоматического перезапуска обслуживает `dck-bootstrap.service`. Контейнер `unless-stopped`, остановленный вручную, не запускается при boot recovery; `always` запускается после перезагрузки. Сервис устанавливается автоматически, если это возможно для root, либо явно командой `dck bootstrap --install`.

Частые быстрые падения защищены: когда исчерпан бюджет crash-loop (по умолчанию 5 перезапусков, `--restart-max-attempts` и `--restart-window`), автоматический перезапуск блокируется, и контейнер остаётся остановленным до явного `dck start`.

Host bind source должен существовать и не должен находиться в защищённых системных путях `/root`, `/etc`, `/var`, `/usr`, `/opt` или `/run`. Используйте каталог данных `/data/myapp` либо именованный volume:

```bash
mkdir -p /data/myapp
dck run -d -v /data/myapp:/app myapp:latest
```

### `dck ps [-a]`

Показать контейнеры. `-a` добавляет остановленные и созданные контейнеры.

```bash
dck ps
dck ps -a
```

### `dck inspect [--sensitive] CONTAINER [CONTAINER ...]`

Вывести состояние контейнера в JSON. Чувствительные поля скрыты, если не указать `--sensitive`.

```bash
dck inspect web
dck inspect --sensitive web
```

### `dck start CONTAINER`

Запустить остановленные контейнеры, сохраняя overlay и volumes.

### `dck stop [--all] CONTAINER`

Остановить контейнеры. `--all` останавливает все работающие контейнеры.

### `dck restart CONTAINER`

Перезапустить контейнер: stop + start.

### `dck rm [-f] CONTAINER`

Удалить контейнер. `-f` принудительно удаляет работающий контейнер. Изменяемый overlay удаляется, именованные volumes сохраняются.

### `dck rename CONTAINER NEW_NAME`

Переименовать контейнер.

### `dck set CONTAINER [OPTIONS]`

Изменить параметры без удаления контейнера. Если контейнер работал, dck остановит и снова запустит его.

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

```bash
dck set minecraft --restart unless-stopped --restart-delay 1m
dck set web --memory 2g --cpus 2
```

## 4. Сетевые команды

### `dck network create [--subnet CIDR] NAME`

Создать пользовательскую Linux bridge-сеть. Если `--subnet` не указан, dck выберет свободную private-сеть `/24`.

```bash
dck network create --subnet 10.20.0.0/24 appnet
dck network ls
dck network inspect appnet
dck run -d --network appnet alpine sleep infinity
dck network rm appnet
```

`network rm` откажет, пока сеть используется и IP-адреса заняты. Сначала удалите контейнеры сети. Пользовательский bridge требует root или `CAP_NET_ADMIN`, а также `ip`/`iptables`.

### `dck network ls|list`

Показать пользовательские bridge-сети. Встроенная сеть `dck0` сюда не входит.

### `dck network inspect NAME`

Показать ID, driver, subnet, gateway, bridge-интерфейс и число занятых IP.

### `dck network rm|remove NAME`

Удалить неиспользуемую пользовательскую сеть и её firewall-правила.

## 5. Логи, мониторинг, выполнение и файлы

### `dck logs [-f] [--tail N] [--previous] [--all] CONTAINER`

Показать stdout/stderr контейнера. `-f` следит за текущим выводом, `--tail` ограничивает текущий вывод, `--previous` показывает предыдущий повернутый запуск, `--all` показывает текущий и повернутые логи. При каждом новом запуске dck создаёт свежий файл лога.

```bash
dck logs web
dck logs --tail 100 web
dck logs -f web
dck logs --previous web
dck logs --all web
```

### `dck attach CONTAINER`

Подключиться к главному процессу через Unix console socket. Контейнер должен работать. `Ctrl+C` безопасен: отсоединение не останавливает контейнер.

### `dck exec [-i] [-t] CONTAINER COMMAND [ARGS ...]`

Запустить новый процесс внутри работающего контейнера.

```bash
dck exec web nginx -s reload
dck exec -i -t web /bin/sh
```

### `dck console CONTAINER`

Открыть интерактивную оболочку: сначала Bash, затем `/bin/sh`.

### `dck console-serve ...`

Внутренний сервер консоли для attach; обычно вручную не запускается.

### `dck cp SRC DST`

Копировать между хостом и контейнером. Для endpoint контейнера используется `CONTAINER:/path`; копирование контейнер-контейнер не поддерживается.

```bash
dck cp ./config.yml web:/etc/app/config.yml
dck cp web:/etc/app/config.yml ./backup/
```

### `dck fs ls|cat|tree|find ...`

Просматривать merged filesystem работающего или остановленного контейнера.

```text
dck fs ls CONTAINER [PATH]
dck fs cat CONTAINER PATH
dck fs tree CONTAINER [PATH]
dck fs find CONTAINER [PATH] [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
dck fs find [--name PATTERN] [--grep TEXT] [--type f|d] [--max-depth N]
```

Последняя форма ищет во всех контейнерах. `--name` фильтрует по подстроке имени, `--grep` ищет содержимое файлов, `--type` выбирает файлы или каталоги, `--max-depth` ограничивает глубину.

### `dck stats [CONTAINER] [--no-stream]`

Показать статистику cgroups v2: CPU, память, I/O и процессы. `--no-stream` выводит один снимок и завершает работу.

### `dck top CONTAINER`

Показать процессы внутри контейнера.

### `dck info`

Показать сведения о хосте, runtime, storage, CPU, памяти, диске и контейнерах.

### `dck events [--since TIME]`

Поток событий контейнеров в JSON. `--since` принимает RFC3339 или `YYYY-MM-DD HH:MM:SS`.

## 6. Порты, volumes и backup

### `dck port CONTAINER`

Показать проброшенные порты.

### `dck port add CONTAINER HOST:CONTAINER[/PROTO]`

Добавить TCP (по умолчанию) или UDP mapping без пересоздания контейнера.

### `dck port remove|rm CONTAINER HOST[/PROTO]`

Удалить dynamic mapping. `rm` — алиас `remove`.

### `dck volume create [OPTIONS] [NAME]`

Создать именованный local volume. Если имя не указано, dck сгенерирует его.

| Флаг | Описание |
|---|---|
| `-d DRIVER` | Драйвер; по умолчанию `local` |
| `-l`, `--label KEY=VALUE` | Повторяемая метка |

```bash
dck volume create app-data
dck volume create -l env=prod app-data
```

### `dck volume ls|list`

Список именованных volumes.

### `dck volume inspect NAME [NAME ...]`

Показать драйвер, mountpoint, дату создания и labels.

### `dck volume rm|remove NAME [NAME ...]`

Удалить volumes. Операция необратима.

### `dck volume prune`

Удалить local volumes, на которые не ссылается ни один контейнер.

### `dck backup COMMAND`

| Команда | Синтаксис | Описание |
|---|---|---|
| `create` | `dck backup create CONTAINER [-o FILE.tar.gz]` | Разовый backup; контейнер должен быть остановлен |
| `list` / `ls` | `dck backup list` | Список архивов в каталоге dck backup |
| `restore` | `dck backup restore CONTAINER FILE.tar.gz` | Восстановление в остановленный подходящий контейнер |
| `enable` | `dck backup enable CONTAINER [OPTIONS]` | Включить расписание |
| `disable` | `dck backup disable CONTAINER` | Выключить расписание |
| `status` | `dck backup status CONTAINER` | Показать расписание и результат |
| `verify` | `dck backup verify FILE.tar.gz` | Проверить архив бэкапа по контрольной сумме |

Флаги `backup enable`:

- `--interval DURATION` — по умолчанию `24h`.
- `--retention N` — по умолчанию `7`, допустимо `1..1000`.
- `--dir PATH` — каталог назначения; защищённые host paths и symlink-компоненты отклоняются.

Backup содержит writable overlay и именованные volumes, но не host bind mounts. Плановый backup ненадолго останавливает работающий контейнер, создаёт согласованный архив и запускает контейнер снова. Включение расписания не создаёт архив немедленно; первый архив появится после указанного интервала. До создания первого архива `backup status` может показывать время инициализации расписания, а не время готового архива. Для продолжения после выхода CLI установите supervisor: `dck bootstrap --install`.

Полное руководство по автоматическим бэкапам, ручным бэкапам, восстановлению, скачиванию на локальный компьютер и граничным случаям см. в [Руководстве по бэкапам](backups.md).

## 7. Compose-подобная конфигурация

### `dck up [-f FILE] [SERVICE]`

Загрузить `dck.toml` или указанный файл, скачать образы, учесть `depends_on` и создать/запустить контейнеры. `--generate` создаёт конфигурацию из существующих именованных контейнеров.

| Флаг | Описание |
|---|---|
| `-f FILE` | Путь к конфигурации |
| `--generate` | Создать конфигурацию; по умолчанию `dck.toml` |

```bash
dck up
dck up -f production.toml api
dck up --generate -f generated.toml
```

### `dck down [-f FILE] [-a] [SERVICE]`

Удалить контейнеры из конфигурации. `-f` выбирает файл, позиционный service ограничивает действие, `-a` удаляет все контейнеры без чтения конфигурации.

```bash
dck down
dck down -f production.toml api
dck down -a
```

## 8. API, cluster, services и functions

### `dck serve [-p PORT] [-H HOST] [-d] [--token TOKEN] [--tls-cert FILE --tls-key FILE]`

Запустить REST API. По умолчанию `127.0.0.1:2375`; `DCK_HOST` может переопределить host/port, `DCK_TOKEN` — передать token. Внешний bind требует token. Для HTTPS передайте одновременно сертификат и приватный ключ; Bearer token всё равно обязателен для внешнего bind.

```bash
dck serve
dck serve -H 0.0.0.0 -p 2375 --token "$DCK_TOKEN" --tls-cert /etc/dck/server.crt --tls-key /etc/dck/server.key -d
```

### `dck doctor [--strict]` и `dck security check [--strict]`

Запустить read-only проверки хоста и runtime. Команды проверяют права каталогов, Linux helpers, namespaces, cgroups, OverlayFS, rootless prerequisites и настройки API. Они не устанавливают пакеты и не запускают/останавливают контейнеры. Код выхода ненулевой при ошибке; `--strict` также считает предупреждения ошибками.

```bash
dck doctor
dck doctor --strict
dck security check
```

```bash
dck serve
dck serve -H 0.0.0.0 -p 2375 --token "$DCK_TOKEN" -d
```

### `dck cluster COMMAND`

| Команда | Синтаксис/флаги |
|---|---|
| `init` | `dck cluster init [--name NAME] [--bind ADDR] [--port PORT] [--api-port PORT] [--serve] [--token TOKEN]` |
| `join` | `dck cluster join PEER [--bind ADDR] [--port PORT] [--serve] [--token TOKEN]` |
| `join-token` | `dck cluster join-token` |
| `leave` | `dck cluster leave` |
| `info` | `dck cluster info` |
| `ls` / `list` | `dck cluster ls` |
| `node ls` / `node list` | `dck cluster node ls` |
| `node inspect` | `dck cluster node inspect ID` |
| `serve` | `dck cluster serve [-p PORT] [-H HOST] [--token TOKEN]` |

Если `--token` не передан, используется `DCK_TOKEN`. Не открывайте API/cluster ports наружу без authentication и firewall.

### `dck service COMMAND`

| Команда | Синтаксис |
|---|---|
| `create` | `dck service create --name NAME [--replicas N] [-p PORT[:TARGET]] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `dck service ls` |
| `rm` / `remove` | `dck service rm NAME [NAME ...]` |
| `scale` | `dck service scale NAME REPLICAS` |
| `update` | `dck service update NAME --image NEW_IMAGE` |

### `dck fn COMMAND`

| Команда | Синтаксис |
|---|---|
| `deploy` | `dck fn deploy --name NAME [--port N] [--handler PATH] [--timeout SEC] [--idle SEC] [--memory LIMIT] [--cpus N] [--warm N] [-e KEY=VALUE] IMAGE` |
| `ls` / `list` | `dck fn ls` |
| `rm` / `remove` | `dck fn rm NAME [NAME ...]` |
| `call` | `dck fn call NAME [--data PAYLOAD]` |

Defaults `fn deploy`: port `8080`, handler `/handler`, timeout `30` секунд, idle timeout `300` секунд, warm replicas `0`. В `fn call` флаг `-d` — алиас `--data`.

### `dck blueprint COMMAND`

| Команда | Синтаксис |
|---|---|
| `list` / `ls` | `dck blueprint list` |
| `info` / `show` | `dck blueprint info NAME` |
| `install` / `i` | `dck blueprint install NAME [-n NAME] [-d] [--memory LIMIT] [--cpus N] [-e KEY=VALUE] [-y]` |
| `repo list` / `repo ls` | `dck blueprint repo list` |
| `repo add` | `dck blueprint repo add URL [--name NAME] [--branch BRANCH]` |
| `repo remove` / `repo rm` | `dck blueprint repo remove NAME\|URL\|INDEX` |

`blueprint info` принимает полное имя или совпадающий prefix. `-y` пропускает подтверждения установки.

## 9. Системные команды

### `dck system prune`

Удалить неиспользуемые контейнеры и образы по правилам runtime.

### `dck update [--check]`

Проверить новую версию. Без `--check` (или `-c`) dck спросит подтверждение и скачает/заменит бинарник. При наличии checksum обновление проверяется. На скачивание даётся до пяти минут; при ошибке каждый метод загрузки сообщает собственную причину. Если автоматическое скачивание не удалось, установите релиз вручную (см. [Руководство по запуску](running.md), раздел 2).

### `dck bootstrap [--install] [--remove]`

Установить/запустить или удалить systemd unit `dck-bootstrap.service`. `-i` — алиас `--install`, `-r` — алиас `--remove`. Без action-флага выполняется разовый проход запуска контейнеров.

```bash
dck bootstrap --install
systemctl status dck-bootstrap
dck bootstrap --remove
```

### `dck supervisor`

Запустить постоянный supervisor перезапусков и scheduled backups. Обычно запускается systemd; не запускайте второй экземпляр вручную.

### `dck version`, `dck --version`, `dck -v`

Показать установленную версию.

### `dck help`, `dck --help`, `dck -h`

Показать встроенный обзор команд. Отдельные команды выводят usage, если не хватает аргументов.

### Внутренние команды

`dck init <container-id> <merged-path>` подготавливает namespace контейнера и вызывается runtime. `dck console-serve` обслуживает attach-подключения. Обычно пользователю их запускать не нужно.

## 10. Данные и переменные окружения

- `DCK_DATA_DIR` меняет каталог состояния runtime; для root по умолчанию `/root/.dck`.
- `DCK_TOKEN` передаёт authentication для API/cluster, если token-флаг не указан.
- `DCK_HOST` может переопределить host и port REST API.
- State контейнеров, overlay, логи, образы, named volumes, sockets и backups находятся внутри каталога dck data.
- Bind mount не копируется в backup контейнера. Host source нужно архивировать отдельно.

Практические рецепты находятся в [Примерах команд](examples.md), установка и troubleshooting — в [Руководстве по запуску](running.md).
