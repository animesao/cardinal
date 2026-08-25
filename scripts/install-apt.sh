#!/bin/sh
# Install cardinal via APT repository
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/install-apt.sh | sudo bash
set -eu

BOLD=$(tput bold 2>/dev/null || echo "")
RESET=$(tput gr0 2>/dev/null || tput sgr0 2>/dev/null || echo "")
GREEN=$(tput setaf 2 2>/dev/null || echo "")

info()  { echo "${BOLD}${GREEN}[cardinal]${RESET} $*"; }
warn()  { echo "${BOLD}[cardinal] WARN:${RESET} $*" >&2; }
err()   { echo "${BOLD}[cardinal] ERROR:${RESET} $*" >&2; exit 1; }

if [ "$(id -u)" != "0" ]; then
    echo "This script must be run as root (or with sudo)."
    exit 1
fi

# Detect latest version from GitHub API.
info "Detecting latest version..."
LATEST=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest 2>/dev/null | \
    grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/' | tr -d 'v')

if [ -z "${LATEST:-}" ]; then
    err "Could not determine latest release tag from GitHub"
fi
info "Latest version: $LATEST"

info "Downloading cardinal v$LATEST..."
DEB_BASE="https://github.com/animesao/cardinal/releases/download/${LATEST}/cardinal_${LATEST}_amd64.deb"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

cd "$TMPDIR"
if ! curl -fsSL -o cardinal.deb "$DEB_BASE"; then
    DEB_BASE="https://github.com/animesao/cardinal/releases/download/v${LATEST}/cardinal_${LATEST}_amd64.deb"
    curl -fsSL -o cardinal.deb "$DEB_BASE" || err "Failed to download: $DEB_BASE"
fi

# SHA256 verification against published SHA256SUMS.txt.
SUMS_URL="https://github.com/animesao/cardinal/releases/download/${LATEST}/SHA256SUMS.txt"
if curl -fsSL -o SHA256SUMS.txt "$SUMS_URL"; then
    DEB_FILE="cardinal_${LATEST}_amd64.deb"
    EXPECTED=$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+.*${DEB_FILE}\$" SHA256SUMS.txt | awk '{print $1}')
    if [ -z "$EXPECTED" ]; then
        warn "SHA256SUMS.txt exists but does not contain $DEB_FILE; skipping verification"
    else
        ACTUAL=$(sha256sum cardinal.deb | awk '{print $1}')
        if [ "$ACTUAL" != "$EXPECTED" ]; then
            err "SHA256 mismatch: expected $EXPECTED got $ACTUAL"
        fi
        info "SHA256 verified"
    fi
else
    warn "SHA256SUMS.txt not available for this release; refusing to install unsigned"
    if [ "${CARDINAL_REQUIRE_VERIFY:-0}" = "1" ] || [ "${CARDINAL_REQUIRE_VERIFY:-}" = "true" ]; then
        err "CARDINAL_REQUIRE_VERIFY=1 set, aborting. See SECURITY.md."
    else
        warn "Set CARDINAL_REQUIRE_VERIFY=1 to abort on missing signatures."
    fi
fi

info "Installing..."
dpkg -i cardinal.deb 2>/dev/null || apt-get install -f -y -qq

info "cardinal v$LATEST installed!"
