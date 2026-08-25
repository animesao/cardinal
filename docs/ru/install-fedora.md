<!-- cardinal-version:start -->
**Documentation version:** `2.0.1`
**Project release:** `v2.0.1`
<!-- cardinal-version:end -->

# Установка cardinal на Fedora / RHEL / CentOS

## Вариант 1: RPM-пакет (Рекомендуется)

Скачайте и установите `.rpm` пакет с GitHub Releases:

```bash
# Определить архитектуру
ARCH=$(rpm --eval '%{_arch}')
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  armv7hl) SUFFIX="armv6" ;;
  *)       echo "Не поддерживается: $ARCH"; exit 1 ;;
esac

# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.rpm" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.rpm"
sudo rpm -i "cardinal-${VERSION}-linux-${SUFFIX}.rpm"
rm "cardinal-${VERSION}-linux-${SUFFIX}.rpm"
```

## Вариант 2: Универсальный установщик

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

## Вариант 3: Архив с бинарником

```bash
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "cardinal-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-amd64.tar.gz"
tar xzf "cardinal-${VERSION}-linux-amd64.tar.gz"
sudo mv "cardinal-${VERSION}/cardinal" /usr/local/bin/cardinal
rm -rf "cardinal-${VERSION}" "cardinal-${VERSION}-linux-amd64.tar.gz"
```

## Брандмауэр

Если используется `firewalld`, разрешите сетевые операции контейнеров:

```bash
sudo firewall-cmd --permanent --add-masquerade
sudo firewall-cmd --reload
```

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
# Если установлен через RPM
sudo rpm -e cardinal

# Удалить данные
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
