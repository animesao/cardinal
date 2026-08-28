<!-- cardinal-version:start -->
**Documentation version:** `2.0.10`
**Project release:** `v2.0.10`
<!-- cardinal-version:end -->

# Установка cardinal через AppImage

AppImage — портативный формат, не требующий менеджера пакетов.

## Быстрая установка (Рабочий стол)

Дважды щёлкните по файлу `.AppImage`. Терминальный установщик:

1. Скопирует бинарник в `/usr/local/bin/cardinal`
2. Включит systemd-супервизор

## CLI-установка

```bash
# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать
curl -fL -o "cardinal-${VERSION}-linux-amd64.AppImage" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-amd64.AppImage"
chmod +x "cardinal-${VERSION}-linux-amd64.AppImage"

# Установить
"./cardinal-${VERSION}-linux-amd64.AppImage" --install

# Или использовать напрямую без установки
"./cardinal-${VERSION}-linux-amd64.AppImage" run --rm alpine echo hello
```

## Поддержка архитектур

| Архитектура | AppImage |
|-------------|----------|
| x86_64 | ✅ `cardinal-*-linux-amd64.AppImage` |
| aarch64 | ✅ `cardinal-*-linux-arm64.AppImage` |
| armv6 | ❌ Используйте бинарник напрямую |

## Портативное использование

AppImage работает без установки:

```bash
./cardinal-*-linux-amd64.AppImage version
./cardinal-*-linux-amd64.AppImage run --rm alpine echo hello
```

## Проверка

```bash
cardinal version
cardinal doctor
```

## Удаление

```bash
# Если установлен через --install
cardinal bootstrap --remove
sudo rm /usr/local/bin/cardinal

# Удалить AppImage файл
rm cardinal-*-linux-amd64.AppImage

# Удалить данные
sudo rm -rf ~/.cardinal
```
