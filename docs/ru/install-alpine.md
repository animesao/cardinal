<!-- dck-version:start -->
**Documentation version:** `1.60.2`
**Project release:** `v1.60.2`
<!-- dck-version:end -->

# Установка dck на Alpine Linux

## Вариант 1: APK-пакет (Рекомендуется)

Скачайте и установите `.apk` пакет с GitHub Releases:

```bash
# Определить архитектуру
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  armv7l)  SUFFIX="armv6" ;;
  *)       echo "Не поддерживается: $ARCH"; exit 1 ;;
esac

# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.apk" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.apk"
sudo apk add --allow-untrusted "dck-${VERSION}-linux-${SUFFIX}.apk"
rm "dck-${VERSION}-linux-${SUFFIX}.apk"
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

## Необходимые пакеты

Alpine требует дополнительные пакеты для сети:

```bash
sudo apk add iptables ip6tables iproute2
```

## Модули ядра

```bash
sudo modprobe overlay
sudo modprobe veth
sudo modprobe br_netfilter

# Сделать постоянными
echo -e "overlay\nveth\nbr_netfilter" | sudo tee /etc/modules
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
dck bootstrap --remove
sudo rm /usr/local/bin/dck
sudo rm -rf ~/.dck
```
