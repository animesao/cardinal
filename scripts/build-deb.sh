#!/bin/sh
# Build cardinal .deb package for Debian/Ubuntu
set -eu

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

umask 022

# --- version ---------------------------------------------------------------
if [ -n "${VERSION:-}" ]; then
    :
elif [ -f VERSION ]; then
    VERSION="$(head -n1 VERSION | tr -d '[:space:]')"
else
    VERSION="1.10.0"
fi
: "${VERSION:=1.10.0}"

# Strip leading non-digit prefix (e.g. "v1.19.0" -> "1.19.0", "<1.19.0" -> "1.19.0").
VERSION_DEB="$(printf '%s' "$VERSION" | sed 's/^[^0-9]*//')"
if [ -z "$VERSION_DEB" ]; then
    echo "Error: cannot derive a numeric Debian version from '$VERSION'" >&2
    exit 1
fi
case "$VERSION_DEB" in
    *[!0-9A-Za-z.+~:-]*)
        echo "Error: invalid character in version '$VERSION_DEB'" >&2
        exit 1
        ;;
esac

# --- arch ------------------------------------------------------------------
ARCH="${ARCH:-amd64}"
case "$ARCH" in
    amd64|arm64|armhf|i386) ;;
    *) echo "Error: unsupported ARCH '$ARCH' (expected amd64/arm64/armhf/i386)" >&2; exit 1 ;;
esac

echo "==> Building cardinal v$VERSION for linux/$ARCH..."

BINARY="cardinal-linux-${ARCH}"

# --- build -----------------------------------------------------------------
if [ ! -f "$BINARY" ]; then
    if ! command -v go >/dev/null 2>&1; then
        echo "Error: Go not found." >&2
        exit 1
    fi
    echo "==> Compiling..."
    CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
        go build -trimpath -buildvcs=false \
        -ldflags="-s -w -X cardinal/cmd.version=$VERSION" \
        -o "$BINARY" .
fi

# --- package layout --------------------------------------------------------
PKG_SRC="packaging/deb"
if [ ! -f "$PKG_SRC/DEBIAN/control" ]; then
    echo "Error: $PKG_SRC/DEBIAN/control not found" >&2
    exit 1
fi

WORK="$(mktemp -d 2>/dev/null || mktemp -d -t cardinal-deb)"
trap 'rm -rf "$WORK"' EXIT INT TERM

PKG_DIR="$WORK/deb"
cp -a "$PKG_SRC" "$PKG_DIR"

BIN_DEST="$PKG_DIR/usr/bin"
install -d "$BIN_DEST"
install -m 755 "$BINARY" "$BIN_DEST/cardinal"

for f in postinst prerm; do
    p="$PKG_DIR/DEBIAN/$f"
    if [ -f "$p" ]; then
        chmod 755 "$p"
    else
        echo "warning: $p missing" >&2
    fi
done

sed -i "s/^Version: .*/Version: $VERSION_DEB/" "$PKG_DIR/DEBIAN/control"
sed -i "s/^Architecture: .*/Architecture: $ARCH/" "$PKG_DIR/DEBIAN/control"

# --- build .deb ------------------------------------------------------------
mkdir -p dist
DEB="dist/cardinal_${VERSION_DEB}_${ARCH}.deb"
rm -f "$DEB"

if ! command -v dpkg-deb >/dev/null 2>&1; then
    echo "Error: dpkg-deb not found" >&2
    exit 1
fi

echo "==> Building .deb package..."
if command -v fakeroot >/dev/null 2>&1; then
    fakeroot dpkg-deb --root-owner-group --build "$PKG_DIR" "$DEB"
else
    echo "warning: fakeroot not found; building without it" >&2
    dpkg-deb --root-owner-group --build "$PKG_DIR" "$DEB"
fi

SIZE="$(du -h "$DEB" | cut -f1)"
SUM="$(sha256sum "$DEB" | awk '{print $1}')"

echo ""
echo "   Package: $DEB"
echo "   Size:    $SIZE"
echo "   SHA256:  $SUM"
echo ""
echo "Install:"
echo "   sudo dpkg -i $DEB"
echo "   sudo apt-get install -f   # install dependencies"
