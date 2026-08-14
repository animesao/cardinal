<!-- dck-version:start -->
**Documentation version:** `1.24.11`
**Project release:** `v1.24.11`
<!-- dck-version:end -->

# Установка dck на Fedora / RHEL / CentOS

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.rpm" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.rpm"
sudo rpm -i "dck-${VERSION}-linux-${SUFFIX}.rpm"
rm "dck-${VERSION}-linux-${SUFFIX}.rpm"
```

## Вариант 2: Универсальный установщик

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

## Вариант 3: Архив с бинарником

```bash
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "dck-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-amd64.tar.gz"
tar xzf "dck-${VERSION}-linux-amd64.tar.gz"
sudo mv "dck-${VERSION}/dck" /usr/local/bin/dck
rm -rf "dck-${VERSION}" "dck-${VERSION}-linux-amd64.tar.gz"
```

## Брандмауэр

Если используется `firewalld`, разрешите сетевые операции контейнеров:

```bash
sudo firewall-cmd --permanent --add-masquerade
sudo firewall-cmd --reload
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
# Если установлен через RPM
sudo rpm -e dck

# Удалить данные
dck bootstrap --remove
sudo rm -rf ~/.dck
```
