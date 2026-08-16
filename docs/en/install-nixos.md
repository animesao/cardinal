<!-- dck-version:start -->
**Documentation version:** `1.24.14`
**Project release:** `v1.24.14`
<!-- dck-version:end -->

# Installing dck on NixOS

## Option 1: Flake (Recommended)

Add dck as a flake input in your NixOS configuration:

### In `flake.nix`

```nix
{
  description = "My NixOS config";

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

### In `configuration.nix` (if not using flake directly)

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

### Build and switch

```bash
sudo nixos-rebuild switch
```

## Option 2: Nix Profile (User Install)

Install dck into your user profile:

```bash
nix profile install github:animesao/dck
```

## Option 3: Temporary Usage

Run dck without installing:

```bash
nix run github:animesao/dck -- run --rm alpine echo "hello"
```

## Option 4: Build from Source

```bash
# Clone and build
git clone https://github.com/animesao/dck.git
cd dck
nix build .#packages.x86_64-linux.dck

# The binary is in ./result/bin/dck
./result/bin/dck version
```

## What the NixOS Module Does

The `services.dck` module:

1. Creates a `dck` system user and group
2. Sets up systemd service with security hardening
3. Configures data directories in `/var/lib/dck`
4. Enables kernel modules: `overlay`, `veth`, `br_netfilter`
5. Applies systemd sandboxing (ProtectSystem, PrivateTmp, etc.)

## Verify

```bash
dck version
dck doctor
systemctl status dck
```

## Uninstall

### From NixOS module

Remove `services.dck.enable = true` from your configuration and rebuild:

```bash
sudo nixos-rebuild switch
```

### From nix profile

```bash
nix profile remove dck
```

### Remove data

```bash
sudo rm -rf /var/lib/dck
```
