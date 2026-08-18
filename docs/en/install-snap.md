<!-- dck-version:start -->
**Documentation version:** `1.60.11`
**Project release:** `v1.60.11`
<!-- dck-version:end -->

# Installing dck via Snap

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.snap" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.snap"
sudo snap install --dangerous --classic "dck-${VERSION}-linux-${SUFFIX}.snap"
rm "dck-${VERSION}-linux-${SUFFIX}.snap"
```

> **Note:** `--dangerous` is required because the snap is not from the Snap Store. `--classic` gives dck full system access (needed for namespace operations).

## Build from Source

```bash
git clone https://github.com/animesao/dck.git
cd dck
snapcraft
sudo snap install --dangerous --classic ./dck_*.snap
```

## What You Get

- Binary at `/snap/bin/dck`
- Classic confinement (full system access)
- Automatic alias: `dck` command available globally

## Verify

```bash
dck version
dck doctor
```

## Uninstall

```bash
sudo snap remove dck
dck bootstrap --remove
sudo rm -rf ~/.dck
```
