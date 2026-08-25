# Fallback for users on classic Nix (pre-flakes) or `nix-env`.
#
# Usage:
#   nix-build -E 'with import <nixpkgs> {}; callPackage ./contrib/nix/default.nix {}'
#   nix-env -if 'nixpkgs=https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz' \
#            -E '(import <nixpkgs> {}).callPackage ./contrib/nix/default.nix {}'
#
# This file intentionally mirrors `contrib/nix/flake.nix` so the two
# derivations stay in lock-step. When you bump `version` in the flake,
# bump it here too.

{ pkgs ? import <nixpkgs> {}
, version ? "1.24.15"
, srcSha ? pkgs.lib.fakeHash
, vendorSha ? pkgs.lib.fakeHash
}:

pkgs.buildGoModule {
  pname = "cardinal";
  inherit version;

  src = pkgs.fetchFromGitHub {
    owner = "animesao";
    repo = "cardinal";
    rev = "refs/tags/v${version}";
    sha256 = srcSha;
  };

  vendorHash = vendorSha;

  doCheck = false;

  env = { CGO_ENABLED = "0"; };
  ldflags = [
    "-s" "-w" "-buildid="
    "-X cardinal/cmd.version=${version}"
  ];

  meta = with pkgs.lib; {
    description = "Lightweight, daemonless, OCI-compatible container runtime.";
    homepage = "https://github.com/animesao/cardinal";
    license = licenses.mit;
    platforms = platforms.linux;
    mainProgram = "cardinal";
  };
}
