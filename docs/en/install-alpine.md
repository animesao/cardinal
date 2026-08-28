<!-- cardinal-version:start -->
**Documentation version:** `2.0.10`
**Project release:** `v2.0.10`
<!-- cardinal-version:end -->

# Installing cardinal on Alpine Linux

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.apk" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.apk"
sudo apk add --allow-untrusted "cardinal-${VERSION}-linux-${SUFFIX}.apk"
rm "cardinal-${VERSION}-linux-${SUFFIX}.apk"
```

## Option 2: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

## Option 3: Binary Archive

```bash
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "cardinal-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-amd64.tar.gz"
tar xzf "cardinal-${VERSION}-linux-amd64.tar.gz"
sudo mv "cardinal-${VERSION}/cardinal" /usr/local/bin/cardinal
rm -rf "cardinal-${VERSION}" "cardinal-${VERSION}-linux-amd64.tar.gz"
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
cardinal version
cardinal doctor
```

## Uninstall

```bash
cardinal bootstrap --remove
sudo rm /usr/local/bin/cardinal
sudo rm -rf ~/.cardinal
```
