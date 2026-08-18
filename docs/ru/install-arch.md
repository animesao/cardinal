<!-- dck-version:start -->
**Documentation version:** `1.60.7`
**Project release:** `v1.60.7`
<!-- dck-version:end -->

# Установка dck на Arch Linux

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.pkg.tar.zst" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
sudo pacman -U --noconfirm "dck-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
rm "dck-${VERSION}-linux-${SUFFIX}.pkg.tar.zst"
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

## Модули ядра

Убедитесь, что необходимые модули загружены:

```bash
sudo modprobe overlay
sudo modprobe veth
sudo modprobe br_netfilter

# Сделать постоянными
echo -e "overlay\nveth\nbr_netfilter" | sudo tee /etc/modules-load.d/dck.conf
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
sudo pacman -R dck
dck bootstrap --remove
sudo rm -rf ~/.dck
```
