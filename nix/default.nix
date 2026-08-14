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
