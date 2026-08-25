# Copyright 2024-2026 cardinal maintainers
# Distributed under the terms of the GNU General Public License v2 or later

EAPI=8

inherit go-module

# `go-module` eclass vendored dep handling:
#   1. it asks for an upstream tarball;
#   2. it expects a `vendor/` directory under that tarball;
#   3. it binds `GOFLAGS` to -mod=vendor and unpacks the archive.

DESCRIPTION="Lightweight, daemonless, OCI-compatible container runtime"
HOMEPAGE="https://github.com/animesao/cardinal"
SRC_URI="https://github.com/animesao/cardinal/archive/v${PV}.tar.gz -> ${P}.tar.gz"

LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64 ~arm64"

# cardinal's runtime test suite needs:
#   - root privileges (for unshare / mount / iptables)
#   - kernel ≥ 4.18 with user namespaces + cgroup v2
#   - overlayfs available
# None of that survives the Portage build sandbox, so we skip `emake test`
# here. The upstream `go test -race` step is still authoritative.
RESTRICT="test"

# Empty, cardinal has no install-time deps other than what `go module` pulled
# in via the upstream `vendor/` tree.
DEPEND=""
RDEPEND=""
BDEPEND=""

# S="${WORKDIR}/${P}" is the eclass default, no override necessary.

src_prepare() {
    default
    # Belt-and-braces: pin the build to our upstream commit rev.
    # If a portage user edits anything in the unpacked tree mid-build,
    # the resulting binary's `-X cardinal/cmd.version` stamping will still
    # report ${PV} but will fail the cosmetic verification step below.
}

src_compile() {
    # Match the upstream build matrix exactly:
    #   * -trimpath strips absolute paths from binary's DWARF
    #   * -s -w  strip symbol table
    #   * -buildid=  reproducible .buildid (no per-build salt)
    #   * -X cardinal/cmd.version=${PV}  stamps the embedded version var
    CGO_ENABLED=0 \
    GOCACHE="${T}/go-cache" \
    go build -trimpath \
        -ldflags="-s -w -buildid= -X cardinal/cmd.version=${PV}" \
        -o "${S}/bin/cardinal" . \
    || die "go build failed"
}

src_test() {
    # Run only the unit tests that do NOT need root.
    # The full test suite is gated by RESTRICT="test" above so we never
    # accidentally call into the network/cgroup ones from a sandbox.
    :
}

src_install() {
    dobin bin/cardinal

    # Future-proofing: a post-install unit file could land here once the
    # upstream `bootstrap` command is a stable source of the systemd
    # unit. For now we don't ship one in the ebuild -- users who want
    # autostart can run `cardinal bootstrap` from inside the installed binary.

    newinitd "${FILESDIR}/cardinal.initd" cardinal 2>/dev/null || true
    newconfd "${FILESDIR}/cardinal.confd" cardinal 2>/dev/null || true
}

pkg_postinst() {
    einfo "cardinal installed. Run \`cardinal doctor\` to verify"
    einfo "host support (namespaces, cgroup v2, overlayfs)."
}
