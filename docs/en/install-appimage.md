<!-- dck-version:start -->
**Documentation version:** `1.60.5`
**Project release:** `v1.60.5`
<!-- dck-version:end -->

# Installing dck via AppImage

AppImage is a portable format — no package manager required.

## Quick Install (Desktop)

Double-click the `.AppImage` file. A terminal-based installer will:

1. Copy the binary to `/usr/local/bin/dck`
2. Enable the systemd supervisor

## CLI Install

```bash
# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download
curl -fL -o "dck-${VERSION}-linux-amd64.AppImage" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-amd64.AppImage"
chmod +x "dck-${VERSION}-linux-amd64.AppImage"

# Install
"./dck-${VERSION}-linux-amd64.AppImage" --install

# Or use directly without installing
"./dck-${VERSION}-linux-amd64.AppImage" run --rm alpine echo hello
```

## Architecture Support

| Arch | AppImage |
|------|----------|
| x86_64 | ✅ `dck-*-linux-amd64.AppImage` |
| aarch64 | ✅ `dck-*-linux-arm64.AppImage` |
| armv6 | ❌ Use raw binary instead |

## Portable Usage

AppImage works without installation:

```bash
./dck-*-linux-amd64.AppImage version
./dck-*-linux-amd64.AppImage run --rm alpine echo hello
```

## Verify

```bash
dck version
dck doctor
```

## Uninstall

```bash
# If installed via --install
dck bootstrap --remove
sudo rm /usr/local/bin/dck

# Remove AppImage file
rm dck-*-linux-amd64.AppImage

# Remove data
sudo rm -rf ~/.dck
```
