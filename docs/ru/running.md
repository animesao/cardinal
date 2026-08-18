<!-- dck-version:start -->
**Documentation version:** `1.25.7`
**Project release:** `v1.25.7`
<!-- dck-version:end -->

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

### Универсальный установщик (все дистрибутивы)

Скрипт автоматически определяет дистрибутив и устанавливает dck + зависимости:

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

Поддерживаемые дистрибутивы: **Ubuntu, Debian, Arch, Manjaro, Fedora, RHEL, CentOS, Rocky, Alma, openSUSE, Alpine, Void Linux** и другие.

### Установка для конкретных дистрибутивов

**Arch Linux / Manjaro (AUR):**

```bash
# Через AUR-хелпер (yay/paru)
yay -S dck
# Или из исходников
git clone https://aur.archlinux.org/dck.git
cd dck
makepkg -si
```

**Fedora / RHEL / CentOS:**

Скачайте и установите последний RPM asset со [страницы GitHub-релиза](https://github.com/animesao/dck/releases/latest):

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Не удалось определить последний релиз" >&2; exit 1; }
VERSION="${TAG#v}"
FILE="dck-${VERSION}-linux-amd64.rpm"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo dnf install "./$FILE"
# Для старых систем:
# sudo rpm -Uvh "./$FILE"
```

**Debian / Ubuntu (.deb):**

Скачайте и установите последний DEB asset со [страницы GitHub-релиза](https://github.com/animesao/dck/releases/latest):

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Не удалось определить последний релиз" >&2; exit 1; }
VERSION="${TAG#v}"
FILE="dck-${VERSION}-linux-amd64.deb"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo apt install "./$FILE"
```

**Snap-пакет (из GitHub Releases):**

Snap-пакеты собираются для каждого релиза и прикладываются к GitHub Releases,
но автоматически в Snap Store не загружаются. Скачайте и установите версионный
asset `.snap` напрямую:

```bash
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"\]*\)".*/\1/p')"
test -n "$TAG" || { echo "Не удалось определить последний релиз" >&2; exit 1; }
FILE="dck-${TAG#v}-linux-amd64.snap"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
sudo snap install --dangerous --classic "$FILE"
```

На ARM64 используйте суффикс `arm64`. Snap использует classic confinement,
потому что dck требует namespace, mount, cgroup и сетевые возможности хоста.

**Ручная установка бинарника:**

```bash
curl -fsSL https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64 -o /tmp/dck-new
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
dck bootstrap --install
```

**AppImage (amd64 и arm64):**

AppImage — это самодостаточный исполняемый формат. Скачайте подходящий asset
из [последнего GitHub-релиза](https://github.com/animesao/dck/releases/latest),
добавьте право на запуск и запустите файл напрямую:

```bash
# x86_64 / amd64
TAG="$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
test -n "$TAG" || { echo "Не удалось определить последний релиз" >&2; exit 1; }
FILE="dck-${TAG#v}-linux-amd64.AppImage"
curl -fL -o "$FILE" "https://github.com/animesao/dck/releases/download/$TAG/$FILE"
chmod +x "$FILE"
"./$FILE" version

# Установить встроенный бинарник и включить supervisor
"./$FILE" --install
```

Для ARM64 используйте matching `dck-*-linux-arm64.AppImage` asset. AppImage не
требует пакетного менеджера, но dck всё равно требует от хоста Linux
namespace, cgroups, OverlayFS, mount и сетевые возможности. Стандартный
AppImage для ARMv6 не публикуется: официальный runtime поддерживает
x86_64, aarch64 и armhf, но не настоящий ARMv6. Пользователям ARMv6 нужно
использовать бинарник `dck-linux-armv6` или архив `.tar.gz`.

#### Установка двойным кликом на Linux Desktop

AppImage dck — это CLI-runtime, а не графическое приложение. При двойном
клике по AppImage в файловом менеджере он откроет доступный терминал и
запустит desktop-установщик. Установщик извлечёт встроенный статический
бинарник dck в `/usr/local/bin/dck`, при необходимости запросит права
администратора и установит/запустит `dck-bootstrap.service`, если доступен
systemd. Существующие контейнеры и образы в каталоге данных dck сохраняются,
а исходный AppImage остаётся переносимым.

Тот же установщик можно запустить из терминала. Используйте имя скачанного AppImage:

```bash
APPIMAGE="$(find . -maxdepth 1 -type f -name 'dck-*-linux-amd64.AppImage' -print -quit)"
test -n "$APPIMAGE" || { echo "AppImage не найден в текущем каталоге" >&2; exit 1; }
chmod +x "$APPIMAGE"
"$APPIMAGE" --install
```

Чтобы использовать AppImage только как переносимый CLI, передайте обычную
команду dck. Этот блок сам найдёт скачанный файл в текущем каталоге:

```bash
APPIMAGE="$(find . -maxdepth 1 -type f -name 'dck-*-linux-amd64.AppImage' -print -quit)"
test -n "$APPIMAGE" || { echo "AppImage не найден в текущем каталоге" >&2; exit 1; }
chmod +x "$APPIMAGE"
"$APPIMAGE" version
"$APPIMAGE" run --rm --network none alpine:latest echo OK
```

Если двойной клик ничего не делает, добавьте файлу право на запуск и выберите
**Run/Запустить**, а не **Display/Показать** в файловом менеджере. В Desktop
должен быть установлен эмулятор терминала (`x-terminal-emulator`, GNOME
Terminal, Konsole, XFCE Terminal или MATE Terminal). На headless VPS используйте
команды из терминала. Если старый AppImage сообщает `cannot stat ... Permission
denied` на этапе `sudo install`, скачайте новый AppImage: старые сборки пытались
читать бинарник из FUSE mount от root вместо предварительного копирования в
`/tmp`.

### Проверка

```bash
dck version
dck info
```

Обновление установленной версии:

```bash
dck update --check
dck update
```

Если dck запущен из AppImage, изменить read-only mount самого AppImage
невозможно. Теперь updater устанавливает проверенный новый статический
бинарник в `/usr/local/bin/dck`, а исходный AppImage оставляет без изменений.
Если обычному пользователю недоступна запись в этот каталог, updater запросит
`sudo`.

После обновления проверьте бинарник и запустите временный контейнер:

```bash
dck version
dck run --rm alpine:latest echo "DCK UPDATE OK"
```

Если `dck update` не может скачать бинарник (в старых версиях — ошибка `Failed to download binary: all methods failed`), установите релиз вручную. Подставьте нужную версию и архитектуру:

```bash
curl -fsSL --connect-timeout 10 -o /tmp/dck-new \
  https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
sudo systemctl restart dck-bootstrap   # если установлен systemd supervisor
```

Имена бинарников: `dck-linux-amd64`, `dck-linux-arm64`, `dck-linux-armv6`. В релизах также будут нативные пакеты каждой поддерживаемой архитектуры: `.deb`, `.rpm`, `.pkg.tar.zst` и `.apk`, где в имени указано `amd64`, `arm64` или `armv6`. AppImage публикуется для `amd64` и `arm64`; для ARMv6 используйте бинарник или архив `.tar.gz`. Выбирайте пакет одновременно по дистрибутиву и архитектуре CPU. Если GitHub недоступен, скачайте asset через другую доверенную сеть или передайте его по SSH; не используйте непроверенные сторонние зеркала для release-бинарников. Здесь `${VERSION}` означает номер тега без начальной `v` (например, `1.23.17`).

## 3. Загрузка и запуск образа

Образ можно передать позиционно. Флаг `--image` тоже поддерживается, а `--images` — нет.

```bash
dck pull alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

Для workflow pull → verify → run проверяйте целостность образа после скачивания и перед запуском. `dck verify` сверяет config и диджесты всех слоёв с сохранённым манифестом локально, без обращения к registry:

```bash
dck pull alpine:latest
dck verify alpine:latest
dck run --rm alpine:latest echo "hello from dck"
```

`dck verify` завершится с ненулевым кодом, если образ отсутствует локально, диджест config не совпадает с сохранённой мета-информацией или манифестом, либо повреждён какой-либо файл слоя. Это быстрая офлайн-проверка для образов, восстановленных через `dck import` или перенесённых между хостами.

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

Частые быстрые падения защищены: когда исчерпан бюджет crash-loop (по умолчанию 5 перезапусков; настраивается через `--restart-max-attempts` и `--restart-window`), автоматический перезапуск блокируется — `dck inspect ИМЯ` покажет `"restart_blocked": true` — и контейнер остаётся остановленным до явного `dck start`.

## 5. Bind mount и именованные тома

Формат volume: `источник:путь_в_контейнере`. Путь внутри контейнера должен быть абсолютным:

```bash
--vol /data/myapp:/app
--vol myapp_data:/var/lib/myapp
```

Добавьте `:ro` или `:rw`, чтобы смонтировать том только на чтение или на запись (по умолчанию — на запись), либо режим propagation, например `:shared`/`:rshared`:

```bash
--vol /data/myapp:/app:ro
--vol /data/config:/etc/app:shared
```

Также поддерживаются спецификации `tmpfs:` (в памяти) и `nfs://сервер:/export:/путь/в/контейнере`.

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

Для каждого контейнера можно включить собственное расписание. Архив содержит writable overlay и именованные тома, но не host bind mount. Каталоги вроде `/data/minecraft` нужно архивировать отдельно. Чтобы данные были согласованными, dck ненадолго остановит работающий контейнер, создаст архив и запустит его снова. При включении расписания архив сразу не создаётся: первый архив появится после заданного интервала.

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

Один раз установите постоянный supervisor, чтобы расписание продолжало работать после выхода из терминала и после перезагрузки. При ошибке бэкапа supervisor сохраняет время следующей попытки и не запускает бесконечный цикл повторов каждую секунду:

```bash
dck bootstrap --install
systemctl status dck-bootstrap
```

Разовый бэкап можно создать командой `dck backup create ИМЯ`; перед этим остановите контейнер. Восстанавливать архив можно только в остановленный контейнер. Ручные и автоматические архивы включают данные overlay и именованных томов dck, но не host bind mount:

```bash
dck backup restore minecraft /data/backups/minecraft/minecraft-20260811-120000.tar.gz
```

Настройки авто-бэкапа хранятся в state контейнера и переживают `stop`, `start` и обновление dck. Retention удаляет самые старые архивы расписания после успешного бэкапа; файлы с другими именами он не трогает.

Архив можно проверить по контрольной сумме: `dck backup verify ФАЙЛ.tar.gz`. Если checksum-файла рядом нет, dck сообщит, что архив валиден, но не проверен.

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

Используйте `--network none`, если приложению не нужна сеть, и `--network host` только осознанно — этот режим делит сетевое пространство хоста. Контейнеры с `--network none` и `--network host` запускаются без ожидания интерфейса; в bridge-режиме ожидание появления veth-интерфейса ограничено пятью секундами.

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

### `Failed to download binary: all methods failed` при `dck update`

Раньше обновление завершалось таймаутом через десять секунд — этого мало для бинарника в несколько мегабайт на медленном канале. Установите релиз вручную:

```bash
curl -fsSL --connect-timeout 10 -o /tmp/dck-new \
  https://github.com/animesao/dck/releases/latest/download/dck-linux-amd64
sudo install -D -m 0755 /tmp/dck-new /usr/local/bin/dck
rm -f /tmp/dck-new
sudo systemctl restart dck-bootstrap
```

### Контейнер остаётся `running` после завершения процесса

Старые версии считали zombie-процессы (defunct) живыми, поэтому supervisor не замечал выход — контейнер застревал в `running`, рестарты почти не срабатывали, а ресурсы не очищались. Обновите dck и проверьте:

```bash
dck version
dck ps -a
dck inspect ИМЯ | grep -E '"status"|"pid"'
```

### Контейнер `--network none` «висит» перед запуском команды

При `--network none` интерфейс `eth0` не должен появляться, поэтому dck не ждёт его. При `--network host` уже используется интерфейс хоста, а в bridge-режиме dck недолго ждёт появления veth. Если запуск всё равно зависает, обновите dck и проверьте логи контейнера.

## 17. Где хранятся данные

Для root стандартный каталог данных dck — `/root/.dck`. Точный путь также показывает `dck info`:

```text
/root/.dck/
├── images/       скачанные rootfs образов
├── containers/   JSON-состояние контейнеров
├── overlay/      записываемые слои контейнеров
├── logs/         stdout/stderr dck
├── volumes/      именованные тома
├── cache/        кэш слоёв образов
├── consoles/     socket-файлы для attach
└── backups/      архивы автоматических бэкапов
```

Чтобы использовать другое место для внутреннего состояния, задайте `DCK_DATA_DIR` до запуска dck. Bind mount проекта, например `/data/alfheimguide`, хранится отдельно от этого внутреннего каталога.
