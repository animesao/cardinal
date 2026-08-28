<!-- cardinal-version:start -->
**Documentation version:** `2.0.12`
**Project release:** `v2.0.12`
<!-- cardinal-version:end -->

# Установка cardinal на Arch Linux

## Вариант 1: Pacman-пакет (Рекомендуется)

Скачайте и установите `.pkg.tar.zst` пакет с GitHub Releases:

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
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.pkg.tar.zst" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
sudo pacman -U --noconfirm "cardinal-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
rm "cardinal-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
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

## Модули ядра

Убедитесь, что необходимые модули загружены:

```bash
sudo modprobe overlay
sudo modprobe veth
sudo modprobe br_netfilter

# Сделать постоянными
echo -e "overlay\nveth\nbr_netfilter" | sudo tee /etc/modules-load.d/cardinal.conf
```

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
sudo pacman -R cardinal
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
