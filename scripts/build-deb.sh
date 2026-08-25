#!/bin/sh
# Build cardinal .deb package for Debian/Ubuntu
set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

VERSION="${VERSION:-$(cat VERSION 2>/dev/null | head -1 | tr -d '[:space:]')}"
: "${VERSION:=1.10.0}"
# Strip leading non-digit prefix for Debian version compliance (e.g. "<1.19.0" -> "1.19.0")
VERSION_DEB="$(echo "$VERSION" | sed 's/^[^0-9]*//')"
ARCH="${ARCH:-amd64}"

echo "==> Building cardinal v$VERSION for linux/$ARCH..."

BINARY="cardinal-linux-${ARCH}"

# Build the binary only if it doesn't already exist
if [ ! -f "$BINARY" ]; then
    if ! command -v go >/dev/null 2>&1; then
        echo "Error: Go not found."
        exit 1
    fi
    echo "==> Compiling..."
    CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -ldflags="-s -w -X cardinal/cmd.version=$VERSION" -o "$BINARY" .
fi

mkdir -p dist

# Prepare package directory
PKG_DIR="packaging/deb"
BIN_DEST="$PKG_DIR/usr/bin"

# Copy binary
install -d "$BIN_DEST"
install -m 755 "$BINARY" "$BIN_DEST/cardinal"

# Set permissions
chmod 755 "$PKG_DIR/DEBIAN/postinst"
chmod 755 "$PKG_DIR/DEBIAN/prerm"

# Update version and architecture in control file
sed -i "s/^Version: .*/Version: $VERSION_DEB/" "$PKG_DIR/DEBIAN/control"
sed -i "s/^Architecture: .*/Architecture: $ARCH/" "$PKG_DIR/DEBIAN/control"

# Build .deb
echo "==> Building .deb package..."
fakeroot dpkg-deb --build "$PKG_DIR" "dist/cardinal_${VERSION_DEB}_${ARCH}.deb" 2>/dev/null || \
dpkg-deb --build "$PKG_DIR" "dist/cardinal_${VERSION_DEB}_${ARCH}.deb"

# Clean up binary from package dir
rm -f "$BIN_DEST/cardinal"

echo ""
echo "   Package: dist/cardinal_${VERSION_DEB}_${ARCH}.deb"
echo "   Size:    $(ls -lh "dist/cardinal_${VERSION_DEB}_${ARCH}.deb" | awk '{print $5}')"
echo ""
echo "Install:"
echo "   sudo dpkg -i dist/cardinal_${VERSION_DEB}_${ARCH}.deb"
echo "   sudo apt-get install -f   # install dependencies"
