<!-- dck-version:start -->
**Documentation version:** `1.25.9`
**Project release:** `v1.25.9`
<!-- dck-version:end -->

# Void Linux xbps packaging for `dck`

This directory contains a drop-in `template` for the upstream
`void-packages` repository (https://github.com/void-linux/void-packages),
which is the canonical place where Void's xbps packages are maintained.

## Why it's split like this

`void-packages` is a Git monorepo with hundreds of packages. We
don't want to fork the entire tree just to add one entry, and we
also don't want to maintain two build systems (the upstream
goreleaser-driven one and a parallel xbps one). So `dck`'s
xbps support ships entirely in this `contrib/void/` directory and
the user (or a future maintainer) copies the file in.

## How to use

```bash
# 1. Have a fork of void-packages. If you don't:
git clone https://github.com/void-linux/void-packages
cd void-packages

# 2. Place the template at the canonical location:
mkdir -p srcpkgs/dck
cp path/to/dck/contrib/void/template srcpkgs/dck/template

# 3. Replace the SHA256 placeholder with the real one. xbps-src prints it
#    on the first failed build attempt:
./xbps-src update-sums dck
# This edits `template` in place and replaces the placeholder.

# 4. Build:
./xbps-src pkg dck
# Produces: hostdir/binpkgs/dck-1.24.16_1.<arch>.xbps

# 5. (Local) install the result:
sudo xbps-install --repository=hostdir/binpkgs dck

# 6. Verify:
dck version
dck doctor
```

## Submitting to void-packages

Once the template builds locally for both x86_64 and aarch64-musl,
submit a PR against `[gro Nil at](mailto:gro.nil@…)`:

- https://github.com/void-linux/void-packages/pulls

Checklist before opening the PR:

| Field | Value |
|---|---|
| `maintainer` | `animesao <animesao@users.noreply.github.com>` |
| `upstream` | not present in `void-packages` yet; submit a separate maintainership request first if you want to merge from the void-packages repo |
| `repository` | `dck` |
| `version` | follow the upstream tag |
| `revision` | bump to 1, 2, ... when the template is changed without a version bump |

The Void maintainer workflow expects `xbps-src update-sums -d dck`
before committing — it locates the canonical `template`, computes
the distfile SHA, and rewrites the file in place.

## See also

- `docs/en/install-void.md` — full walk-through
- `docs/ru/install-void.md` — Russian mirror
