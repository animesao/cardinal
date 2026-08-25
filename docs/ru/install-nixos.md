<!-- cardinal-version:start -->
**Documentation version:** `2.0.3`
**Project release:** `v2.0.3`
<!-- cardinal-version:end -->

# Установка cardinal в Nix / NixOS

`cardinal` поставляется в виде flake + классического Nix-выражения
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
go build -trimpath -ldflags="-s -w -buildid= -X cardinal/cmd.version=${version}"
```

## Однострочник через flake

```bash
# Однократный запуск, без глобальных изменений
nix run github:animesao/cardinal -- --version

# Установка для текущего пользователя
nix profile install github:animesao/cardinal
```

`nix run` запускает `cardinal ...` без изменения состояния системы;
`nix profile install` — «постоянный» вариант на non-NixOS системе.

## Включение в NixOS system configuration

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    cardinal.url = "github:animesao/cardinal/v1.24.15";
    cardinal.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, cardinal, ... }:
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
              cardinal.packages.${system}.default
            ];
          })
        ];
      };
    };
}
```

После `nixos-rebuild switch` команда `cardinal` окажется
в `/run/current-system/sw/bin/cardinal` для всех пользователей системы.

## В профиль Home Manager

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager.url = "github:nix-community/home-manager";
    cardinal.url = "github:animesao/cardinal/v1.24.15";
  };

  outputs = { self, nixpkgs, home-manager, cardinal, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      homeConfigurations.example = home-manager.lib.homeManagerConfiguration {
        inherit pkgs;
        modules = [
          ({ pkgs, ... }: {
            home.packages = [ cardinal.packages.${system}.default ];
          })
        ];
      };
    };
}
```

## Проверка требований к ядру

`cardinal` требует активную kernel-config. Запустите на хосте после
установки:

```bash
cardinal doctor
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
О: Да. `nix shell github:animesao/cardinal#devShells.<system>.default`
откроет шелл с `go`, `golangci-lint`, `shellcheck` — теми же
инструментами, что в upstream build-matrix.
