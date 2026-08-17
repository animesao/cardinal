<!-- dck-version:start -->
**Documentation version:** `1.25.2`
**Project release:** `v1.25.2`
<!-- dck-version:end -->

# Installing dck on Fedora / RHEL / CentOS

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
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

# Download and install
curl -fL -o "dck-${VERSION}-linux-${SUFFIX}.rpm" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-${SUFFIX}.rpm"
sudo rpm -i "dck-${VERSION}-linux-${SUFFIX}.rpm"
rm "dck-${VERSION}-linux-${SUFFIX}.rpm"
```

## Option 2: Universal Installer

```bash
curl -fsSL https://raw.githubusercontent.com/animesao/dck/main/install.sh | sudo bash
```

## Option 3: Binary Archive

```bash
# Download tar.gz
TAG=$(curl -fsSL https://api.github.com/repos/animesao/dck/releases/latest | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
VERSION="${TAG#v}"

curl -fL -o "dck-${VERSION}-linux-amd64.tar.gz" \
  "https://github.com/animesao/dck/releases/download/${TAG}/dck-${VERSION}-linux-amd64.tar.gz"
tar xzf "dck-${VERSION}-linux-amd64.tar.gz"
sudo mv "dck-${VERSION}/dck" /usr/local/bin/dck
rm -rf "dck-${VERSION}" "dck-${VERSION}-linux-amd64.tar.gz"
```

## Firewall

If using `firewalld`, allow container networking:

```bash
sudo firewall-cmd --permanent --add-masquerade
sudo firewall-cmd --reload
```

## Verify

```bash
dck version
dck doctor
```

## Uninstall

```bash
# If installed via RPM
sudo rpm -e dck

# Remove data
dck bootstrap --remove
sudo rm -rf ~/.dck
```
