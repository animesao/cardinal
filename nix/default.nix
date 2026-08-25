{ lib
, stdenv
, go
, installShellFiles
, version ? "dev"
}:

stdenv.mkDerivation {
  pname = "cardinal";
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
      -ldflags="-s -w -X cardinal/cmd.version=${version}" \
      -o $TMPDIR/cardinal \
      .
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp $TMPDIR/cardinal $out/bin/cardinal
  '';

  meta = with lib; {
    description = "Lightweight container runtime for Linux";
    homepage = "https://github.com/animesao/cardinal";
    license = licenses.mit;
    platforms = platforms.linux;
    maintainers = [ ];
    mainProgram = "cardinal";
  };
}
