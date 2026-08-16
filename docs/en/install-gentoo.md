<!-- dck-version:start -->
**Documentation version:** `1.24.18`
**Project release:** `v1.24.18`
<!-- dck-version:end -->

# Installing dck on Gentoo / Funtoo / Calculate

The canonical ebuild lives under
[`contrib/gentoo/`](../../contrib/gentoo/). You'll either:

1. Run `dck` from a **local overlay** that you maintain by hand,
   or
2. Submit a PR to https://github.com/gentoo/guru (target category:
   `app-containers/dck`) so it becomes available to the wider
   community.

## Quick path: local overlay

```bash
# Create or reuse a local overlay root
if [ ! -d /var/db/repos/dck-overlay ]; then
    sudo install -d -o root -g portage \
        /var/db/repos/dck-overlay/{profiles,metadata,app-containers/dck}
    echo 'dck-overlay' | sudo tee /var/db/repos/dck-overlay/profiles/repo_name
    cat <<'EOF' | sudo tee /var/db/repos/dck-overlay/metadata/layout.conf
masters = gentoo
EOF
fi

# Drop the ebuild into the conventional category
sudo cp contrib/gentoo/dck-1.24.15.ebuild \
        /var/db/repos/dck-overlay/app-containers/dck/

# Pin it via repos.conf so portage sees the overlay
repos_conf="/etc/portage/repos.conf/dck-overlay.conf"
if [ ! -f "$repos_conf" ]; then
    sudo tee "$repos_conf" >/dev/null <<'EOF'
[dck-overlay]
location = /var/db/repos/dck-overlay
priority = 50
EOF
fi

# Generate the SHA256 manifest
sudo chown -R portage:portage /var/db/repos/dck-overlay
cd /var/db/repos/dck-overlay/app-containers/dck
sudo Manifest-md5 dck-1.24.15.ebuild || \
sudo Manifest-sha256 dck-1.24.15.ebuild

# Emerge
sudo emerge --sync
sudo emerge --ask app-containers/dck
```

If Portage complains about missing keyword `~amd64` on your arch,
add to `/etc/portage/package.accept_keywords/`:

```
=app-containers/dck-1.24.15 ~amd64
```

## Required USE flags / dependencies

The ebuild isUSE-free. Internally it requires:

- `>=dev-lang/go-1.23` (matches the upstream `go.mod` floor)
- Kernel ≥ 4.18 with user namespaces + cgroup v2 (verified by `dck doctor`)
- syscall-mount / `sys-apps/util-linux` to provide `mount`/`unshare`

Optional but recommended:

```
sys-apps/iproute2   # for network plugins (bridge / iptables)
```

## Submitting to ::guru

If you prefer the community-overlay route, prepare a PR against
`gentoo/guru`. The conventional structure is:

```
app-containers/dck/
├── dck-1.24.15.ebuild
├── dck-9999.ebuild          # git-versioned live build
└── files/
    └── dck.initd            # (empty stub today)
```

The ebuild in this repository has placeholders for `newinitd` /
`newconfd` already wired up; fill them in if you want to ship a
systemd/OpenRC supervisor too.

A PR description that gets accepted fast:

> Adds `app-containers/dck` to ::guru. dck is a Go 1.23+ pure-Go
> binary that mirrors the docker CLI; it does not require a daemon.
> The ebuild uses `go-module.eclass` and consumes the upstream
> `vendor/` tree. Tests are RESTRICT-ed because they need
> namespaces + root.

## Verify after install

```bash
$ which dck
/usr/bin/dck
$ dck version
dck version 1.24.15 (commit 52ba511, ...)
$ sudo dck doctor
... kernel-feature report ...
```

If `dck doctor` reports `WARN` on `user namespaces`, your kernel
needs:

```
CONFIG_USER_NS=y
CONFIG_USER_NS_FASYNC=y
```

(Usually `=y` for both on ~amd64 kernels shipped in sys-kernel/gentoo-sources
since 5.10; older kernels need explicit enabling.)
