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
echo "=== Updating dck-panel ==="
cd /opt/dck-panel
git fetch origin
git reset --hard origin/main
cd /opt/dck-panel/server
CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/cardinal-server .

mkdir -p /root/.dck-panel /root/.cardinal

cat > /etc/systemd/system/dck-panel.service << 'SYSTEMD'
[Unit]
Description=cardinal Panel
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cardinal-server --port 80 --sftp-port 2222 --data-dir /root/.dck-panel
Restart=always
RestartSec=5
Environment=HOME=/root
Environment=JWT_SECRET=my_fixed_secret_key_32_char_long
Environment=ADMIN_PASSWORD=admin123

[Install]
WantedBy=multi-user.target
SYSTEMD

systemctl daemon-reload

if systemctl is-active --quiet dck-panel 2>/dev/null; then
  systemctl restart dck-panel
  echo "dck-panel restarted via systemd"
elif systemctl is-active --quiet cardinal-server 2>/dev/null; then
  systemctl stop cardinal-server 2>/dev/null || true
  systemctl enable --now dck-panel
  echo "migrated from cardinal-server to dck-panel service"
else
  systemctl enable --now dck-panel
  echo "dck-panel started via systemd"
fi

echo ""
echo "=== Done ==="
cardinal blueprint list | head -5
