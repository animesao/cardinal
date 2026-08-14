<!-- dck-version:start -->
**Documentation version:** `1.24.11`
**Project release:** `v1.24.11`
<!-- dck-version:end -->

# Installing dck on Debian / Ubuntu

## Option 1: APT Repository (Recommended)

Add the official dck APT repository and install:

```bash
# Add repository
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/scripts/install-apt.sh | sudo bash

# Install
sudo apt update
sudo apt install dck
```

### What the Script Does

1. Adds the GPG key to `/usr/share/keyrings/dck-archive-keyring.gpg`
2. Adds the repository to `/etc/apt/sources.list.d/dck.list`
3. Runs `apt update && apt install dck`

### Architecture Support

| Architecture | Package |
|-------------|---------|
| amd64 | `dck` (default) |
| arm64 | `dck` (auto-detected) |
| armhf (armv6) | `dck` (auto-detected) |

## Option 2: Manual .deb Package

Download and install the `.deb` package directly from GitHub Releases:

```bash
# Detect architecture
ARCH=$(dpkg --print-architecture)
case "$ARCH" in
  amd64)  SUFFIX="amd64" ;;
  arm64)  SUFFIX="arm64" ;;
  armhf)  SUFFIX="armv6" ;;
  *)      echo "Unsupported: $ARCH"; exit 1 ;;
esac

# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.deb" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.deb"
sudo dpkg -i "dck-${VERSION}-linux-${SUFFIX}.deb"
rm "dck-${VERSION}-linux-${SUFFIX}.deb"
```

## Option 3: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

## Verify

```bash
dck version
dck doctor
```

## Uninstall

```bash
# If installed via APT
sudo apt remove dck
sudo rm /etc/apt/sources.list.d/dck.list
sudo rm /usr/share/keyrings/dck-archive-keyring.gpg

# Remove data
dck bootstrap --remove
sudo rm -rf ~/.dck
```
