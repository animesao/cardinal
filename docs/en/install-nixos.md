<!-- cardinal-version:start -->
**Documentation version:** `2.0.1`
**Project release:** `v2.0.1`
<!-- cardinal-version:end -->

# Installing cardinal on Nix / NixOS

`cardinal` ships native flake + classic Nix expressions under
[`contrib/nix/`](../../contrib/nix/). Both produce the same
binary; pick the one that matches your workflow.

## Pick your style

| You use | Pick |
|---|---|
| Nix ≥ 2.4 with flakes enabled | `contrib/nix/flake.nix` (canonical) |
| Pre-flake Nix or `nix-build` / `nix-env` | `contrib/nix/default.nix` |

Both produce a binary identical to the upstream goreleaser
artefact:

```
CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -buildid= -X cardinal/cmd.version=${version}"
```

## Flake one-liner

```bash
# Try it (one-shot, no install required)
nix run github:animesao/cardinal -- --version

# Install for the current user (adds to ~/.nix-profile)
nix profile install github:animesao/cardinal
```

`nix run` drops you into a `cardinal ...` invocation with no global
state changes; `nix profile install` is the persistence equivalent
on a non-NixOS system.

## Add to a NixOS system configuration

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

After running `nixos-rebuild switch`, `cardinal` lands at
`/run/current-system/sw/bin/cardinal` for every user on the system.

## Add to a Home Manager user profile

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

## Verify kernel prerequisites

`cardinal` needs an active kernel configuration. Run on the host after
install:

```bash
cardinal doctor
```

The output is structured: the same script decides whether `cardinal`
can run `run`, `serve`, `network create`, etc. Useful lines to look
at:

```
platform                       OK   Linux runtime
user namespaces                OK   available
cgroups                        OK   cgroups v2 detected
overlayfs                      OK   available
```

If any of those are `WARN` or `FAIL`, you have a kernel-config or
kernel-version issue to fix first; `cardinal` will refuse to update the
filesystem state on a kernel that can't honour the namespace
guarantees it advertises.

## Q & A

**Q: Can I use `nix shell` to enter a dev shell?**
A: Yes. `nix shell github:animesao/cardinal#devShells.<system>.default`
gives you a shell with `go`, `golangci-lint`, `shellcheck`,
matching the upstream build matrix.

**Q: I'm getting a hash mismatch on first build.**
A: That's expected if you forked and bumped `version` without
re-fingerprinting both. Look at the build log, copy the printed
hashes into `flake.nix` and `default.nix`, rebuild. See
`contrib/nix/README.md` for the diagnostic flow.

**Q: Does `cardinal` work on NixOS itself, or do I need a different
kernel?** A: Both. NixOS uses the upstream kernel with user
namespaces + cgroup v2 enabled by default since 22.11. Older
NixOS installations need:

```nix
boot.kernel.sysctl."kernel.unprivileged_userns_clone" = 1;
boot.kernelFeatures.enable = [ "cgroup_namespaces" "userns" ];
```
