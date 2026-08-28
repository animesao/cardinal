<!-- cardinal-version:start -->
**Documentation version:** `2.0.11`
**Project release:** `v2.0.11`
<!-- cardinal-version:end -->

# Installing cardinal on Debian / Ubuntu

## Option 1: APT Repository (Recommended)

Add the official cardinal APT repository and install:

```bash
# Add repository
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/scripts/install-apt.sh | sudo bash

# Install
sudo apt update
sudo apt install cardinal
```

### What the Script Does

1. Adds the GPG key to `/usr/share/keyrings/cardinal-archive-keyring.gpg`
2. Adds the repository to `/etc/apt/sources.list.d/cardinal.list`
3. Runs `apt update && apt install cardinal`

### Architecture Support

| Architecture | Package |
|-------------|---------|
| amd64 | `cardinal` (default) |
| arm64 | `cardinal` (auto-detected) |
| armhf (armv6) | `cardinal` (auto-detected) |

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.deb" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.deb"
sudo dpkg -i "cardinal-${VERSION}-linux-${SUFFIX}.deb"
rm "cardinal-${VERSION}-linux-${SUFFIX}.deb"
```

## Option 3: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

## Verify

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
# If installed via APT
sudo apt remove cardinal
sudo rm /etc/apt/sources.list.d/cardinal.list
sudo rm /usr/share/keyrings/cardinal-archive-keyring.gpg

# Remove data
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
