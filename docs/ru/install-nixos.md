<!-- dck-version:start -->
**Documentation version:** `1.24.18`
**Project release:** `v1.24.18`
<!-- dck-version:end -->

# Установка dck в Nix / NixOS

`dck` поставляется в виде flake + классического Nix-выражения
в каталоге [`contrib/nix/`](../../contrib/nix/). Оба дают одинаковый
бинарник — берите тот, что подходит под ваш рабочий процесс.

## Выбор стиля

| У вас | Используйте |
|---|---|
| Nix ≥ 2.4 с включёнными flakes | `contrib/nix/flake.nix` (канонично) |
| Pre-flake Nix или `nix-build` / `nix-env` | `contrib/nix/default.nix` |

Оба собирают бинарник, идентичный upstream-артефакту goreleaser:

```
CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -buildid= -X dck/cmd.version=${version}"
```

## Однострочник через flake

```bash
# Однократный запуск, без глобальных изменений
nix run github:animesao/dck -- --version

# Установка для текущего пользователя
nix profile install github:animesao/dck
```

`nix run` запускает `dck ...` без изменения состояния системы;
`nix profile install` — «постоянный» вариант на non-NixOS системе.

## Включение в NixOS system configuration

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    dck.url = "github:animesao/dck/v1.24.15";
    dck.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, dck, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      nixosConfigurations.example = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          ({ pkgs, ... }: {
            environment.systemPackages = [
              dck.packages.${system}.default
            ];
          })
        ];
      };
    };
}
```

После `nixos-rebuild switch` команда `dck` окажется
в `/run/current-system/sw/bin/dck` для всех пользователей системы.

## В профиль Home Manager

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager.url = "github:nix-community/home-manager";
    dck.url = "github:animesao/dck/v1.24.15";
  };

  outputs = { self, nixpkgs, home-manager, dck, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      homeConfigurations.example = home-manager.lib.homeManagerConfiguration {
        inherit pkgs;
        modules = [
          ({ pkgs, ... }: {
            home.packages = [ dck.packages.${system}.default ];
          })
        ];
      };
    };
}
```

## Проверка требований к ядру

`dck` требует активную kernel-config. Запустите на хосте после
установки:

```bash
dck doctor
```

Полезные строки, на которые стоит обратить внимание:

```
platform                       OK   Linux runtime
user namespaces                OK   available
cgroups                        OK   cgroups v2 detected
overlayfs                      OK   available
```

Если что-то в режиме `WARN` или `FAIL` — нужно докрутить ядро.
NixOS ≥ 22.11 по умолчанию включает user namespaces и cgroup v2 —
на старых инсталляциях добавьте:

```nix
boot.kernel.sysctl."kernel.unprivileged_userns_clone" = 1;
boot.kernelFeatures.enable = [ "cgroup_namespaces" "userns" ];
```

## Q & A

**В: При первом билде вылетает hash mismatch.**
О: Это ожидаемо, если вы сделали форк и подняли `version` без
пересчёта хэшей. Возьмите реальные хэши из лога сборки, впишите
их в `flake.nix` и `default.nix`, пересоберите. Подробнее —
см. `contrib/nix/README.md`.

**В: Можно воспользоваться `nix shell`?**
О: Да. `nix shell github:animesao/dck#devShells.<system>.default`
откроет шелл с `go`, `golangci-lint`, `shellcheck` — теми же
инструментами, что в upstream build-matrix.
