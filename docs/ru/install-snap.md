<!-- dck-version:start -->
**Documentation version:** `1.24.18`
**Project release:** `v1.24.18`
<!-- dck-version:end -->

# Установка dck через Snap

## Установка с GitHub Releases

Скачайте `.snap` пакет с GitHub Releases:

```bash
# Определить архитектуру
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  *)       echo "Не поддерживается: $ARCH"; exit 1 ;;
esac

# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.snap" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.snap"
sudo snap install --dangerous --classic "dck-${VERSION}-linux-${SUFFIX}.snap"
rm "dck-${VERSION}-linux-${SUFFIX}.snap"
```

> **Примечание:** `--dangerous` required, так как snap не из Snap Store. `--classic` даёт dck полный доступ к системе (необходимо для операций с namespace).

## Сборка из исходников

```bash
git clone https://github.com/animesao/dck.git
cd dck
snapcraft
sudo snap install --dangerous --classic ./dck_*.snap
```

## Что вы получаете

- Бинарник в `/snap/bin/dck`
- Classic confinement (полный доступ к системе)
- Автоматический алиас: команда `dck` доступна глобально

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
sudo snap remove dck
dck bootstrap --remove
sudo rm -rf ~/.dck
```
