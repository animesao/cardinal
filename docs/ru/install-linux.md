<!-- cardinal-version:start -->
**Documentation version:** `1.61.1`
**Project release:** `v1.61.1`
<!-- cardinal-version:end -->

# Установка cardinal на Linux (Универсальная)

Универсальный установщик работает на любом Linux-дистрибутиве с systemd.

## Быстрая установка

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

Устанавливает последнюю стабильную версию в `/usr/local/bin/cardinal` и включает systemd-супервизор.

## Что делает скрипт

1. Определяет архитектуру (amd64, arm64, armv6)
2. Скачивает бинарник последнего релиза с GitHub
3. Проверяет SHA256 контрольную сумму
4. Устанавливает в `/usr/local/bin/cardinal`
5. Включает `cardinal-bootstrap.service` (автозапуск при загрузке)

## Требования

- Linux ядро с поддержкой namespaces (PID, Mount, Net, UTS, IPC)
- `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep`
- Модуль ядра overlayfs
- systemd (для супервизора и автозапуска)

## Проверка установки

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
