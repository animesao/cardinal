<!-- dck-version:start -->
**Documentation version:** `1.24.12`
**Project release:** `v1.24.12`
<!-- dck-version:end -->

# Installing dck on Alpine Linux

## Option 1: APK Package (Recommended)

Download and install the `.apk` package from GitHub Releases:

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

# Download and install
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.apk" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.apk"
sudo apk add --allow-untrusted "dck-${VERSION}-linux-${SUFFIX}.apk"
rm "dck-${VERSION}-linux-${SUFFIX}.apk"
```

## Option 2: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

## Option 3: Binary Archive

```bash
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "dck-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-amd64.tar.gz"
tar xzf "dck-${VERSION}-linux-amd64.tar.gz"
sudo mv "dck-${VERSION}/dck" /usr/local/bin/dck
rm -rf "dck-${VERSION}" "dck-${VERSION}-linux-amd64.tar.gz"
```

## Required Packages

Alpine needs extra packages for networking:

```bash
sudo apk add iptables ip6tables iproute2
```

## Kernel Modules

```bash
sudo modprobe overlay
sudo modprobe veth
sudo modprobe br_netfilter

# Make persistent
echo -e "overlay\nveth\nbr_netfilter" | sudo tee /etc/modules
```

## Verify

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
