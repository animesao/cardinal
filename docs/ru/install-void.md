<!-- cardinal-version:start -->
**Documentation version:** `2.0.6`
**Project release:** `v2.0.6`
<!-- cardinal-version:end -->

# Установка cardinal в Void Linux

Void использует **xbps** (`xbps-install` и семейство) и
поддерживаемое сообществом дерево исходников
[`void-packages`](https://github.com/void-linux/void-packages).
Канонический drop-in для `cardinal` лежит в каталоге
[`contrib/void/`](../../contrib/void/); вы копируете файл
`template` в свой локальный форк `void-packages` и собираете там.

## Быстрый путь: локальный форк void-packages

```bash
# 1. Форк void-packages, если его нет:
git clone https://github.com/void-linux/void-packages
cd void-packages

# 2. template на каноническое место для xbps-src
mkdir -p srcpkgs/cardinal
cp path/to/cardinal/contrib/void/template srcpkgs/cardinal/template

# 3. Пересчитать SHA distfile (заполнит строку `checksum=` в template)
./xbps-src update-sums cardinal

# 4. Собрать (зависимости подтянутся из дерева void-packages)
./xbps-src pkg cardinal
#   Получится: hostdir/binpkgs/cardinal-1.24.15_1.x86_64.xbps

# 5. (Локально) установить пакет
sudo xbps-install --repository=hostdir/binpkgs cardinal

# 6. Проверка
cardinal version
sudo cardinal doctor
```

## Важно: подменить SHA256-placeholder

`template` идёт с `checksum="sha256-PLACEHOLDER-..."` — при первом
`./xbps-src pkg cardinal` он корректно сломается с «bad checksum», а
`./xbps-src update-sums cardinal` сам исправит строку на корректный
`sha256-...` digest. Не править руками — всегда через `update-sums`
(учитывает tarball и вендорные проверки), чтобы вы ничего не
пропустили.

## Сборка под musl

Void выпускает и glibc, и musl. cardinal — pure-Go бинарник, работает
на обоих, но нужно выбрать архитектуру:

```bash
# glibc (по умолчанию на x86_64)
./xbps-src pkg cardinal

# musl
./xbps-src -a x86_64-musl pkg cardinal
```

Поскольку в upstream зашито `CGO_ENABLED=0`, один и тот же `.xbps`
работает на любой libc без перекомпиляции.

## Требования к ядру

```bash
cardinal doctor
```

Если `cardinal doctor` показывает `WARN` на `user namespaces`:

```
# /boot/grub/grub.cfg добавить в строку linux:
GRUB_CMDLINE_LINUX_DEFAULT="lsm=capability,landlock ... module.sig_enforce=1"
# Дефолтное vmlinuz в Void содержит CONFIG_USER_NS=y с 2020.
```

Если `cgroups v2` показывает `WARN`, примонтируйте как default:

```
# /etc/fstab
none /sys/fs/cgroup cgroup2 defaults 0 0
```

## Отправить в void-packages

Когда `./xbps-src pkg cardinal` собирается зелёным на матрицах
`x86_64-glibc`, `x86_64-musl` и `aarch64-*`:

1. Форкните https://github.com/void-linux/void-packages
2. Сначала отправьте `maintainers.md` (однострочный PR о
   вступлении в maintainers)
3. Затем отправьте `srcpkgs/cardinal/template` с посчитанным
   checksum'ом

Строка `maintainers.md`:

```
animesao <animesao@users.noreply.github.com> cardinal
```

## Редкий случай: pivot_root в контейнерах на Void

`runit` не нуждается в полноценном init-демоне — `cardinal run` прозрачно
запускает entrypoint и работает на Void. Если в контейнере вылетит
pivot_root, используйте флаг `--no-pivot` в `cmd/run` — он
переключает на `chroot`-семантику, поддерживаемую в Void из коробки.

## См. также

- `docs/en/install-void.md` — полный walkthrough
- `contrib/void/README.md` — чек-лист для PR
