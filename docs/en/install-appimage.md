<!-- cardinal-version:start -->
**Documentation version:** `2.0.12`
**Project release:** `v2.0.12`
<!-- cardinal-version:end -->

# Installing cardinal via AppImage

AppImage is a portable format — no package manager required.

## Quick Install (Desktop)

Double-click the `.AppImage` file. A terminal-based installer will:

1. Copy the binary to `/usr/local/bin/cardinal`
2. Enable the systemd supervisor

## CLI Install

```bash
# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download
curl -fL -o "cardinal-${VERSION}-linux-amd64.AppImage" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-amd64.AppImage"
chmod +x "cardinal-${VERSION}-linux-amd64.AppImage"

# Install
"./cardinal-${VERSION}-linux-amd64.AppImage" --install

# Or use directly without installing
"./cardinal-${VERSION}-linux-amd64.AppImage" run --rm alpine echo hello
```

## Architecture Support

| Arch | AppImage |
|------|----------|
| x86_64 | ✅ `cardinal-*-linux-amd64.AppImage` |
| aarch64 | ✅ `cardinal-*-linux-arm64.AppImage` |
| armv6 | ❌ Use raw binary instead |

## Portable Usage

AppImage works without installation:

```bash
./cardinal-*-linux-amd64.AppImage version
./cardinal-*-linux-amd64.AppImage run --rm alpine echo hello
```

## Verify

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
# If installed via --install
cardinal bootstrap --remove
sudo rm /usr/local/bin/cardinal

# Remove AppImage file
rm cardinal-*-linux-amd64.AppImage

# Remove data
sudo rm -rf ~/.cardinal
```
