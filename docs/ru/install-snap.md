<!-- cardinal-version:start -->
**Documentation version:** `2.0.12`
**Project release:** `v2.0.12`
<!-- cardinal-version:end -->

# Установка cardinal через Snap

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать и установить
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.snap" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.snap"
sudo snap install --dangerous --classic "cardinal-${VERSION}-linux-${SUFFIX}.snap"
rm "cardinal-${VERSION}-linux-${SUFFIX}.snap"
```

> **Примечание:** `--dangerous` required, так как snap не из Snap Store. `--classic` даёт cardinal полный доступ к системе (необходимо для операций с namespace).

## Сборка из исходников

```bash
git clone https://github.com/animesao/cardinal.git
cd cardinal
snapcraft
sudo snap install --dangerous --classic ./cardinal_*.snap
```

## Что вы получаете

- Бинарник в `/snap/bin/cardinal`
- Classic confinement (полный доступ к системе)
- Автоматический алиас: команда `cardinal` доступна глобально

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
sudo snap remove cardinal
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
