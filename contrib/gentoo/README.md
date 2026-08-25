<!-- cardinal-version:start -->
**Documentation version:** `2.0.1`
**Project release:** `v2.0.1`
<!-- cardinal-version:end -->

# Gentoo packaging for `cardinal`

This directory carries a Portage ebuild that builds `cardinal` from source
matching the upstream release pipeline exactly:

```
CGO_ENABLED=0 \
go build -trimpath \
    -ldflags="-s -w -buildid= -X cardinal/cmd.version=${PV}" \
    -o bin/cardinal .
```

The upstream `vendor/` tree is left untouched and consumed via
`go-module.eclass`'s `-mod=vendor` default.

## Tree layout

```
contrib/gentoo/
├── cardinal-1.24.16.ebuild     # bump the version on each release
└── README.md              # this file
```

## Local overlay (recommended for end users)

```bash
# 1. Create a local overlay root if you don't already have one
sudo mkdir -p /var/db/repos/cardinal-overlay/{profiles,metadata}

echo "cardinal-overlay" | sudo tee /var/db/repos/cardinal-overlay/profiles/repo_name
cat <<'EOF' | sudo tee /var/db/repos/cardinal-overlay/metadata/layout.conf
masters = gentoo
EOF

# 2. Copy the ebuild into your overlay
sudo cp contrib/gentoo/cardinal-1.24.16.ebuild \
        /var/db/repos/cardinal-overlay/app-containers/cardinal/cardinal-1.24.16.ebuild

# 3. Generate an SHA256 manifest
sudo chown -R portage:portage /var/db/repos/cardinal-overlay
sudo Manifest-md5 /var/db/repos/cardinal-overlay/app-containers/cardinal/cardinal-1.24.16.ebuild || true

# 4. Emerge
sudo emerge --sync cardinal-overlay
sudo emerge --ask app-containers/cardinal
```

## Submitting to ::guru or ::gentoo

If you want the ebuild maintained in a community overlay, prepare a
PR against:

- https://github.com/gentoo/guru (for `~amd64` initially)
- https://github.com/gentoo/gentoo (for stabilisation later, requires maintainer review)

Conventional category: `app-containers/cardinal` (consistent with
`app-containers/docker`, `app-containers/podman`, etc.).

REQUIRED before sending the PR:

| Field | Value |
|---|---|
| `DESCRIPTION` | lightweight, daemonless, OCI-compatible container runtime |
| `HOMEPAGE` | https://github.com/animesao/cardinal |
| `LICENSE` | MIT (must match upstream LICENSE file) |
| `SLOT` | 0 |
| `KEYWORDS` | start with `~amd64`, add more as testing progresses |
| `RESTRICT` | `test` (sandbox has no namespaces) |
| `DEPEND` | `>=dev-lang/go-1.23` (matches upstream `go.mod`); see note below |
| `RDEPEND` | empty for now, eventually `sys-apps/iproute2` if we add `net_admin` plumbing |

Note on `DEPEND`: many current overlays omit the Go compiler from
DEPEND because the `go-module.eclass` declares it transitively.
We explicitly require `>=dev-lang/go-1.23` here so an old Go version
gets blocked at dep resolution instead of failing the build with an
obscure `go.mod requires go 1.23` error.

## See also

- `docs/en/install-gentoo.md` — full walk-through
- `docs/ru/install-gentoo.md` — the same in Russian
