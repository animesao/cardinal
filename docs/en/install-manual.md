<!-- dck-version:start -->
**Documentation version:** `1.60.0`
**Project release:** `v1.60.0`
<!-- dck-version:end -->

# Installing dck — Manual Binary

Download the raw binary and install it manually.

## Download

```bash
# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  armv7l)  SUFFIX="armv6" ;;
  *)       echo "Unsupported: $ARCH"; exit 1 ;;
esac

# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.tar.gz" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Option 1: Extract and Install

```bash
tar xzf "dck-${VERSION}-linux-${SUFFIX}.tar.gz"
sudo mv "dck-${VERSION}/dck" /usr/local/bin/dck
chmod +x /usr/local/bin/dck
rm -rf "dck-${VERSION}" "dck-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Option 2: Direct Binary Download

```bash
curl -fL -o /usr/local/bin/dck \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-linux-${SUFFIX}"
chmod +x /usr/local/bin/dck
```

## Option 3: From Source

```bash
# Requires Go 1.26+
git clone https://github.com/animesao/dck.git
cd dck
go build -tags netgo -ldflags="-s -w" -o /usr/local/bin/dck .
```

## Enable Supervisor (Optional)

For auto-start on boot and scheduled backups:

```bash
dck bootstrap --install
```

## Verify

```bash
dck version
dck doctor
```

## Uninstall

```bash
dck bootstrap --remove 2>/dev/null || true
sudo rm /usr/local/bin/dck
sudo rm -rf ~/.dck
```
