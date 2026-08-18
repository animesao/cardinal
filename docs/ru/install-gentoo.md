<!-- dck-version:start -->
**Documentation version:** `1.60.11`
**Project release:** `v1.60.11`
<!-- dck-version:end -->

# Установка dck в Gentoo / Funtoo / Calculate

Канонический ebuild лежит в каталоге
[`contrib/gentoo/`](../../contrib/gentoo/). Два варианта:

1. Содержать `dck` в **локальном overlay**, который вы поддерживаете
   руками.
2. Открыть PR в https://github.com/gentoo/guru (целевая категория:
   `app-containers/dck`), чтобы пакет стал общедоступным.

## Быстрый путь: локальный overlay

```bash
# Создать или переиспользовать локальный overlay root
if [ ! -d /var/db/repos/dck-overlay ]; then
    sudo install -d -o root -g portage \
        /var/db/repos/dck-overlay/{profiles,metadata,app-containers/dck}
    echo 'dck-overlay' | sudo tee /var/db/repos/dck-overlay/profiles/repo_name
    cat <<'EOF' | sudo tee /var/db/repos/dck-overlay/metadata/layout.conf
masters = gentoo
EOF
fi

# Положить ebuild в стандартную категорию
sudo cp contrib/gentoo/dck-1.24.15.ebuild \
        /var/db/repos/dck-overlay/app-containers/dck/

# Подключить overlay через repos.conf
repos_conf="/etc/portage/repos.conf/dck-overlay.conf"
if [ ! -f "$repos_conf" ]; then
    sudo tee "$repos_conf" >/dev/null <<'EOF'
[dck-overlay]
location = /var/db/repos/dck-overlay
priority = 50
EOF
fi

# Сгенерировать SHA256-манифест
sudo chown -R portage:portage /var/db/repos/dck-overlay
cd /var/db/repos/dck-overlay/app-containers/dck
sudo Manifest-md5 dck-1.24.15.ebuild || \
sudo Manifest-sha256 dck-1.24.15.ebuild

# Поставить
sudo emerge --sync
sudo emerge --ask app-containers/dck
```

Если Portage ругается на `~amd64`, добавьте в
`/etc/portage/package.accept_keywords/`:

```
=app-containers/dck-1.24.15 ~amd64
```

## USE-флаги / зависимости

USE-флагов в ebuild нет. Внутри:

- `>=dev-lang/go-1.23` (соответствует `go.mod`)
- Ядро ≥ 4.18 с user namespaces и cgroup v2 (проверяется через `dck doctor`)
- `sys-apps/util-linux` (для `mount`, `unshare`)

Опционально (рекомендуется):

```
sys-apps/iproute2   # для сетевых плагинов (bridge / iptables)
```

## Отправить в ::guru

Если предпочитаете community-overlay, готовьте PR в
`gentoo/guru`. Стандартная структура:

```
app-containers/dck/
├── dck-1.24.15.ebuild
├── dck-9999.ebuild          # для git-версии в реальном времени
└── files/
    └── dck.initd            # (сейчас пустой стаб)
```

В нашем ebuild уже подготовлены места под `newinitd` / `newconfd` —
если захотите добавить systemd/OpenRC supervisor, дополните их.

Описание PR, которое проходит быстро:

> Добавляет `app-containers/dck` в ::guru. dck — Go 1.23+ чисто-Go
> бинарник, повторяющий docker CLI, без демона. Ebuild использует
> `go-module.eclass` и работает с upstream-`vendor/`. Тесты
> RESTRICT-ятся, потому что им нужны namespaces и root.

## Проверка после установки

```bash
$ which dck
/usr/bin/dck
$ dck version
dck version 1.24.15 (commit 52ba511, ...)
$ sudo dck doctor
... отчёт о возможностях ядра ...
```

Если `dck doctor` ругается на `user namespaces`, проверьте ядро:

```
CONFIG_USER_NS=y
CONFIG_USER_NS_FASYNC=y
```

(Обычно `=y` на ~amd64 в sys-kernel/gentoo-sources с 5.10+;
старые ядра включают явно.)
