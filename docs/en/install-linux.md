<!-- cardinal-version:start -->
**Documentation version:** `2.0.11`
**Project release:** `v2.0.11`
<!-- cardinal-version:end -->

# Installing cardinal on Linux (Universal)

The universal installer works on any Linux distribution with systemd.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

This installs the latest stable binary to `/usr/local/bin/cardinal` and enables the systemd supervisor.

## What It Does

1. Detects your architecture (amd64, arm64, armv6)
2. Downloads the latest release binary from GitHub
3. Verifies the SHA256 checksum
4. Installs to `/usr/local/bin/cardinal`
5. Enables `cardinal-bootstrap.service` (auto-start on boot)

## Requirements

- Linux kernel with namespaces support (PID, Mount, Net, UTS, IPC)
- `unshare`, `nsenter`, `ip`, `iptables`, `mount`, `pgrep`
- overlayfs kernel module
- systemd (for supervisor and auto-start)

## Verify Installation

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
cardinal bootstrap --remove
sudo rm /usr/local/bin/cardinal
sudo rm -rf ~/.cardinal
```
