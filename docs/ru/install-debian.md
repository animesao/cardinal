<!-- dck-version:start -->
**Documentation version:** `1.60.0`
**Project release:** `v1.60.0`
<!-- dck-version:end -->

# Установка dck на Debian / Ubuntu

## Вариант 1: APT-репозиторий (Рекомендуется)

Добавьте официальный APT-репозиторий dck и установите:

```bash
# Добавить репозиторий
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/scripts/install-apt.sh | sudo bash

# Установить
sudo apt update
sudo apt install dck
```

### Что делает скрипт

1. Добавляет GPG-ключ в `/usr/share/keyrings/dck-archive-keyring.gpg`
2. Добавляет репозиторий в `/etc/apt/sources.list.d/dck.list`
3. Запускает `apt update && apt install dck`

### Поддержка архитектур

| Архитектура | Пакет |
|-------------|-------|
| amd64 | `dck` (по умолчанию) |
| arm64 | `dck` (автоопределение) |
| armhf (armv6) | `dck` (автоопределение) |

## Вариант 2: Ручная установка .deb

Скачайте и установите `.deb` пакет напрямую с GitHub Releases:

```bash
# Определить архитектуру
ARCH=$(dpkg --print-architecture)
case "$ARCH" in
  amd64)  SUFFIX="amd64" ;;
  arm64)  SUFFIX="arm64" ;;
  armhf)  SUFFIX="armv6" ;;
  *)      echo "Не поддерживается: $ARCH"; exit 1 ;;
esac

# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.deb" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.deb"
sudo dpkg -i "dck-${VERSION}-linux-${SUFFIX}.deb"
rm "dck-${VERSION}-linux-${SUFFIX}.deb"
```

## Вариант 3: Универсальный установщик

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
# Если установлен через APT
sudo apt remove dck
sudo rm /etc/apt/sources.list.d/dck.list
sudo rm /usr/share/keyrings/dck-archive-keyring.gpg

# Удалить данные
dck bootstrap --remove
sudo rm -rf ~/.dck
```
