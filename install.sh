#!/usr/bin/env bash
set -euo pipefail

# cardinal Container Runtime Installer — Universal Linux
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
#
# Downloads from the site mirror first (works where GitHub is slow/blocked);
# falls back to GitHub Releases. Every download has a hard timeout and an
# IPv4 retry, so the installer can never hang silently.

REPO="animesao/cardinal"
CARDINAL_BIN="/usr/local/bin/cardinal"
# Site mirror first; set CARDINAL_MIRROR="" to force GitHub-only.
CARDINAL_MIRROR="${CARDINAL_MIRROR:-https://cardinal.spcfy.eu/downloads/cardinal}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[i]${NC} $1"; }

# ---- robust download: retries, hard timeouts, IPv4 fallback (never hangs) ----
download() {
  local url="$1" dest="$2"
  local tmp="${dest}.tmp.$$"
  local try
  for try in 1 2 3; do
    # Try normally first, then force IPv4 — some networks black-hole IPv6 and
    # stall transfers without any error (the classic "installer hangs" case).
    if curl -fsSL --retry 2 --connect-timeout 15 --max-time 300 "${url}" -o "${tmp}" \
      || curl -4 -fsSL --retry 2 --connect-timeout 15 --max-time 300 "${url}" -o "${tmp}"; then
      if [ -s "${tmp}" ]; then
        mv -f "${tmp}" "${dest}"
        return 0
      fi
      echo "warning: downloaded empty file from ${url}" >&2
      rm -f "${tmp}"
    else
      echo "warning: download attempt ${try}/3 failed for ${url}" >&2
      rm -f "${tmp}"
    fi
    sleep 2
  done
  echo "error: failed to download ${url} after 3 attempts" >&2
  return 1
}

if [[ $EUID -ne 0 ]]; then err "Must run as root: sudo bash install.sh"; fi
if [[ ! -f /etc/os-release ]]; then err "Cannot detect OS — /etc/os-release not found"; fi
source /etc/os-release

log "OS: ${PRETTY_NAME:-$ID $VERSION_ID}"

# ---- Detect architecture ----
ARCH="amd64"
case "$(uname -m)" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l)  ARCH="armv6" ;;
  *)       ARCH="amd64"; warn "Unknown arch $(uname -m), defaulting to amd64" ;;
esac
info "Architecture: $ARCH"

# ---- Detect package manager and distro family ----
PKG_MGR=""
PKG_INSTALL=""
PKG_DEPS=""
SERVICE_RELOAD=""

case "${ID:-}" in
  ubuntu|debian|linuxmint|pop|elementary|zorin|kali|raspbian)
    PKG_MGR="apt"
    PKG_INSTALL="apt-get install -y -qq"
    PKG_DEPS="curl tar gzip sudo ufw util-linux iproute2 iptables procps"
    SERVICE_RELOAD="systemctl daemon-reload"
    ;;
  arch|manjaro|endeavouros|garuda|arco|athena)
    PKG_MGR="pacman"
    PKG_INSTALL="pacman -S --noconfirm --needed"
    PKG_DEPS="curl tar gzip sudo iptables-nft procps-ng iproute2"
    SERVICE_RELOAD="systemctl daemon-reload"
    ;;
  fedora|rhel|centos|rocky|alma|nobara|rawhide)
    PKG_MGR="dnf"
    PKG_INSTALL="dnf install -y -q"
    PKG_DEPS="curl tar gzip sudo iptables procps-ng iproute"
    SERVICE_RELOAD="systemctl daemon-reload"
    # Fall back to yum on older RHEL
    if command -v yum &>/dev/null && ! command -v dnf &>/dev/null; then
      PKG_MGR="yum"
      PKG_INSTALL="yum install -y -q"
    fi
    ;;
  opensuse*|sles|opensuse-tumbleweed)
    PKG_MGR="zypper"
    PKG_INSTALL="zypper install -y -n"
    PKG_DEPS="curl tar gzip sudo iptables procps iproute2"
    SERVICE_RELOAD="systemctl daemon-reload"
    ;;
  alpine)
    PKG_MGR="apk"
    PKG_INSTALL="apk add --no-cache"
    PKG_DEPS="curl tar gzip sudo iptables procps iproute2"
    SERVICE_RELOAD=""
    ;;
  void)
    PKG_MGR="xbps"
    PKG_INSTALL="xbps-install -Sy"
    PKG_DEPS="curl tar gzip sudo iptables procps iproute2"
    SERVICE_RELOAD=""
    ;;
  *)
    warn "Unknown distro: ${ID:-unknown}. Will try to install binary only."
    PKG_MGR="manual"
    PKG_INSTALL=""
    PKG_DEPS=""
    SERVICE_RELOAD=""
    ;;
esac

if [[ "$PKG_MGR" == "manual" ]]; then
  warn "Package manager not detected. Will download binary directly."
else
  info "Package manager: $PKG_MGR"
fi

# ---- Install dependencies ----
if [[ -n "$PKG_DEPS" ]]; then
  log "Installing dependencies..."
  case "$PKG_MGR" in
    apt)    apt-get update -qq ;;
    pacman) pacman -Sy --noconfirm --needed 2>/dev/null || true ;;
  esac
  $PKG_INSTALL $PKG_DEPS 2>/dev/null || warn "Some dependencies may be missing"
fi

# ---- Detect latest version: site mirror first, then GitHub API ----
log "Fetching latest release..."
LATEST_TAG=""
if [[ -n "$CARDINAL_MIRROR" ]]; then
  MV="$(curl -sfL --retry 2 --connect-timeout 12 --max-time 25 "$CARDINAL_MIRROR/VERSION" 2>/dev/null | tail -1 | tr -d '[:space:]')"
  if [[ -n "$MV" ]] && [[ "$MV" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    LATEST_TAG="${MV}"
    [[ "$LATEST_TAG" != v* ]] && LATEST_TAG="v$LATEST_TAG"
    log "Latest version (mirror): $LATEST_TAG"
  else
    log "Mirror has no version ($CARDINAL_MIRROR) — using GitHub API"
  fi
fi
if [[ -z "$LATEST_TAG" ]]; then
  log "Fetching latest release from GitHub..."
  LATEST_TAG=$(curl -sfL --connect-timeout 12 --max-time 25 "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | cut -d'"' -f4)
fi
if [[ -z "$LATEST_TAG" ]]; then
  err "Could not detect latest release (mirror and GitHub both unreachable). Check https://github.com/$REPO/releases"
fi
log "Latest version: $LATEST_TAG"

# ---- Download binary and verify SHA256 checksum ----
TMP_BIN="$(mktemp -t cardinal.XXXXXX)"
TMP_ARCHIVE="$(mktemp -t cardinal-archive.XXXXXX)"
TMP_SUMS="$(mktemp -t cardinal-sums.XXXXXX)"
VERSION="${LATEST_TAG#v}"
ARCHIVE_NAME="cardinal_${VERSION}_linux_${ARCH}.tar.gz"
CHECKSUMS_NAME="cardinal_${VERSION}_checksums.txt"
log "Downloading cardinal ${LATEST_TAG} (${ARCH})..."

dl_ok=0
if [[ -n "$CARDINAL_MIRROR" ]]; then
  if download "$CARDINAL_MIRROR/$LATEST_TAG/$ARCHIVE_NAME" "$TMP_ARCHIVE"; then
    dl_ok=1
  else
    log "Mirror download failed — falling back to GitHub"
  fi
fi
if [[ "$dl_ok" = "0" ]]; then
  download "https://github.com/$REPO/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}" "$TMP_ARCHIVE" \
    || err "Download failed: ${ARCHIVE_NAME}"
fi

# Verify archive checksum (mirror SHA256SUMS first, then GitHub checksums file)
SUMS_URL="https://github.com/$REPO/releases/download/${LATEST_TAG}/${CHECKSUMS_NAME}"
SUMS_SRC=""
if [[ "$dl_ok" = "1" ]] && download "$CARDINAL_MIRROR/$LATEST_TAG/SHA256SUMS" "$TMP_SUMS"; then
  SUMS_SRC="mirror"
elif download "$SUMS_URL" "$TMP_SUMS"; then
  SUMS_SRC="github"
fi
if [[ -n "$SUMS_SRC" ]]; then
  EXPECTED="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+${ARCHIVE_NAME}\$" "$TMP_SUMS" | awk '{print $1}')"
  if [[ -z "$EXPECTED" ]]; then
    warn "Checksums file ($SUMS_SRC) does not contain ${ARCHIVE_NAME}; cannot verify"
  else
    ACTUAL="$(sha256sum "$TMP_ARCHIVE" | awk '{print $1}')"
    if [[ "$ACTUAL" != "$EXPECTED" ]]; then
      rm -f "$TMP_BIN" "$TMP_ARCHIVE" "$TMP_SUMS"
      err "SHA256 mismatch: expected ${EXPECTED}, got ${ACTUAL}"
    fi
    log "SHA256 verified (${SUMS_SRC}): ${ACTUAL}"
  fi
else
  warn "Checksums file not available for this release; skipping verification"
fi

# Extract binary from tar.gz
tar -xzf "$TMP_ARCHIVE" -C "$(dirname "$TMP_BIN")" --strip-components=0 cardinal 2>/dev/null \
  || tar -xzf "$TMP_ARCHIVE" -C /tmp cardinal 2>/dev/null && cp /tmp/cardinal "$TMP_BIN"
rm -f "$TMP_ARCHIVE" /tmp/cardinal

if [[ ! -s "$TMP_BIN" ]]; then
  rm -f "$TMP_BIN" "$TMP_SUMS"
  err "Failed to extract cardinal binary from archive"
fi

install -m 0755 "$TMP_BIN" "$CARDINAL_BIN"
rm -f "$TMP_BIN" "$TMP_SUMS"
log "Binary installed: $CARDINAL_BIN"

# ---- Verify binary works (check for glibc error) ----
if ! "$CARDINAL_BIN" --version &>/dev/null; then
  warn "Binary failed to run (likely glibc mismatch). Building from source..."
  if command -v go &>/dev/null; then
    log "Building cardinal from source..."
    TMPDIR=$(mktemp -d)
    timeout 240 git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR" 2>/dev/null || {
      err "Git clone failed. Install Go manually and run: CGO_ENABLED=0 go build"
    }
    cd "$TMPDIR"
    CGO_ENABLED=0 go build -tags netgo -installsuffix netgo -ldflags="-s -w" -o cardinal .
    cp cardinal "$CARDINAL_BIN"
    chmod +x "$CARDINAL_BIN"
    cd /
    rm -rf "$TMPDIR"
    log "Built from source: $CARDINAL_BIN"
  else
    warn "Go not installed. Installing Go to build from source..."
    GO_VER="1.23.4"
    curl -fsSL --connect-timeout 15 --max-time 300 "https://go.dev/dl/go${GO_VER}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz \
      || err "Go download failed — install Go manually and re-run"
    tar -C /usr/local -xzf /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    TMPDIR=$(mktemp -d)
    timeout 240 git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR" \
      || err "Git clone failed. Install Go manually and run: CGO_ENABLED=0 go build"
    cd "$TMPDIR"
    CGO_ENABLED=0 /usr/local/go/bin/go build -tags netgo -installsuffix netgo -ldflags="-s -w" -o cardinal .
    cp cardinal "$CARDINAL_BIN"
    chmod +x "$CARDINAL_BIN"
    cd /
    rm -rf "$TMPDIR" /tmp/go.tar.gz
    log "Built from source: $CARDINAL_BIN"
  fi
fi

# ---- Enable IP forwarding ----
if [[ -f /proc/sys/net/ipv4/ip_forward ]]; then
  echo 1 > /proc/sys/net/ipv4/ip_forward
  grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf 2>/dev/null || \
    echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
  log "IP forwarding enabled"
fi

# ---- Firewall (if available) ----
if command -v ufw &>/dev/null; then
  ufw allow 22/tcp 2>/dev/null || true
  ufw --force enable 2>/dev/null || true
  log "UFW configured (allow SSH)"
elif command -v firewall-cmd &>/dev/null; then
  firewall-cmd --permanent --add-service=ssh 2>/dev/null || true
  firewall-cmd --reload 2>/dev/null || true
  log "firewalld configured (allow SSH)"
elif command -v iptables &>/dev/null; then
  iptables -A INPUT -p tcp --dport 22 -j ACCEPT 2>/dev/null || true
  log "iptables configured (allow SSH)"
fi

# ---- Bootstrap systemd service ----
if [[ -d /run/systemd/system ]]; then
  log "Installing systemd service..."
  "$CARDINAL_BIN" bootstrap --install 2>/dev/null || true
  if [[ -n "$SERVICE_RELOAD" ]]; then
    $SERVICE_RELOAD 2>/dev/null || true
  fi
fi

# ---- Verify ----
log "Verifying installation..."
if command -v cardinal &>/dev/null; then
  log "cardinal installed: $(cardinal --version 2>/dev/null || echo 'ok')"
else
  warn "cardinal not found in PATH — ensure $CARDINAL_BIN is accessible"
fi

# ---- Done ----
echo ""
log "═══════════════════════════════════════════════"
log "  cardinal installed successfully!"
log "═══════════════════════════════════════════════"
log ""
log "  Quick start:"
log "    cardinal pull alpine"
log "    cardinal run --rm alpine echo hello"
log "    cardinal --help"
log ""
log "  Docs:  https://github.com/$REPO"
echo ""
