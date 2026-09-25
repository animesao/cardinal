#!/bin/sh
# Install cardinal .deb from GitHub Releases
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/install-apt.sh | sudo bash
set -eu

BOLD=$(tput bold 2>/dev/null || echo "")
RESET=$(tput sgr0 2>/dev/null || echo "")
GREEN=$(tput setaf 2 2>/dev/null || echo "")

info() { echo "${BOLD}${GREEN}[cardinal]${RESET} $*"; }
warn() { echo "${BOLD}[cardinal] WARN:${RESET} $*" >&2; }
err()  { echo "${BOLD}[cardinal] ERROR:${RESET} $*" >&2; exit 1; }

[ "$(id -u)" = "0" ] || err "must be run as root (or with sudo)"

command -v curl     >/dev/null 2>&1 || err "curl not found"
command -v dpkg     >/dev/null 2>&1 || err "dpkg not found"
command -v apt-get  >/dev/null 2>&1 || err "apt-get not found"

ARCH="$(dpkg --print-architecture 2>/dev/null || echo amd64)"
case "$ARCH" in
    amd64|arm64|armhf|i386) ;;
    *) err "unsupported architecture: $ARCH" ;;
esac

# --- detect latest tag ------------------------------------------------------
info "Detecting latest version..."
RAW_TAG=$(curl -fsSL --connect-timeout 10 --max-time 60 \
        https://api.github.com/repos/animesao/cardinal/releases/latest 2>/dev/null \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)

[ -n "${RAW_TAG:-}" ] || err "could not determine latest release tag from GitHub"

case "$RAW_TAG" in
    *[!0-9A-Za-z._-]*) err "suspicious tag from GitHub: $RAW_TAG" ;;
esac

TAG="$RAW_TAG"
LATEST="${RAW_TAG#v}"

info "Latest version: $LATEST (tag $TAG)"

# --- download ---------------------------------------------------------------
DEB_FILE="cardinal_${LATEST}_${ARCH}.deb"
BASE_URL="https://github.com/animesao/cardinal/releases/download/${TAG}"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT INT TERM
cd "$WORKDIR"

info "Downloading $DEB_FILE..."
curl -fsSL --connect-timeout 10 --max-time 300 \
    -o cardinal.deb "${BASE_URL}/${DEB_FILE}" \
    || err "failed to download ${BASE_URL}/${DEB_FILE}"

# sanity: is it really a .deb?
if ! head -c4 cardinal.deb | grep -q '!<arch>'; then
    err "downloaded file is not a .deb archive"
fi

# --- SHA256 verification ----------------------------------------------------
SUMS_URL="${BASE_URL}/SHA256SUMS.txt"
if curl -fsSL --connect-timeout 10 --max-time 60 -o SHA256SUMS.txt "$SUMS_URL"; then
    EXPECTED=$(grep -E "[[:space:]]\*?${DEB_FILE}\$" SHA256SUMS.txt \
        | awk '{print $1}' | head -n1)
    if [ -z "$EXPECTED" ]; then
        warn "SHA256SUMS.txt exists but does not list $DEB_FILE"
        VERIFIED=0
    else
        ACTUAL=$(sha256sum cardinal.deb | awk '{print $1}')
        [ "$ACTUAL" = "$EXPECTED" ] \
            || err "SHA256 mismatch: expected $EXPECTED got $ACTUAL"
        info "SHA256 verified"
        VERIFIED=1
    fi
else
    warn "SHA256SUMS.txt not available for this release"
    VERIFIED=0
fi

if [ "${VERIFIED:-0}" != "1" ]; then
    case "${CARDINAL_REQUIRE_VERIFY:-0}" in
        1|true|TRUE|True|yes|YES)
            err "CARDINAL_REQUIRE_VERIFY is set but no verified checksum was available" ;;
        *)
            warn "installing without checksum verification; set CARDINAL_REQUIRE_VERIFY=1 to abort" ;;
    esac
fi

# --- install ----------------------------------------------------------------
info "Installing..."
if ! dpkg -i cardinal.deb; then
    warn "dpkg reported errors; resolving dependencies with apt-get -f"
    apt-get install -f -y -qq
fi

dpkg -s cardinal >/dev/null 2>&1 \
    || err "cardinal is not installed after dpkg/apt"

info "cardinal v$LATEST installed"
