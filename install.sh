#!/usr/bin/env bash
set -euo pipefail

# dck Container Runtime Installer — Universal Linux
# Usage: curl -sSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash

REPO="animesao/dck"
DCK_BIN="/usr/local/bin/dck"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[i]${NC} $1"; }

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

# ---- Detect latest version ----
log "Fetching latest release..."
LATEST_TAG=$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [[ -z "$LATEST_TAG" ]]; then
  err "Could not detect latest release. Check https://github.com/$REPO/releases"
fi
log "Latest version: $LATEST_TAG"

# ---- Download binary and verify SHA256 checksum ----
TMP_BIN="$(mktemp -t dck.XXXXXX)"
TMP_SUMS="$(mktemp -t dck-sums.XXXXXX)"
log "Downloading dck ${LATEST_TAG} (${ARCH})..."
curl -fsSL "https://github.com/$REPO/releases/download/${LATEST_TAG}/dck-linux-${ARCH}" \
  -o "$TMP_BIN"

# SHA256SUMS.txt is published alongside every release; verify by default.
SUMS_URL="https://github.com/$REPO/releases/download/${LATEST_TAG}/SHA256SUMS.txt"
if curl -fsSL "$SUMS_URL" -o "$TMP_SUMS"; then
  EXPECTED="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+(.*/)?dck-linux-${ARCH}\$" "$TMP_SUMS" | awk '{print $1}')"
  if [[ -z "$EXPECTED" ]]; then
    warn "SHA256SUMS.txt does not contain dck-linux-${ARCH}; cannot verify"
  else
    ACTUAL="$(sha256sum "$TMP_BIN" | awk '{print $1}')"
    if [[ "$ACTUAL" != "$EXPECTED" ]]; then
      rm -f "$TMP_BIN" "$TMP_SUMS"
      err "SHA256 mismatch: expected ${EXPECTED}, got ${ACTUAL}"
    fi
    log "SHA256 verified: ${ACTUAL}"
  fi
else
  warn "SHA256SUMS.txt not available for this release; skipping verification"
fi

install -m 0755 "$TMP_BIN" "$DCK_BIN"
rm -f "$TMP_BIN" "$TMP_SUMS"
log "Binary installed: $DCK_BIN"

# ---- Verify binary works (check for glibc error) ----
if ! "$DCK_BIN" --version &>/dev/null; then
  warn "Binary failed to run (likely glibc mismatch). Building from source..."
  if command -v go &>/dev/null; then
    log "Building dck from source..."
    TMPDIR=$(mktemp -d)
    git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR" 2>/dev/null || {
      err "Git clone failed. Install Go manually and run: CGO_ENABLED=0 go build"
    }
    cd "$TMPDIR"
    CGO_ENABLED=0 go build -tags netgo -installsuffix netgo -ldflags="-s -w" -o dck .
    cp dck "$DCK_BIN"
    chmod +x "$DCK_BIN"
    cd /
    rm -rf "$TMPDIR"
    log "Built from source: $DCK_BIN"
  else
    warn "Go not installed. Installing Go to build from source..."
    GO_VER="1.23.4"
    curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
    tar -C /usr/local -xzf /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    TMPDIR=$(mktemp -d)
    git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR"
    cd "$TMPDIR"
    CGO_ENABLED=0 /usr/local/go/bin/go build -tags netgo -installsuffix netgo -ldflags="-s -w" -o dck .
    cp dck "$DCK_BIN"
    chmod +x "$DCK_BIN"
    cd /
    rm -rf "$TMPDIR" /tmp/go.tar.gz
    log "Built from source: $DCK_BIN"
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
  "$DCK_BIN" bootstrap --install 2>/dev/null || true
  if [[ -n "$SERVICE_RELOAD" ]]; then
    $SERVICE_RELOAD 2>/dev/null || true
  fi
fi

# ---- Verify ----
log "Verifying installation..."
if command -v dck &>/dev/null; then
  log "dck installed: $(dck --version 2>/dev/null || echo 'ok')"
else
  warn "dck not found in PATH — ensure $DCK_BIN is accessible"
fi

# ---- Done ----
echo ""
log "═══════════════════════════════════════════════"
log "  dck installed successfully!"
log "═══════════════════════════════════════════════"
log ""
log "  Quick start:"
log "    dck pull alpine"
log "    dck run --rm alpine echo hello"
log "    dck --help"
log ""
log "  Docs:  https://github.com/$REPO"
echo ""
