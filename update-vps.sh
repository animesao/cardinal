#!/bin/bash
set -e

# Pinned release tag: never track a moving branch (a compromised or broken
# main would otherwise flow straight into root on every VPS running this).
CARDINAL_VERSION="${CARDINAL_VERSION:-v2.3.14}"

echo "=== Updating cardinal CLI ($CARDINAL_VERSION) ==="
cd /tmp
rm -rf cardinal-update
git clone --branch "$CARDINAL_VERSION" --depth 1 https://github.com/animesao/cardinal.git cardinal-update
cd cardinal-update
# Fail closed if the tag does not resolve to a commit (typo, deleted tag).
git rev-parse --verify "HEAD" >/dev/null
CGO_ENABLED=0 go build -ldflags="-s -w" -o cardinal .
pkill -f "cardinal " 2>/dev/null || true
cp cardinal /usr/local/bin/cardinal
rm -rf /tmp/cardinal-update
echo "cardinal updated: $(cardinal version)"

echo ""
echo "=== Updating cardinal-panel ==="
cd /opt/cardinal-panel
git fetch origin --tags
git checkout "$CARDINAL_VERSION"
cd /opt/cardinal-panel/server
CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/cardinal-server .

mkdir -p /root/.cardinal-panel /root/.cardinal

# Secrets: reuse the existing unit's values when present, otherwise generate.
# Never ship hardcoded defaults: every VPS with this script would share the
# same JWT secret and admin password.
UNIT=/etc/systemd/system/cardinal-panel.service
OLD_JWT="$(sed -n 's/^Environment=JWT_SECRET=//p' "$UNIT" 2>/dev/null || true | head -1)"
OLD_ADMIN="$(sed -n 's/^Environment=ADMIN_PASSWORD=//p' "$UNIT" 2>/dev/null || true | head -1)"
if [ -z "$OLD_JWT" ]; then OLD_JWT="$(openssl rand -hex 32)"; fi
if [ -z "$OLD_ADMIN" ]; then OLD_ADMIN="$(openssl rand -hex 16)"; fi

cat > /etc/systemd/system/cardinal-panel.service << SYSTEMD
[Unit]
Description=cardinal Panel
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cardinal-server --port 80 --sftp-port 2222 --data-dir /root/.cardinal-panel
Restart=always
RestartSec=5
Environment=HOME=/root
Environment=JWT_SECRET=$OLD_JWT
Environment=ADMIN_PASSWORD=$OLD_ADMIN

[Install]
WantedBy=multi-user.target
SYSTEMD

systemctl daemon-reload

if systemctl is-active --quiet cardinal-panel 2>/dev/null; then
  systemctl restart cardinal-panel
  echo "cardinal-panel restarted via systemd"
elif systemctl is-active --quiet cardinal-server 2>/dev/null; then
  systemctl stop cardinal-server 2>/dev/null || true
  systemctl enable --now cardinal-panel
  echo "migrated from cardinal-server to cardinal-panel service"
else
  systemctl enable --now cardinal-panel
  echo "cardinal-panel started via systemd"
fi

echo ""
echo "=== Done ==="
cardinal blueprint list | head -5
