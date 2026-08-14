{ lib
, stdenv
, go
, installShellFiles
, version ? "dev"
}:

stdenv.mkDerivation {
  pname = "dck";
  inherit version;

  src = ./..;

  nativeBuildInputs = [ go installShellFiles ];

  dontUnpack = true;
  dontConfigure = true;

  buildPhase = ''
    export GOCACHE=$TMPDIR/gocache
    mkdir -p $GOCACHE

    cp -r $src $TMPDIR/src
    cd $TMPDIR/src

    go version
    go build \
      -tags netgo \
      -ldflags="-s -w -X dck/cmd.version=${version}" \
      -o $TMPDIR/dck \
      .
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp $TMPDIR/dck $out/bin/dck
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
