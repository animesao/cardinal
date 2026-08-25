<!-- cardinal-version:start -->
**Documentation version:** `1.61.1`
**Project release:** `v1.61.1`
<!-- cardinal-version:end -->

# Установка cardinal на Alpine Linux

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.apk" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.apk"
sudo apk add --allow-untrusted "cardinal-${VERSION}-linux-${SUFFIX}.apk"
rm "cardinal-${VERSION}-linux-${SUFFIX}.apk"
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
cardinal version
cardinal doctor
```

## Удаление

```bash
cardinal bootstrap --remove
sudo rm /usr/local/bin/cardinal
sudo rm -rf ~/.cardinal
```
