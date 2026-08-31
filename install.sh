#!/usr/bin/env bash
# ============================================================
# cardinal — Universal Node Installer
#
# Installs cardinal (container runtime) + cardinal-wings (node
# manager) + bootstrap (supervisor) in one shot. Works on any
# Linux distribution with systemd.
#
# Hosted by the cardinal website. Usage:
#   curl -fsSL https://cardinal.spcfy.eu/downloads/install-universal.sh | sudo bash
#
# Environment:
#   CARDINAL_MIRROR=...   mirror base (default https://cardinal.spcfy.eu/downloads/cardinal)
#   WINGS_MIRROR=...      mirror base (default https://cardinal.spcfy.eu/downloads/wings)
#   WINGS_HOST=...        bind address for wings (default 0.0.0.0)
#   WINGS_PORT=...        bind port for wings (default 8080)
# ============================================================
set -euo pipefail

# ---- colours / logging ----
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[i]${NC} $1"; }

if [[ $EUID -ne 0 ]]; then err "Must run as root: sudo bash install-universal.sh"; fi

# ---- env / defaults ----
CARDINAL_MIRROR="${CARDINAL_MIRROR:-https://cardinal.spcfy.eu/downloads/cardinal}"
WINGS_MIRROR="${WINGS_MIRROR:-https://cardinal.spcfy.eu/downloads/wings}"
WINGS_HOST="${WINGS_HOST:-0.0.0.0}"
WINGS_PORT="${WINGS_PORT:-8080}"
WINGS_SFTP_PORT="${WINGS_SFTP_PORT:-2022}"
CARDINAL_BIN="/usr/local/bin/cardinal"
WINGS_BIN="/usr/local/bin/cardinal-wings"
CONF_DIR="/etc/cardinal-wings"

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

# Detect version: mirror first, then GitHub
if [[ -n "$CARDINAL_MIRROR" ]]; then
  MV="$(curl -sfL --retry 2 --connect-timeout 12 --max-time 25 "$CARDINAL_MIRROR/VERSION" 2>/dev/null | tail -1 | tr -d '[:space:]')"
  if [[ -n "$MV" ]] && [[ "$MV" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    LATEST_TAG="${MV}"
    [[ "$LATEST_TAG" != v* ]] && LATEST_TAG="v$LATEST_TAG"
    log "Latest cardinal version (mirror): $LATEST_TAG"
  fi
fi
if [[ -z "$LATEST_TAG" ]]; then
  LATEST_TAG=$(curl -sfL --connect-timeout 12 --max-time 25 "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | cut -d'"' -f4)
fi
if [[ -z "$LATEST_TAG" ]]; then
  err "Could not detect latest cardinal release"
fi

TMP_ARCHIVE="$(mktemp -t cardinal-archive.XXXXXX)"
TMP_SUMS="$(mktemp -t cardinal-sums.XXXXXX)"
VERSION="${LATEST_TAG#v}"
ARCHIVE_NAME="cardinal_${VERSION}_linux_${ARCH}.tar.gz"

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

# SHA256 verification
SUMS_SRC=""
if [[ "$dl_ok" = "1" ]] && download "$CARDINAL_MIRROR/$LATEST_TAG/SHA256SUMS" "$TMP_SUMS"; then
  SUMS_SRC="mirror"
elif download "https://github.com/$REPO/releases/download/${LATEST_TAG}/cardinal_${VERSION}_checksums.txt" "$TMP_SUMS"; then
  SUMS_SRC="github"
fi
if [[ -n "$SUMS_SRC" ]]; then
  EXPECTED="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+${ARCHIVE_NAME}\$" "$TMP_SUMS" | awk '{print $1}')"
  if [[ -n "$EXPECTED" ]]; then
    ACTUAL="$(sha256sum "$TMP_ARCHIVE" | awk '{print $1}')"
    if [[ "$ACTUAL" != "$EXPECTED" ]]; then
      rm -f "$TMP_ARCHIVE" "$TMP_SUMS"
      err "SHA256 mismatch: expected ${EXPECTED}, got ${ACTUAL}"
    fi
    log "SHA256 verified (${SUMS_SRC}): ${ACTUAL:0:16}…"
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
# PART 2: cardinal-wings binary
# ============================================================
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log "  Installing cardinal-wings (node manager)"
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

WINGS_REPO="animesao/cardinal-wings"
WINGS_LATEST=""

# Detect version
if [[ -n "$WINGS_MIRROR" ]]; then
  WV="$(curl -sfL --retry 2 --connect-timeout 12 --max-time 25 "$WINGS_MIRROR/VERSION" 2>/dev/null | tail -1 | tr -d '[:space:]')"
  if [[ -n "$WV" ]] && [[ "$WV" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    WINGS_LATEST="${WV}"
    [[ "$WINGS_LATEST" != v* ]] && WINGS_LATEST="v$WINGS_LATEST"
    log "Latest wings version (mirror): $WINGS_LATEST"
  fi
fi
if [[ -z "$WINGS_LATEST" ]]; then
  WINGS_LATEST=$(curl -sfL --connect-timeout 12 --max-time 25 "https://api.github.com/repos/$WINGS_REPO/releases/latest" \
    | grep '"tag_name"' | cut -d'"' -f4)
fi
if [[ -z "$WINGS_LATEST" ]]; then
  err "Could not detect latest wings release"
fi

ASSET="cardinal-wings-linux-${ARCH}"
mkdir -p "$(dirname "$WINGS_BIN")"

log "Downloading wings ${WINGS_LATEST} (${ARCH})..."
dl_ok=0
if [[ -n "$WINGS_MIRROR" ]]; then
  if download "$WINGS_MIRROR/$WINGS_LATEST/$ASSET" "$WINGS_BIN"; then
    dl_ok=1
  else
    log "Mirror download failed — falling back to GitHub"
  fi
fi
if [[ "$dl_ok" = "0" ]]; then
  download "https://github.com/$WINGS_REPO/releases/download/${WINGS_LATEST}/${ASSET}" "$WINGS_BIN" \
    || err "Download failed: ${ASSET}"
fi
chmod +x "$WINGS_BIN"

# SHA256 verification
if [[ "$dl_ok" = "1" ]]; then
  TMP_WS="$(mktemp -t wings-sums.XXXXXX)"
  if download "$WINGS_MIRROR/$WINGS_LATEST/SHA256SUMS" "$TMP_WS"; then
    W_EXPECTED="$(grep "  ${ASSET}$" "$TMP_WS" | awk '{print $1}')"
    if [[ -n "$W_EXPECTED" ]]; then
      W_ACTUAL="$(sha256sum "$WINGS_BIN" | awk '{print $1}')"
      if [[ "$W_ACTUAL" != "$W_EXPECTED" ]]; then
        rm -f "$TMP_WS"
        err "SHA256 mismatch for wings: expected ${W_EXPECTED}, got ${W_ACTUAL}"
      fi
      log "Wings SHA256 verified: ${W_ACTUAL:0:16}…"
    fi
  fi
  rm -f "$TMP_WS"
fi

log "cardinal-wings installed: $($WINGS_BIN --version 2>/dev/null || echo 'ok')"

# ============================================================
# PART 3: wings config (generate once)
# ============================================================
mkdir -p "$CONF_DIR"
chmod 700 "$CONF_DIR"

if [ -f "$CONF_DIR/config.toml" ]; then
  chmod 600 "$CONF_DIR/config.toml"
  log "Wings config exists — keeping it ($CONF_DIR/config.toml)"
else
  API_KEY="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  umask 077
  {
    echo "# cardinal-wings configuration"
    echo "# generated by install-universal.sh"
    echo ""
    echo "[server]"
    echo "host = \"${WINGS_HOST}\""
    echo "port = ${WINGS_PORT}"
    echo "sftp_enabled = true"
    echo "sftp_host = \"0.0.0.0\""
    echo "sftp_port = ${WINGS_SFTP_PORT}"
    echo ""
    echo "[rate_limit]"
    echo "ip_tps = 25"
    echo "ip_burst = 50"
    echo "key_tps = 10"
    echo "key_burst = 30"
    echo "max_clients = 4096"
    echo ""
    echo "[[keys]]"
    echo "name = \"panel\""
    echo "key = \"${API_KEY}\""
    echo "role = \"admin\""
  } > "$CONF_DIR/config.toml"
  chmod 600 "$CONF_DIR/config.toml"
  log "Wings config written: $CONF_DIR/config.toml"
fi

# ============================================================
# PART 4: systemd units
# ============================================================
log "Installing systemd services..."

# cardinal-wings service
cat > /tmp/cardinal-wings.service <<EOF
[Unit]
Description=cardinal-wings REST API daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${WINGS_BIN} --config ${CONF_DIR}/config.toml
Restart=on-failure
RestartSec=5s
Environment=CARDINAL_DATA_DIR=/root/.cardinal
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
install -m 644 /tmp/cardinal-wings.service /etc/systemd/system/cardinal-wings.service

# IP forwarding
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
# PART 5: firewall
# ============================================================
open_ports=("${WINGS_PORT}/tcp" "${WINGS_SFTP_PORT}/tcp")

ssh_port() {
  local p
  p="$(grep -Ei '^\s*Port\s+[0-9]+' /etc/ssh/sshd_config 2>/dev/null | tail -1 | awk '{print $2}')"
  echo "${p:-22}"
}

if command -v ufw &>/dev/null; then
  sshp="$(ssh_port)"
  echo "==> opening firewall ports: SSH :${sshp} + ${open_ports[*]}"
  ufw allow "${sshp}/tcp" 2>/dev/null || true
  ufw allow "${WINGS_PORT}/tcp" 2>/dev/null || true
  ufw allow "${WINGS_SFTP_PORT}/tcp" 2>/dev/null || true
  ufw --force enable 2>/dev/null || true
elif command -v firewall-cmd &>/dev/null; then
  systemctl enable --now firewalld >/dev/null 2>&1 || true
  firewall-cmd --permanent --add-service=ssh 2>/dev/null || true
  firewall-cmd --permanent --add-port="${WINGS_PORT}/tcp" 2>/dev/null || true
  firewall-cmd --permanent --add-port="${WINGS_SFTP_PORT}/tcp" 2>/dev/null || true
  firewall-cmd --reload 2>/dev/null || true
elif command -v iptables &>/dev/null; then
  iptables -A INPUT -p tcp --dport "${WINGS_PORT}" -j ACCEPT 2>/dev/null || true
  iptables -A INPUT -p tcp --dport "${WINGS_SFTP_PORT}" -j ACCEPT 2>/dev/null || true
fi

# ============================================================
# PART 6: start services
# ============================================================
systemctl enable cardinal-wings 2>/dev/null || true
systemctl start cardinal-wings 2>/dev/null || true

# ============================================================
# DONE — panel binding summary
# ============================================================
SCHEME="http"
external_ip() {
  local ip url
  for url in "https://api.ipify.org" "https://ifconfig.me/ip" "https://icanhazip.com"; do
    ip="$(curl -fsSL --connect-timeout 5 --max-time 10 "$url" 2>/dev/null | tr -d '[:space:]')"
    if [ -n "$ip" ] && printf '%s' "$ip" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
      echo "$ip"; return 0
    fi
  done
  hostname -I 2>/dev/null | tr ' ' '\n' | grep -vE '^(127\.|::)' | head -1
}
EXT_IP="$(external_ip || true)"

# Read API key from config
API_KEY=""
if [ -f "$CONF_DIR/config.toml" ]; then
  API_KEY="$(sed -n 's/^key *= *"\([^"]*\)".*/\1/p' "$CONF_DIR/config.toml" | head -1)"
fi

echo ""
log "════════════════════════════════════════════════════════════"
log "  cardinal node installed successfully!"
log "════════════════════════════════════════════════════════════"
log ""
log "  cardinal:       $($CARDINAL_BIN --version 2>/dev/null || echo 'installed')"
log "  cardinal-wings: $($WINGS_BIN --version 2>/dev/null || echo 'installed')"
log ""
log "  ============================================================"
log "   PANEL BINDING  —  enter these in the panel's \"Add node\" form"
log "   ------------------------------------------------------------"
log "   URL   : ${SCHEME}://${EXT_IP:-127.0.0.1}:${WINGS_PORT}"
log "   Token : ${API_KEY:-<read from $CONF_DIR/config.toml>}"
log "   ------------------------------------------------------------"
log "   Panel → Admin → Nodes → Add node → URL + Token."
log "  ============================================================"
log ""
log "  Open ports:"
log "    ${WINGS_PORT}/tcp   — wings API"
log "    ${WINGS_SFTP_PORT}/tcp — per-container SFTP"
log ""
log "  Quick check:"
log "    curl -s localhost:${WINGS_PORT}/v1/ping"
log "    curl -s -H \"Authorization: Bearer ${API_KEY}\" localhost:${WINGS_PORT}/v1/system/info"
log ""
