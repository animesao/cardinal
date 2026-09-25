#!/bin/sh
# Add the cardinal APT repository.
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/add-apt-repo.sh | sudo bash
#
# The repository is signed with the maintainer's key (see docs/apt/keyring.gpg).
# APT verifies the Release file signature on every `apt update`, so packages
# cannot be silently replaced by a compromised CDN.
set -eu

if [ "$(id -u)" != "0" ]; then
    echo "This script must be run as root (or with sudo)." >&2
    exit 1
fi

if ! command -v apt >/dev/null 2>&1; then
    echo "apt not found; this script is intended for Debian/Ubuntu systems." >&2
    exit 1
fi

KEYRING_DIR="/etc/apt/keyrings"
KEYRING_FILE="${KEYRING_DIR}/cardinal-archive-keyring.gpg"
SOURCES_FILE="/etc/apt/sources.list.d/cardinal.sources"
REPO_URL="https://animesao.github.io/cardinal/apt"
KEY_URL="https://animesao.github.io/cardinal/keyring.gpg"

TMP_KEY="${KEYRING_FILE}.tmp"
NEW_KEY="${KEYRING_FILE}.new"

cleanup() {
    rm -f "$TMP_KEY" "$NEW_KEY"
}
trap cleanup EXIT INT TERM

# Modern apt supports /etc/apt/keyrings/ with a single (binary) key file.
echo "Installing cardinal APT signing key..."
mkdir -p "$KEYRING_DIR"
chmod 0755 "$KEYRING_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 10 --max-time 60 "$KEY_URL" -o "$TMP_KEY"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMP_KEY" --timeout=60 "$KEY_URL"
else
    echo "Neither curl nor wget is installed; cannot fetch signing key." >&2
    exit 1
fi

if [ ! -s "$TMP_KEY" ]; then
    echo "Downloaded signing keyring is empty; aborting." >&2
    exit 1
fi

# gpg --dearmor if the file is armored (starts with "-----BEGIN PGP PUBLIC KEY").
if head -c 27 "$TMP_KEY" | grep -q '^-----BEGIN PGP PUBLIC KEY'; then
    if command -v gpg >/dev/null 2>&1; then
        gpg --dearmor < "$TMP_KEY" > "$NEW_KEY"
        mv "$NEW_KEY" "$KEYRING_FILE"
    else
        echo "Signing key is armored but gpg is not installed; please install gnupg." >&2
        exit 1
    fi
else
    mv "$TMP_KEY" "$KEYRING_FILE"
fi

if [ ! -s "$KEYRING_FILE" ]; then
    echo "Resulting keyring is empty; aborting." >&2
    exit 1
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
if [ -f /etc/apt/sources.list.d/cardinal.list ]; then
    echo "Removing legacy /etc/apt/sources.list.d/cardinal.list (was using [trusted=yes])"
    rm -f /etc/apt/sources.list.d/cardinal.list
fi

echo "Updating package lists..."
if ! apt update -qq; then
    echo "warning: 'apt update' returned a non-zero status; continuing anyway." >&2
fi

echo "Installing cardinal..."
DEBIAN_FRONTEND=noninteractive apt install -y cardinal
