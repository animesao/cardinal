<!-- cardinal-version:start -->
**Documentation version:** `1.61.1`
**Project release:** `v1.61.1`
<!-- cardinal-version:end -->

# Nix / NixOS packaging for `cardinal`

This directory contains:

| File | Format | Use when |
|---|---|---|
| `flake.nix` | Nix flake (outputs) | You're on Nix ≥ 2.4 with flakes enabled |
| `default.nix` | Classic Nix expression | You're on pre-flake Nix or evaluating via `nix-build` |

Both produce the same derivation. Pick whichever matches your workflow.

## Quick start (flake)

```bash
# One-shot, runnable shell wrapper that points to `--version`
nix run github:animesao/cardinal -- --version

# Persistent install for the current user
nix profile install github:animesao/cardinal

# Add to a NixOS system configuration
{
  inputs.cardinal.url = "github:animesao/cardinal/v1.24.16";
  outputs.nixosConfigurations.example = nixpkgs.lib.nixosSystem {
    modules = [
      ({ pkgs, ... }: { environment.systemPackages = [ inputs.cardinal.packages.${system}.default ]; })
    ];
  };
}
```

## First-time-fork fixup

Both `flake.nix` and `default.nix` ship with placeholder hashes:

```
srcSha     = pkgs.lib.fakeHash;   # = "sha256-AAAA…"
vendorHash = pkgs.lib.fakeHash;   # = "sha256-AAAA…"
```

That's deliberate — when the maintainer who forked the derivation
runs `nix build` for the first time against a new `cardinal` tag, Nix will
print the real hashes in the build log and refuse to silently accept
the placeholders. The maintainer copies the printed hashes back into
this file and commits. This is how `nixpkgs` itself handles every
fetchFromGitHub + buildGoModule pair; doing it for our flake keeps it
honest and reproducible.

If you're a **consumer** installing from the canonical
`github:animesao/cardinal`, you do not need to fix hashes yourself —
the flake already carries them (post-tag).

## Verification after install

```bash
cardinal version    # expect v1.24.16 (or the tag you pinned)
cardinal doctor     # reports kernel-features / namespace / cgroup state
```

## See also

- `docs/en/install-nixos.md` — full step-by-step walk-through
- `docs/ru/install-nixos.md` — same content in Russian
