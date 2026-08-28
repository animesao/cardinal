<!-- cardinal-version:start -->
**Documentation version:** `2.0.11`
**Project release:** `v2.0.11`
<!-- cardinal-version:end -->

# Installing cardinal on Fedora / RHEL / CentOS

## Option 1: RPM Package (Recommended)

Download and install the `.rpm` package from GitHub Releases:

```bash
# Detect architecture
ARCH=$(rpm --eval '%{_arch}')
case "$ARCH" in
  x86_64)  SUFFIX="amd64" ;;
  aarch64) SUFFIX="arm64" ;;
  armv7hl) SUFFIX="armv6" ;;
  *)       echo "Unsupported: $ARCH"; exit 1 ;;
esac

# Get latest version
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "cardinal-${VERSION}-linux-${SUFFIX}.rpm" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-${SUFFIX}.rpm"
sudo rpm -i "cardinal-${VERSION}-linux-${SUFFIX}.rpm"
rm "cardinal-${VERSION}-linux-${SUFFIX}.rpm"
```

## Option 2: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/cardinal/main/install.sh | sudo bash
```

## Option 3: Binary Archive

```bash
# Download tar.gz
TAG=$(curl -fsSL https://api.github.com/repos/animesao/cardinal/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "cardinal-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/cardinal/releases/download/${TAG}/cardinal-${VERSION}-linux-amd64.tar.gz"
tar xzf "cardinal-${VERSION}-linux-amd64.tar.gz"
sudo mv "cardinal-${VERSION}/cardinal" /usr/local/bin/cardinal
rm -rf "cardinal-${VERSION}" "cardinal-${VERSION}-linux-amd64.tar.gz"
```

## Firewall

If using `firewalld`, allow container networking:

```bash
sudo firewall-cmd --permanent --add-masquerade
sudo firewall-cmd --reload
```

## Verify

```bash
cardinal version
cardinal doctor
```

## Uninstall

```bash
# If installed via RPM
sudo rpm -e cardinal

# Remove data
cardinal bootstrap --remove
sudo rm -rf ~/.cardinal
```
