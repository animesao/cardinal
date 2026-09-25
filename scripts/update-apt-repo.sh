#!/bin/bash
# Generate APT repository metadata in docs/apt/.
# Called by GitHub Actions after building the .deb package.
#
# Layout: flat repository.
#   URI + Suites: ./ + Components: (empty)
# The matching sources.list entry must use "Suites: ./" (see add-apt-repo.sh).
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

umask 022

[ -f VERSION ] || { echo "VERSION file not found" >&2; exit 1; }
VERSION=$(head -n1 VERSION | tr -d '[:space:]')
[ -n "$VERSION" ] || { echo "VERSION is empty" >&2; exit 1; }

ARCH="${ARCH:-amd64}"
APT_DIR="docs/apt"
DEB_SRC="dist/cardinal_${VERSION}_${ARCH}.deb"

[ -f "$DEB_SRC" ] || { echo "Missing $DEB_SRC" >&2; exit 1; }

mkdir -p "$APT_DIR"

# Clean previously generated metadata so stale InRelease/Release.gpg
# cannot survive an unsigned run.
rm -f "$APT_DIR"/InRelease "$APT_DIR"/Release.gpg "$APT_DIR"/Packages "$APT_DIR"/Packages.gz

# Publish only the current .deb (single-version flat repo).
find "$APT_DIR" -maxdepth 1 -name 'cardinal_*.deb' -delete
cp "$DEB_SRC" "$APT_DIR/"

cd "$APT_DIR"

shopt -s nullglob
debs=( *.deb )
if (( ${#debs[@]} == 0 )); then
    echo "No .deb files in $APT_DIR" >&2
    exit 1
fi

: > Packages
for deb in "${debs[@]}"; do
    # Derive version/arch from the file name, not from the root VERSION file.
    if [[ "$deb" =~ ^cardinal_(.+)_([a-z0-9]+)\.deb$ ]]; then
        v="${BASH_REMATCH[1]}"
        a="${BASH_REMATCH[2]}"
    else
        echo "Unexpected .deb name: $deb" >&2
        exit 1
    fi

    SIZE=$(stat -c%s "$deb" 2>/dev/null || stat -f%z "$deb")
    SHA256=$(sha256sum "$deb" | cut -d' ' -f1)

    {
        echo "Package: cardinal"
        echo "Version: $v"
        echo "Architecture: $a"
        echo "Maintainer: animesao <animesao@users.noreply.github.com>"
        echo "Priority: optional"
        echo "Section: utils"
        echo "Filename: $deb"
        echo "Size: $SIZE"
        echo "SHA256: $SHA256"
        echo "Description: cardinal - lightweight container runtime"
        echo " No daemon. No Docker. Just containers."
        echo ""
    } >> Packages
done

gzip -9n -k -f Packages

# --- Release file ----------------------------------------------------------
if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then
    NOW=$(LC_ALL=C date -u -d "@${SOURCE_DATE_EPOCH}" +"%a, %d %b %Y %H:%M:%S UTC" 2>/dev/null \
          || LC_ALL=C date -u -r "${SOURCE_DATE_EPOCH}" +"%a, %d %b %Y %H:%M:%S UTC")
else
    NOW=$(LC_ALL=C date -u +"%a, %d %b %Y %H:%M:%S UTC")
fi

PKG_SIZE=$(stat -c%s Packages 2>/dev/null || stat -f%z Packages)
PKG_GZ_SIZE=$(stat -c%s Packages.gz 2>/dev/null || stat -f%z Packages.gz)
PKG_SHA256=$(sha256sum Packages | cut -d' ' -f1)
PKG_GZ_SHA256=$(sha256sum Packages.gz | cut -d' ' -f1)

cat > Release <<EOF
Origin: cardinal
Label: cardinal APT Repository
Suite: stable
Codename: cardinal
Date: $NOW
Architectures: $ARCH
Components: main
Description: cardinal lightweight container runtime
SHA256:
 $PKG_SHA256 $PKG_SIZE Packages
 $PKG_GZ_SHA256 $PKG_GZ_SIZE Packages.gz
EOF

# --- signing ---------------------------------------------------------------
GPG_PRIVATE_KEY="${GPG_PRIVATE_KEY:-}"
GPG_PASSPHRASE="${GPG_PASSPHRASE:-}"

if [ -n "$GPG_PRIVATE_KEY" ]; then
    GNUPGHOME="$(mktemp -d)"
    chmod 0700 "$GNUPGHOME"
    export GNUPGHOME
    trap 'rm -rf "$GNUPGHOME"' EXIT INT TERM

    printf '%s\n' "$GPG_PRIVATE_KEY" | gpg --batch --import

    SIGN_ARGS=( --batch --yes --armor --detach-sign --output Release.gpg )
    if [ -n "$GPG_PASSPHRASE" ]; then
        SIGN_ARGS=( --batch --yes --pinentry-mode loopback \
                    --passphrase "$GPG_PASSPHRASE" \
                    --armor --detach-sign --output Release.gpg )
    fi
    gpg "${SIGN_ARGS[@]}" Release

    # Optional: also emit cleartext-signed InRelease (modern APT prefers it).
    if [ "${GPG_EMIT_INRELEASE:-1}" = "1" ]; then
        if [ -n "$GPG_PASSPHRASE" ]; then
            gpg --batch --yes --pinentry-mode loopback \
                --passphrase "$GPG_PASSPHRASE" \
                --clearsign --output InRelease Release
        else
            gpg --batch --yes --clearsign --output InRelease Release
        fi
    fi

    echo "Signed Release.gpg published"
else
    echo "WARNING: GPG_PRIVATE_KEY not provided; Release file is UNSIGNED." >&2
    echo "Set the repository secret GPG_PRIVATE_KEY (and optionally" >&2
    echo "GPG_PASSPHRASE) to publish a signed Release.gpg." >&2
fi

echo "APT repo updated at $APT_DIR"
