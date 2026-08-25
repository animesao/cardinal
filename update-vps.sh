#!/bin/bash
set -e

echo "=== Updating cardinal CLI ==="
cd /tmp
rm -rf cardinal-update
git clone https://github.com/animesao/cardinal.git cardinal-update
cd cardinal-update
CGO_ENABLED=0 go build -ldflags="-s -w" -o cardinal .
pkill -f "cardinal " 2>/dev/null || true
cp cardinal /usr/local/bin/cardinal
rm -rf /tmp/cardinal-update
echo "cardinal updated: $(cardinal version)"

echo ""
echo "=== Updating cardinal-panel ==="
cd /opt/cardinal-panel
git fetch origin
git reset --hard origin/main
cd /opt/cardinal-panel/server
CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/cardinal-server .

mkdir -p /root/.cardinal-panel /root/.cardinal

cat > /etc/systemd/system/cardinal-panel.service << 'SYSTEMD'
[Unit]
Description=cardinal Panel
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cardinal-server --port 80 --sftp-port 2222 --data-dir /root/.cardinal-panel
Restart=always
RestartSec=5
Environment=HOME=/root
Environment=JWT_SECRET=my_fixed_secret_key_32_char_long
Environment=ADMIN_PASSWORD=admin123

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
