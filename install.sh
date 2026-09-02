#!/usr/bin/env bash
# ============================================================
# cardinal — Universal Installer
#
# Installs cardinal (container runtime) + bootstrap (supervisor)
# in one shot. Works on any Linux distribution with systemd.
#
# cardinal-wings (node manager) is a separate project with its own
# installer — it is NOT installed by this script.
#
# Hosted on GitHub. Usage:
#   curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
#
# Everything (version detection, binaries, checksums) is downloaded
# exclusively from the official GitHub repository.
# ============================================================
set -euo pipefail

# ---- colours / logging ----
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[i]${NC} $1"; }

if [[ $EUID -ne 0 ]]; then err "Must run as root: sudo bash install.sh"; fi

# ---- env / defaults ----
CARDINAL_BIN="/usr/local/bin/cardinal"

# ---- robust download: retries, hard timeouts, IPv4 fallback ----
download() {
  local url="$1" dest="$2"
  local tmp="${dest}.tmp.$$"
  local try
  for try in 1 2 3; do
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

# ---- OS / arch detection ----
if [[ ! -f /etc/os-release ]]; then err "Cannot detect OS — /etc/os-release not found"; fi
source /etc/os-release
log "OS: ${PRETTY_NAME:-$ID $VERSION_ID}"

ARCH="amd64"
case "$(uname -m)" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l)  ARCH="armv6" ;;
  *)       ARCH="amd64"; warn "Unknown arch $(uname -m), defaulting to amd64" ;;
esac
info "Architecture: $ARCH"

# ---- package manager detection ----
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
  *)
    warn "Unknown distro: ${ID:-unknown}. Will try to install binary only."
    PKG_MGR="manual"
    ;;
esac

if [[ "$PKG_MGR" != "manual" ]]; then
  info "Package manager: $PKG_MGR"
fi

# ---- install dependencies ----
if [[ -n "${PKG_DEPS:-}" ]]; then
  log "Installing dependencies..."
  case "$PKG_MGR" in
    apt)    apt-get update -qq ;;
    pacman) pacman -Sy --noconfirm --needed 2>/dev/null || true ;;
  esac
  $PKG_INSTALL $PKG_DEPS 2>/dev/null || warn "Some dependencies may be missing"
fi

# ============================================================
# PART 1: cardinal binary
# ============================================================
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log "  Installing cardinal (container runtime)"
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

REPO="animesao/cardinal"
LATEST_TAG=""

# Detect latest version from GitHub
LATEST_TAG=$(curl -sfL --connect-timeout 12 --max-time 25 "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)
if [[ -z "$LATEST_TAG" ]]; then
  err "Could not detect latest cardinal release"
fi

TMP_ARCHIVE="$(mktemp -t cardinal-archive.XXXXXX)"
TMP_SUMS="$(mktemp -t cardinal-sums.XXXXXX)"
VERSION="${LATEST_TAG#v}"
ARCHIVE_NAME="cardinal_${VERSION}_linux_${ARCH}.tar.gz"

log "Downloading cardinal ${LATEST_TAG} (${ARCH})..."
download "https://github.com/$REPO/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}" "$TMP_ARCHIVE" \
  || err "Download failed: ${ARCHIVE_NAME}"

# SHA256 verification against the release checksums file on GitHub
if download "https://github.com/$REPO/releases/download/${LATEST_TAG}/cardinal_${VERSION}_checksums.txt" "$TMP_SUMS"; then
  EXPECTED="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+${ARCHIVE_NAME}$" "$TMP_SUMS" | awk '{print $1}')"
  if [[ -n "$EXPECTED" ]]; then
    ACTUAL="$(sha256sum "$TMP_ARCHIVE" | awk '{print $1}')"
    if [[ "$ACTUAL" != "$EXPECTED" ]]; then
      rm -f "$TMP_ARCHIVE" "$TMP_SUMS"
      err "SHA256 mismatch: expected ${EXPECTED}, got ${ACTUAL}"
    fi
    log "SHA256 verified: ${ACTUAL:0:16}…"
  fi
fi

# Extract
EXTRACT_DIR="$(dirname "$TMP_ARCHIVE")"
tar -xzf "$TMP_ARCHIVE" -C "$EXTRACT_DIR" --strip-components=0 cardinal 2>/dev/null \
  || { tar -xzf "$TMP_ARCHIVE" -C /tmp cardinal 2>/dev/null && cp /tmp/cardinal "$EXTRACT_DIR/cardinal"; }
install -m 0755 "$EXTRACT_DIR/cardinal" "$CARDINAL_BIN"
rm -f "$TMP_ARCHIVE" "$TMP_SUMS" /tmp/cardinal "$EXTRACT_DIR/cardinal"

# Verify
if ! "$CARDINAL_BIN" --version &>/dev/null; then
  warn "Binary failed to run (likely glibc mismatch). Building from source..."
  if command -v go &>/dev/null; then
    log "Building cardinal from source..."
    TMPDIR=$(mktemp -d)
    timeout 240 git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR" 2>/dev/null \
      || err "Git clone failed"
    cd "$TMPDIR"
    CGO_ENABLED=0 go build -tags netgo -installsuffix netgo -ldflags="-s -w" -o cardinal .
    install -m 0755 cardinal "$CARDINAL_BIN"
    cd /; rm -rf "$TMPDIR"
  else
    err "Binary not runnable and Go not installed — install Go and re-run"
  fi
fi

log "cardinal installed: $($CARDINAL_BIN --version 2>/dev/null || echo 'ok')"

# ============================================================
# PART 2: systemd setup
# ============================================================
log "Installing systemd services..."

# IP forwarding (bridge networking)
if [[ -f /proc/sys/net/ipv4/ip_forward ]]; then
  echo 1 > /proc/sys/net/ipv4/ip_forward
  grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf 2>/dev/null || \
    echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
fi

# Bootstrap (cardinal supervisor)
if [[ -d /run/systemd/system ]]; then
  "$CARDINAL_BIN" bootstrap --install 2>/dev/null || true
fi

if [[ -n "${SERVICE_RELOAD:-}" ]]; then
  $SERVICE_RELOAD 2>/dev/null || true
fi

# ============================================================
# PART 3: firewall (keep SSH reachable)
# ============================================================
ssh_port() {
  local p
  p="$(grep -Ei '^\s*Port\s+[0-9]+' /etc/ssh/sshd_config 2>/dev/null | tail -1 | awk '{print $2}')"
  echo "${p:-22}"
}

if command -v ufw &>/dev/null; then
  sshp="$(ssh_port)"
  echo "==> opening firewall port: SSH :${sshp}"
  ufw allow "${sshp}/tcp" 2>/dev/null || true
  ufw --force enable 2>/dev/null || true
elif command -v firewall-cmd &>/dev/null; then
  systemctl enable --now firewalld >/dev/null 2>&1 || true
  firewall-cmd --permanent --add-service=ssh 2>/dev/null || true
  firewall-cmd --reload 2>/dev/null || true
elif command -v iptables &>/dev/null; then
  sshp="$(ssh_port)"
  iptables -A INPUT -p tcp --dport "${sshp}" -j ACCEPT 2>/dev/null || true
fi

# ============================================================
# DONE
# ============================================================
echo ""
log "════════════════════════════════════════════════════════════"
log "  cardinal installed successfully!"
log "════════════════════════════════════════════════════════════"
log ""
log "  cardinal: $($CARDINAL_BIN --version 2>/dev/null || echo 'installed')"
log ""
log "  Quick start:"
log "    cardinal pull alpine"
log "    cardinal run --rm alpine echo hello"
log "    cardinal --help"
log ""