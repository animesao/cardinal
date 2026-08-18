<!-- dck-version:start -->
**Documentation version:** `1.60.3`
**Project release:** `v1.60.3`
<!-- dck-version:end -->

# Установка dck на Linux (Универсальная)

Универсальный установщик работает на любом Linux-дистрибутиве с systemd.

## Быстрая установка

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

Устанавливает последнюю стабильную версию в `/usr/local/bin/dck` и включает systemd-супервизор.

## Что делает скрипт

1. Определяет архитектуру (amd64, arm64, armv6)
2. Скачивает бинарник последнего релиза с GitHub
3. Проверяет SHA256 контрольную сумму
4. Устанавливает в `/usr/local/bin/dck`
5. Включает `dck-bootstrap.service` (автозапуск при загрузке)

## Требования

- Linux ядро с поддержкой namespaces (PID, Mount, Net, UTS, IPC)
- `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep`
- Модуль ядра overlayfs
- systemd (для супервизора и автозапуска)

## Проверка установки

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
