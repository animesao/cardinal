<!-- cardinal-version:start -->
**Documentation version:** `2.0.6`
**Project release:** `v2.0.6`
<!-- cardinal-version:end -->

# Установка cardinal на Debian / Ubuntu

## Вариант 1: APT-репозиторий (Рекомендуется)

Добавьте официальный APT-репозиторий cardinal и установите:

```bash
# Добавить репозиторий
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/install-apt.sh | sudo bash

# Установить
sudo apt update
sudo apt install cardinal
```

### Что делает скрипт

1. Добавляет GPG-ключ в `/usr/share/keyrings/cardinal-archive-keyring.gpg`
2. Добавляет репозиторий в `/etc/apt/sources.list.d/cardinal.list`
3. Запускает `apt update && apt install cardinal`

### Поддержка архитектур

| Архитектура | Пакет |
|-------------|-------|
| amd64 | `cardinal` (по умолчанию) |
| arm64 | `cardinal` (автоопределение) |
| armhf (armv6) | `cardinal` (автоопределение) |

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.deb" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.deb"
sudo dpkg -i "cardinal-${VERSION}-linux-${SUFFIX}.deb"
rm "cardinal-${VERSION}-linux-${SUFFIX}.deb"
```

## Вариант 3: Универсальный установщик

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
# Если установлен через APT
sudo apt remove cardinal
sudo rm /etc/apt/sources.list.d/cardinal.list
sudo rm /usr/share/keyrings/cardinal-archive-keyring.gpg

# Удалить данные
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
