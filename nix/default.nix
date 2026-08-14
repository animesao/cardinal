{ lib
, stdenv
, buildGoModule
, fetchFromGitHub
, installShellFiles
, version ? "dev"
}:

stdenv.mkDerivation {
  pname = "dck";
  inherit version;

  src = ./..;

  nativeBuildInputs = [ installShellFiles ];

  # Disable unpackPhase — we build from src directly
  dontUnpack = true;

  buildPhase = ''
    export GOPATH=$TMPDIR/gopath
    export GOMODCACHE=$TMPDIR/modcache
    export GOFLAGS="-mod=mod"
    
    mkdir -p $GOPATH $GOMODCACHE
    cp -r $src/* $TMPDIR/build/
    cd $TMPDIR/build
    
    go build \
      -tags netgo \
      -ldflags="-s -w -X dck/cmd.version=${version}" \
      -o dck \
      .
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp dck $out/bin/dck
  '';

  meta = with lib; {
    description = "Lightweight container runtime for Linux";
    homepage = "https://github.com/animesao/dck";
    license = licenses.mit;
    platforms = platforms.linux;
    maintainers = [ ];
    mainProgram = "dck";
  };
}
