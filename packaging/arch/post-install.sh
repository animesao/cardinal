#!/bin/sh
# Post-install script for Arch Linux package
set -e

# Reload systemd if available
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload 2>/dev/null || true
  echo "dck installed. Run 'dck bootstrap --install' to enable boot recovery."
fi
