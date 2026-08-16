#!/bin/sh
# Add the dck APT repository.
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/dck/main/scripts/add-apt-repo.sh | sudo bash
#
# The repository is signed with the maintainer's key (see docs/apt/keyring.gpg).
# APT verifies the Release file signature on every `apt update`, so packages
# cannot be silently replaced by a compromised CDN.
set -e

if [ "$(id -u)" != "0" ]; then
    echo "This script must be run as root (or with sudo)."
    exit 1
fi

KEYRING_DIR="/etc/apt/keyrings"
KEYRING_FILE="${KEYRING_DIR}/dck-archive-keyring.gpg"
SOURCES_FILE="/etc/apt/sources.list.d/dck.sources"
REPO_URL="https://animesao.github.io/dck/apt"
KEY_URL="https://animesao.github.io/dck/keyring.gpg"

# Modern apt supports /etc/apt/keyrings/ with dearmored single-key files.
# Older releases fall back to apt-key (deprecated but still functional).
echo "Installing dck APT signing key..."
mkdir -p "$KEYRING_DIR"
chmod 0755 "$KEYRING_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$KEY_URL" -o "$KEYRING_FILE.tmp"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$KEYRING_FILE.tmp" "$KEY_URL"
else
    echo "Neither curl nor wget is installed; cannot fetch signing key." >&2
    exit 1
fi

if [ ! -s "$KEYRING_FILE.tmp" ]; then
    echo "Downloaded signing keyring is empty; aborting." >&2
    rm -f "$KEYRING_FILE.tmp"
    exit 1
fi

# gpg --dearmor if the file is armored (starts with "-----BEGIN PGP").
if head -c 5 "$KEYRING_FILE.tmp" | grep -q -- "-----B"; then
    if command -v gpg >/dev/null 2>&1; then
        gpg --dearmor < "$KEYRING_FILE.tmp" > "$KEYRING_FILE"
        rm -f "$KEYRING_FILE.tmp"
    else
        echo "Signing key is armored but gpg is not installed; please install gnupg." >&2
        exit 1
    fi
else
    mv "$KEYRING_FILE.tmp" "$KEYRING_FILE"
fi
chmod 0644 "$KEYRING_FILE"

# The deb822-format sources.list entry is the modern (and only correct)
# way to attach a per-repo signing key.
cat > "$SOURCES_FILE" <<EOF
Types: deb
URIs: ${REPO_URL}
Suites: stable
Components: main
Signed-By: ${KEYRING_FILE}
EOF
chmod 0644 "$SOURCES_FILE"

# Remove any old, insecure legacy sources.list entry.
if [ -f /etc/apt/sources.list.d/dck.list ]; then
    echo "Removing legacy /etc/apt/sources.list.d/dck.list (was using [trusted=yes])"
    rm -f /etc/apt/sources.list.d/dck.list
fi

echo "Updating package lists..."
apt update -qq

echo "Installing dck..."
apt install -y dck
