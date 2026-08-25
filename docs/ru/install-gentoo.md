<!-- cardinal-version:start -->
**Documentation version:** `1.61.0`
**Project release:** `v1.61.0`
<!-- cardinal-version:end -->

# Установка cardinal в Gentoo / Funtoo / Calculate

Канонический ebuild лежит в каталоге
[`contrib/gentoo/`](../../contrib/gentoo/). Два варианта:

1. Содержать `cardinal` в **локальном overlay**, который вы поддерживаете
   руками.
2. Открыть PR в https://github.com/gentoo/guru (целевая категория:
   `app-containers/cardinal`), чтобы пакет стал общедоступным.

## Быстрый путь: локальный overlay

```bash
# Создать или переиспользовать локальный overlay root
if [ ! -d /var/db/repos/cardinal-overlay ]; then
    sudo install -d -o root -g portage \
        /var/db/repos/cardinal-overlay/{profiles,metadata,app-containers/cardinal}
    echo 'cardinal-overlay' | sudo tee /var/db/repos/cardinal-overlay/profiles/repo_name
    cat <<'EOF' | sudo tee /var/db/repos/cardinal-overlay/metadata/layout.conf
masters = gentoo
EOF
fi

# Положить ebuild в стандартную категорию
sudo cp contrib/gentoo/cardinal-1.24.15.ebuild \
        /var/db/repos/cardinal-overlay/app-containers/cardinal/

# Подключить overlay через repos.conf
repos_conf="/etc/portage/repos.conf/cardinal-overlay.conf"
if [ ! -f "$repos_conf" ]; then
    sudo tee "$repos_conf" >/dev/null <<'EOF'
[cardinal-overlay]
location = /var/db/repos/cardinal-overlay
priority = 50
EOF
fi

# Сгенерировать SHA256-манифест
sudo chown -R portage:portage /var/db/repos/cardinal-overlay
cd /var/db/repos/cardinal-overlay/app-containers/cardinal
sudo Manifest-md5 cardinal-1.24.15.ebuild || \
sudo Manifest-sha256 cardinal-1.24.15.ebuild

# Поставить
sudo emerge --sync
sudo emerge --ask app-containers/cardinal
```

Если Portage ругается на `~amd64`, добавьте в
`/etc/portage/package.accept_keywords/`:

```
=app-containers/cardinal-1.24.15 ~amd64
```

## USE-флаги / зависимости

USE-флагов в ebuild нет. Внутри:

- `>=dev-lang/go-1.23` (соответствует `go.mod`)
- Ядро ≥ 4.18 с user namespaces и cgroup v2 (проверяется через `cardinal doctor`)
- `sys-apps/util-linux` (для `mount`, `unshare`)

Опционально (рекомендуется):

```
sys-apps/iproute2   # для сетевых плагинов (bridge / iptables)
```

## Отправить в ::guru

Если предпочитаете community-overlay, готовьте PR в
`gentoo/guru`. Стандартная структура:

```
app-containers/cardinal/
├── cardinal-1.24.15.ebuild
├── cardinal-9999.ebuild          # для git-версии в реальном времени
└── files/
    └── cardinal.initd            # (сейчас пустой стаб)
```

В нашем ebuild уже подготовлены места под `newinitd` / `newconfd` —
если захотите добавить systemd/OpenRC supervisor, дополните их.

Описание PR, которое проходит быстро:

> Добавляет `app-containers/cardinal` в ::guru. cardinal — Go 1.23+ чисто-Go
> бинарник, повторяющий docker CLI, без демона. Ebuild использует
> `go-module.eclass` и работает с upstream-`vendor/`. Тесты
> RESTRICT-ятся, потому что им нужны namespaces и root.

## Проверка после установки

```bash
$ which cardinal
/usr/bin/cardinal
$ cardinal version
cardinal version 1.24.15 (commit 52ba511, ...)
$ sudo cardinal doctor
... отчёт о возможностях ядра ...
```

Если `cardinal doctor` ругается на `user namespaces`, проверьте ядро:

```
CONFIG_USER_NS=y
CONFIG_USER_NS_FASYNC=y
```

(Обычно `=y` на ~amd64 в sys-kernel/gentoo-sources с 5.10+;
старые ядра включают явно.)
