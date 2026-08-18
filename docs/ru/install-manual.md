<!-- dck-version:start -->
**Documentation version:** `1.25.9`
**Project release:** `v1.25.9`
<!-- dck-version:end -->

# Установка dck — Ручная установка бинарника

Скачайте бинарник и установите вручную.

## Скачивание

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

# Скачать
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.tar.gz" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Вариант 1: Распаковка и установка

```bash
tar xzf "dck-${VERSION}-linux-${SUFFIX}.tar.gz"
sudo mv "dck-${VERSION}/dck" /usr/local/bin/dck
chmod +x /usr/local/bin/dck
rm -rf "dck-${VERSION}" "dck-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Вариант 2: Прямое скачивание бинарника

```bash
curl -fL -o /usr/local/bin/dck \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-linux-${SUFFIX}"
chmod +x /usr/local/bin/dck
```

## Вариант 3: Из исходников

```bash
# Требуется Go 1.26+
git clone https://github.com/animesao/dck.git
cd dck
go build -tags netgo -ldflags="-s -w" -o /usr/local/bin/dck .
```

## Включить супервизор (Опционально)

Для автозапуска при загрузке и запланированных бэкапов:

```bash
dck bootstrap --install
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
dck bootstrap --remove 2>/dev/null || true
sudo rm /usr/local/bin/dck
sudo rm -rf ~/.dck
```
