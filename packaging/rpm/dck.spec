Name:           dck
Version:        1.23.2
Release:        1%{?dist}
Summary:        Lightweight container runtime for Linux

License:        MIT
URL:            https://github.com/animesao/dck
Source0:        %{url}/releases/download/v%{version}/dck-linux-%{_arch}

Requires:       iptables
Requires:       iproute
Requires:       procps-ng
Requires:       curl

%description
dck is a lightweight container runtime that provides container creation,
management, monitoring, and orchestration capabilities using Linux
namespaces, cgroups v2, and OverlayFS.

%prep
# No source to unpack — binary is pre-built

%build
# Binary is pre-built

%install
install -Dm755 %{SOURCE0} %{buildroot}/usr/bin/dck

# Systemd service
install -Dm644 /dev/stdin %{buildroot}/usr/lib/systemd/system/dck-bootstrap.service <<'EOF'
[Unit]
Description=dck container runtime supervisor
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/dck supervisor
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

%post
systemctl daemon-reload 2>/dev/null || true
echo "dck installed. Run 'dck bootstrap --install' to enable boot recovery."

%preun
if [ $1 -eq 0 ]; then
    systemctl stop dck-bootstrap 2>/dev/null || true
    systemctl disable dck-bootstrap 2>/dev/null || true
fi

%postun
systemctl daemon-reload 2>/dev/null || true

%files
/usr/bin/dck
/usr/lib/systemd/system/dck-bootstrap.service

%changelog
* Sat Aug 12 2026 animesao <animesao@users.noreply.github.com> - 1.23.2-1
- Initial RPM package
