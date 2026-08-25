<!-- cardinal-version:start -->
**Documentation version:** `2.0.2`
**Project release:** `v2.0.2`
<!-- cardinal-version:end -->

# Installing cardinal — Manual Binary

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Option 1: Extract and Install

```bash
tar xzf "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
sudo mv "cardinal-${VERSION}/cardinal" /usr/local/bin/cardinal
chmod +x /usr/local/bin/cardinal
rm -rf "cardinal-${VERSION}" "cardinal-${VERSION}-linux-${SUFFIX}.tar.gz"
```

## Option 2: Direct Binary Download

```bash
curl -fL -o /usr/local/bin/cardinal \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-linux-${SUFFIX}"
chmod +x /usr/local/bin/cardinal
```

## Option 3: From Source

```bash
# Requires Go 1.26+
git clone https://github.com/animesao/cardinal.git
cd cardinal
go build -tags netgo -ldflags="-s -w" -o /usr/local/bin/cardinal .
```

## Enable Supervisor (Optional)

For auto-start on boot and scheduled backups:

```bash
cardinal bootstrap --install
```

## Verify

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
cardinal bootstrap --remove 2>/dev/null || true
sudo rm /usr/local/bin/cardinal
sudo rm -rf ~/.cardinal
```
