<!-- cardinal-version:start -->
**Documentation version:** `1.60.11`
**Project release:** `v1.60.11`
<!-- cardinal-version:end -->

# Installing cardinal via Snap

## Install from GitHub Releases

Download the `.snap` package from GitHub Releases:

```bash
# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  *)       echo "Unsupported: $ARCH"; exit 1 ;;
esac

# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.snap" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.snap"
sudo snap install --dangerous --classic "cardinal-${VERSION}-linux-${SUFFIX}.snap"
rm "cardinal-${VERSION}-linux-${SUFFIX}.snap"
```

> **Note:** `--dangerous` is required because the snap is not from the Snap Store. `--classic` gives cardinal full system access (needed for namespace operations).

## Build from Source

```bash
git clone https://github.com/animesao/cardinal.git
cd cardinal
snapcraft
sudo snap install --dangerous --classic ./cardinal_*.snap
```

## What You Get

- Binary at `/snap/bin/cardinal`
- Classic confinement (full system access)
- Automatic alias: `cardinal` command available globally

## Verify

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
sudo snap remove cardinal
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
