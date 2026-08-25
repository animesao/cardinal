{
  description = "cardinal — lightweight, daemonless, OCI-compatible container runtime";

  # Inputs are intentionally pinned to a recent nixos-unstable snapshot so
  # buildGoModule's vendorHash / fetcherHash expectations stay reproducible.
  # Bump `nixpkgs` once a year; bump `flake-utils` opportunistically.
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        # Track the upstream tag of cardinal. Update this when bumping releases:
        #   1. bump `version`
        #   2. leave both `src.sha256` and `vendorHash` set to
        #      `pkgs.lib.fakeHash` -- first `nix build` will print the real
        #      hash and refuse to silently accept it
        #   3. commit the printed hashes back into this file
        version = "1.24.15";
        srcSha = pkgs.lib.fakeHash;
        vendorSha = pkgs.lib.fakeHash;
      in
      {
        # `nix build` / `nix profile install` default derivation.
        packages.default = pkgs.buildGoModule {
          pname = "cardinal";
          inherit version;

          src = pkgs.fetchFromGitHub {
            owner = "animesao";
            repo = "cardinal";
            rev = "refs/tags/v${version}";
            sha256 = srcSha;       # first build prints the real one
          };

          # vendor/ directory lives in the repository, but buildGoModule
          # still wants the hash to validate that no go.mod / vendor drift
          # happens between the upstream tag and the built artefact.
          vendorHash = vendorSha;  # first build prints the real one

          # Tests require root + namespaces + overlayfs + cgroup v2 --
          # which the Nix build sandbox cannot provide. Skip them here;
          # they run upstream on every push via `go test -race`.
          doCheck = false;

          # cardinal is a static binary: pure go, no cgo, no glibc anchor.
          # CGO_ENABLED=0 + the ldflags below reproducibly strip debug
          # info, kill .buildid, and stamp the version string into the
          # embedded cmd.Version var.
          env = { CGO_ENABLED = "0"; };
          ldflags = [
            "-s" "-w" "-buildid="
            "-X cardinal/cmd.version=${version}"
          ];

          # We don't need any sub-commands available on the build
          # host beyond what buildGoModule pulls in transitively.
          meta = with pkgs.lib; {
            description = "Lightweight, daemonless, OCI-compatible container runtime (linux/amd64, linux/arm64).";
            longDescription = ''
              cardinal is a CLI/runtime in the spirit of docker but without
              dockerd: it pulls OCI images, applies overlayfs, drops
              capabilities, sets up an isolated mount/uts/pid/network
              namespace and execs the entrypoint directly. Used for
              serverless workloads (FaaS), single-node clusters,
              blueprints, and Docker-Compose-style `up`/`down` flows.
            '';
            homepage = "https://github.com/animesao/cardinal";
            license = licenses.mit;
            platforms = platforms.linux;
            mainProgram = "cardinal";
            maintainers = [{
              name = "animesao";
              email = "animesao@users.noreply.github.com";
            }];
          };
        };

        # `nix run github:animesao/cardinal` should drop the user straight
        # into a `cardinal --version` invocation.
        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/cardinal";
        };

        # Convenience dev shell with the same toolchain the upstream
        # build matrix uses: go 1.25+, golangci-lint, shellcheck.
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            shellcheck
            git
          ];
          shellHook = ''
            echo "cardinal dev-shell: Go $(go version)"
          '';
        };
      }
    );
}
