<!-- cardinal-version:start -->
**Documentation version:** `2.0.4`
**Project release:** `v2.0.4`
<!-- cardinal-version:end -->

# Installing cardinal on Void Linux

Void uses **xbps** (the `xbps-install` family of tools) and a
community-maintained source tree called
[`void-packages`](https://github.com/void-linux/void-packages).
The canonical drop-in for `cardinal` lives under
[`contrib/void/`](../../contrib/void/); you copy the `template`
into your local fork of `void-packages` and build there.

## Quick path: local fork of void-packages

```bash
# 1. Have a fork of void-packages. If you don't:
git clone https://github.com/void-linux/void-packages
cd void-packages

# 2. Place the template at the canonical xbps-src path
mkdir -p srcpkgs/cardinal
cp path/to/cardinal/contrib/void/template srcpkgs/cardinal/template

# 3. Compute the distfile SHA and patch the `checksum=` line in-place.
#    xbps-src updates the template with the correct sha256:
./xbps-src update-sums cardinal

# 4. Build (this also fetches dependencies via the void-packages repo)
./xbps-src pkg cardinal
#   Produces: hostdir/binpkgs/cardinal-1.24.15_1.x86_64.xbps

# 5. (Local) install the produced package
sudo xbps-install --repository=hostdir/binpkags cardinal

# 6. Verify
cardinal version
sudo cardinal doctor
```

## Important: replace the SHA256 placeholder

The `template` ships with `checksum="sha256-PLACEHOLDER-..."` so
that your first `./xbps-src pkg cardinal` correctly fails with a "bad
checksum" error and `./xbps-src update-sums cardinal` can edit the file
in place with the verified `sha256-...` digest. Don't hand-edit the
checksum; always use `update-sums` (it handles both the tarball and
any future vendor-check) so you don't miss updates.

## Build for musl

Void ships both glibc and musl variants. `cardinal` is a pure-Go binary
and works on both, but you need to pick:

```bash
# glibc (default on x86_64)
./xbps-src pkg cardinal

# musl
./xbps-src -a x86_64-musl pkg cardinal
```

The `env = { CGO_ENABLED = "0"; }` baked into the upstream build
means the produced `.xbps` works on both libc flavours without
re-compiling per arch.

## Required kernel configuration

```bash
cardinal doctor
```

If `cardinal doctor` reports `WARN` on `user namespaces`:

```
# In /boot/grub/grub.cfg add to linux command line:
GRUB_CMDLINE_LINUX_DEFAULT="lsm=capability,landlock ... module.sig_enforce=1"
# Void's default vmlinuz already has CONFIG_USER_NS=y since 2020.
```

If `cgroups v2` shows `WARN`, mount it as default:

```
# /etc/fstab
none /sys/fs/cgroup cgroup2 defaults 0 0
```

## Submitting to void-packages

Once you have a green `./xbps-src pkg cardinal` for `x86_64-glibc`,
`x86_64-musl`, and `aarch64-*` matrices:

1. Fork https://github.com/void-linux/void-packages
2. Add cardinal maintainership request first (a one-line `maintainers.md` PR)
3. PR the `srcpkgs/cardinal/template` + computed checksum

The `maintainers.md` line should read:

```
animesao <animesao@users.noreply.github.com> cardinal
```

## Solving the rare "pivot_root" failure on Void

Void's default `runit` PID-1 does not need a real `init` daemon —
`cardinal run` just exec's the container entrypoint and works fine on
Void hosts. If you do hit a pivot_root failure inside a container,
add to the upstream `cmd/run` invocation `--no-pivot` (a cardinal flag
that swaps to `chroot` semantics) — this is supported in void out
of the box.

## See also

- `docs/en/install-void.md` — full walk-through
- `contrib/void/README.md` — submit-PR checklist
