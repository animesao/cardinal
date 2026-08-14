{ lib
, buildGoModule
, fetchFromGitHub
, installShellFiles
, version ? "dev"
}:

buildGoModule {
  pname = "dck";
  inherit version;

  src = ./..;

  vendorHash = null; # Static binary, no vendor

  subPackages = [ "." ];

  ldflags = [
    "-s"
    "-w"
    "-X dck/cmd.version=${version}"
  ];

  tags = [ "netgo" ];

  # Allow Go to download the exact toolchain version required by go.mod
  # (nixpkgs may ship an older Go, but go.mod requires >= 1.26.6)
  overrideModAttrs = old: {
    GOTOOLCHAIN = "auto";
  };
  GOTOOLCHAIN = "auto";

  nativeBuildInputs = [ installShellFiles ];

  # Shell completions will be added when dck supports 'dck completion' command
  # For now, install the binary only

  meta = with lib; {
    description = "Lightweight container runtime for Linux";
    homepage = "https://github.com/animesao/dck";
    license = licenses.mit;
    platforms = platforms.linux;
    maintainers = [ ];
    mainProgram = "dck";
  };
}
