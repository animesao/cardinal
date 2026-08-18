<!-- dck-version:start -->
**Documentation version:** `1.60.7`
**Project release:** `v1.60.7`
<!-- dck-version:end -->

# Installing dck on Linux (Universal)

The universal installer works on any Linux distribution with systemd.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

This installs the latest stable binary to `/usr/local/bin/dck` and enables the systemd supervisor.

## What It Does

1. Detects your architecture (amd64, arm64, armv6)
2. Downloads the latest release binary from GitHub
3. Verifies the SHA256 checksum
4. Installs to `/usr/local/bin/dck`
5. Enables `dck-bootstrap.service` (auto-start on boot)

## Requirements

- Linux kernel with namespaces support (PID, Mount, Net, UTS, IPC)
- `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep`
- overlayfs kernel module
- systemd (for supervisor and auto-start)

## Verify Installation

```bash
dck version
dck doctor
```

## Uninstall

```bash
dck bootstrap --remove
sudo rm /usr/local/bin/dck
sudo rm -rf ~/.dck
```
