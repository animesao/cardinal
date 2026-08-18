<!-- dck-version:start -->
**Documentation version:** `1.60.9`
**Project release:** `v1.60.9`
<!-- dck-version:end -->

# Установка dck через AppImage

AppImage — портативный формат, не требующий менеджера пакетов.

## Быстрая установка (Рабочий стол)

Дважды щёлкните по файлу `.AppImage`. Терминальный установщик:

1. Скопирует бинарник в `/usr/local/bin/dck`
2. Включит systemd-супервизор

## CLI-установка

```bash
# Получить последнюю версию
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Скачать
curl -fL -o "dck-${VERSION}-linux-amd64.AppImage" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-amd64.AppImage"
chmod +x "dck-${VERSION}-linux-amd64.AppImage"

# Установить
"./dck-${VERSION}-linux-amd64.AppImage" --install

# Или использовать напрямую без установки
"./dck-${VERSION}-linux-amd64.AppImage" run --rm alpine echo hello
```

## Поддержка архитектур

| Архитектура | AppImage |
|-------------|----------|
| x86_64 | ✅ `dck-*-linux-amd64.AppImage` |
| aarch64 | ✅ `dck-*-linux-arm64.AppImage` |
| armv6 | ❌ Используйте бинарник напрямую |

## Портативное использование

AppImage работает без установки:

```bash
./dck-*-linux-amd64.AppImage version
./dck-*-linux-amd64.AppImage run --rm alpine echo hello
```

## Проверка

```bash
dck version
dck doctor
```

## Удаление

```bash
# Если установлен через --install
dck bootstrap --remove
sudo rm /usr/local/bin/dck

# Удалить AppImage файл
rm dck-*-linux-amd64.AppImage

# Удалить данные
sudo rm -rf ~/.dck
```
