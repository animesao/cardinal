<!-- cardinal-version:start -->
**Documentation version:** `2.0.1`
**Project release:** `v2.0.1`
<!-- cardinal-version:end -->

# Установка cardinal — Ручная установка бинарника

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Вариант 1: Распаковка и установка

```bash
tar xzf "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
sudo mv "cardinal-${VERSION}/cardinal" /usr/local/bin/cardinal
chmod +x /usr/local/bin/cardinal
rm -rf "cardinal-${VERSION}" "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Вариант 2: Прямое скачивание бинарника

```bash
curl -fL -o /usr/local/bin/cardinal \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-linux-${SUFFIX}"
chmod +x /usr/local/bin/cardinal
```

## Вариант 3: Из исходников

```bash
# Требуется Go 1.26+
git clone https://github.com/animesao/cardinal.git
cd cardinal
go build -tags netgo -ldflags="-s -w" -o /usr/local/bin/cardinal .
```

## Включить супервизор (Опционально)

Для автозапуска при загрузке и запланированных бэкапов:

```bash
cardinal bootstrap --install
```

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
cardinal bootstrap --remove 2>/dev/null || true
sudo rm /usr/local/bin/cardinal
sudo rm -rf ~/.cardinal
```
