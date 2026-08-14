<!-- dck-version:start -->
**Documentation version:** `1.24.12`
**Project release:** `v1.24.12`
<!-- dck-version:end -->

# Установка dck на NixOS

## Вариант 1: Flake (Рекомендуется)

Добавьте dck как входную точку flake в конфигурацию NixOS:

### В `flake.nix`

```nix
{
  description = "Моя конфигурация NixOS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    dck.url = "github:animesao/dck";
  };

  outputs = { self, nixpkgs, dck, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ./configuration.nix
        dck.nixosModules.dck
        {
          services.dck = {
            enable = true;
            # apiToken = "your-secret-token";
            # apiPort = 2375;
            # dataDir = "/var/lib/dck";
          };
        }
      ];
    };
  };
}
```

### В `configuration.nix` (если не используете flake напрямую)

```nix
{ config, pkgs, ... }:
{
  services.dck = {
    enable = true;
    # apiToken = "your-secret-token";
    # apiPort = 2375;
    # dataDir = "/var/lib/dck";
    # user = "dck";
    # group = "dck";
  };
}
```

### Сборка и применение

```bash
sudo nixos-rebuild switch
```

## Вариант 2: Nix Profile (Пользовательская установка)

Установите dck в профиль пользователя:

```bash
nix profile install github:animesao/dck
```

## Вариант 3: Временное использование

Запустите dck без установки:

```bash
nix run github:animesao/dck -- run --rm alpine echo "hello"
```

## Вариант 4: Сборка из исходников

```bash
# Клонировать и собрать
git clone https://github.com/animesao/dck.git
cd dck
nix build .#packages.x86_64-linux.dck

# Бинарник в ./result/bin/dck
./result/bin/dck version
```

## Что делает NixOS-модуль

Модуль `services.dck`:

1. Создаёт системного пользователя и группу `dck`
2. Настраивает systemd-сервис с усилением безопасности
3. Конфигурирует директории данных в `/var/lib/dck`
4. Включает модули ядра: `overlay`, `veth`, `br_netfilter`
5. Применяет песочницу systemd (ProtectSystem, PrivateTmp и т.д.)

## Проверка

```bash
dck version
dck doctor
systemctl status dck
```

## Удаление

### Из NixOS-модуля

Удалите `services.dck.enable = true` из конфигурации и пересоберите:

```bash
sudo nixos-rebuild switch
```

### Из nix profile

```bash
nix profile remove dck
```

### Удалить данные

```bash
sudo rm -rf /var/lib/dck
```
