#!/bin/bash
# Generate APT repository metadata in docs/apt/.
# Called by GitHub Actions after building the .deb package.
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

VERSION=$(head -1 VERSION | tr -d '[:space:]')
APT_DIR="docs/apt"

mkdir -p "$APT_DIR"

# Copy the deb file into the APT repository.
cp "dist/dck_${VERSION}_amd64.deb" "$APT_DIR/"

# Generate the Packages index.
cd "$APT_DIR"
> Packages
for deb in *.deb; do
    SIZE=$(stat -c%s "$deb" 2>/dev/null || stat -f%z "$deb" 2>/dev/null)
    SHA256=$(sha256sum "$deb" | cut -d' ' -f1)
    {
        echo "Package: dck"
        echo "Version: ${VERSION}"
        echo "Architecture: amd64"
        echo "Maintainer: animesao <animesao@users.noreply.github.com>"
        echo "Filename: $deb"
        echo "Size: $SIZE"
        echo "SHA256: $SHA256"
        echo "Description: dck - lightweight container runtime"
        echo " No daemon. No Docker. Just containers."
        echo ""
    } >> Packages
done
gzip -9fk Packages

# Generate the unsigned Release file with SHA256 sums for Packages{, .gz}.
NOW=$(date -u +"%a, %d %b %Y %H:%M:%S UTC")
PKG_SIZE=$(stat -c%s "Packages" 2>/dev/null || stat -f%z "Packages" 2>/dev/null)
PKG_GZ_SIZE=$(stat -c%s "Packages.gz" 2>/dev/null || stat -f%z "Packages.gz" 2>/dev/null)
PKG_SHA256=$(sha256sum "Packages" | cut -d' ' -f1)
PKG_GZ_SHA256=$(sha256sum "Packages.gz" | cut -d' ' -f1)
cat > Release <<EOF
Origin: dck
Label: dck APT Repository
Suite: stable
Codename: dck
Date: $NOW
Architectures: amd64
Description: dck lightweight container runtime
SHA256:
 $PKG_SHA256 $PKG_SIZE Packages
 $PKG_GZ_SHA256 $PKG_GZ_SIZE Packages.gz
EOF

# Sign Release with the maintainer GPG key. In CI the key is provided as a
# private key + passphrase via repository Secrets. When either is missing
# the unsigned Release is published alongside a clear warning; the consumer
# script (add-apt-repo.sh) refuses to install in that case.
GPG_PRIVATE_KEY="${GPG_PRIVATE_KEY:-}"
if [ -n "$GPG_PRIVATE_KEY" ]; then
    GNUPGHOME="$(mktemp -d)"
    chmod 0700 "$GNUPGHOME"
    export GNUPGHOME
    echo "$GPG_PRIVATE_KEY" | gpg --import
    GPG_PASSPHRASE="${GPG_PASSPHRASE:-}"
    if [ -n "$GPG_PASSPHRASE" ]; then
        gpg --batch --pinentry-mode loopback --passphrase "$GPG_PASSPHRASE" \
            --armor --detach-sign --sign-with default \
            --output Release.gpg Release
    else
        gpg --batch --yes --armor --detach-sign \
            --output Release.gpg Release
    fi
    rm -rf "$GNUPGHOME"
    echo "Signed Release.gpg published"
else
    # Maintainer-flow convenience: cleartext InRelease requires the key.
    cat > InRelease <<'EOF'
UNTITLED
EOF
    echo "WARNING: GPG_PRIVATE_KEY not provided; Release file is UNSIGNED." >&2
    echo "Set the repository secret GPG_PRIVATE_KEY (and optionally" >&2
    echo "GPG_PASSPHRASE) to publish a signed Release.gpg." >&2
fi

echo "APT repo updated at $APT_DIR"
