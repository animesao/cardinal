{
  description = "dck — lightweight container runtime for Linux";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = builtins.readFile ./VERSION;
        versionStr = builtins.replaceStrings ["\n"] [""] version;
      in
      {
        packages = {
          dck = pkgs.callPackage ./nix/default.nix {
            version = versionStr;
          };
          default = self.packages.${system}.dck;
        };

        overlays.default = final: prev: {
          dck = self.packages.${system}.dck;
        };

        apps = {
          dck = flake-utils.lib.mkApp {
            drv = self.packages.${system}.dck;
            name = "dck";
          };
          default = self.apps.${system}.dck;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            git
            curl
            jq
          ];

          shellHook = ''
            echo "dck development shell"
            echo "Run 'go build .' to build"
          '';
        };
      }
    ) // {
      nixosModules.dck = { config, lib, pkgs, ... }:
        with lib;
        let
          cfg = config.services.dck;
        in
        {
          options.services.dck = {
            enable = mkEnableOption "dck container runtime";

            package = mkOption {
              type = types.package;
              default = self.packages.${pkgs.system}.dck;
              defaultText = "self.packages.\${pkgs.system}.dck";
              description = "The dck package to use.";
            };

            dataDir = mkOption {
              type = types.path;
              default = "/var/lib/dck";
              description = "Data directory for dck state.";
            };

            user = mkOption {
              type = types.str;
              default = "dck";
              description = "User to run dck as.";
            };

            group = mkOption {
              type = types.str;
              default = "dck";
              description = "Group to run dck as.";
            };

            apiToken = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = "API authentication token.";
            };

            apiPort = mkOption {
              type = types.port;
              default = 2375;
              description = "API port.";
            };

            apiHost = mkOption {
              type = types.str;
              default = "127.0.0.1";
              description = "API bind host.";
            };
          };

          config = mkIf cfg.enable {
            users.users.${cfg.user} = {
              isSystemUser = true;
              group = cfg.group;
              home = cfg.dataDir;
              extraGroups = [ "networkmanager" ];
            };

            users.groups.${cfg.group} = {};

            systemd.services.dck = {
              description = "dck container runtime";
              after = [ "network-online.target" ];
              wants = [ "network-online.target" ];
              wantedBy = [ "multi-user.target" ];

              serviceConfig = {
                Type = "simple";
                ExecStart = "${cfg.package}/bin/dck supervisor";
                Restart = "always";
                RestartSec = 5;
                KillMode = "process";

                # Security hardening
                User = cfg.user;
                Group = cfg.group;
                WorkingDirectory = cfg.dataDir;

                # Capabilities
                CapabilityBoundingSet = [
                  "CAP_NET_ADMIN"
                  "CAP_NET_RAW"
                  "CAP_SYS_ADMIN"
                  "CAP_SYS_PTRACE"
                  "CAP_SYS_CHROOT"
                  "CAP_MKNOD"
                  "CAP_CHOWN"
                  "CAP_FOWNER"
                  "CAP_FSETID"
                  "CAP_SETGID"
                  "CAP_SETUID"
                  "CAP_SETPCAP"
                  "CAP_NET_BIND_SERVICE"
                ];

                # Namespace restrictions
                ProtectSystem = "strict";
                ProtectHome = "read-only";
                PrivateTmp = "true";
                ProtectKernelTunables = "true";
                ProtectKernelModules = "true";
                ProtectControlGroups = "true";
                RestrictAddressFamilies = "AF_UNIX AF_INET AF_INET6 AF_NETLINK";
                RestrictNamespaces = "true";
                LockPersonality = "true";
                MemoryDenyWriteExecute = "true";
                RestrictRealtime = "true";
                RestrictSUIDSGID = "true";
                RemoveIPC = "true";
                PrivateMounts = "true";

                # Resource limits
                LimitNOFILE = 65536;
                LimitNPROC = 65536;
              };

              environment = {
                DCK_DATA_DIR = cfg.dataDir;
              } // optionalAttrs (cfg.apiToken != null) {
                DCK_TOKEN = cfg.apiToken;
                DCK_HOST = "${cfg.apiHost}:${toString cfg.apiPort}";
              };
            };

            # Create data directory
            systemd.tmpfiles.rules = [
              "d ${cfg.dataDir} 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/containers 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/images 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/overlay 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/logs 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/backups 0700 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/networks 0755 ${cfg.user} ${cfg.group} -"
              "d ${cfg.dataDir}/audit 0700 ${cfg.user} ${cfg.group} -"
            ];
          };
        };
    };
}
