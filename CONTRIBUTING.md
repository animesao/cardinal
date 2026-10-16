<!-- cardinal-version:start -->
**Documentation version:** `2.4.1`
**Project release:** `v2.4.1`
<!-- cardinal-version:end -->

<p align="center">
  <img src="img/cardinal.png" alt="cardinal logo" width="120">
</p>

# Contributors

Thank you to everyone who helps improve **cardinal**.

## Maintainers

- [animesao](https://github.com/animesao) — project maintainer and primary author.

## Automation

- `github-actions[bot]` — automated versioning and release workflow updates.
- `dependabot[bot]` — automated dependency updates.

## Contributors

- [MASHINISTPROJECT](https://github.com/MASHINISTPROJECT) — Linux installer fixes (`install.sh`), image pull progress bar (`internal/image/pull.go`), uninstall script fixes (`uninstall.sh`) via PR #10, #12.

## How to contribute

Contributions are welcome:

1. Fork the repository and create a focused branch.
2. Make the smallest change that solves the problem.
3. Add or update tests for code changes.
4. Run the local checks before opening a pull request:

   ```bash
   gofmt -w $(git ls-files '*.go')
   go test ./... -count=1
   go vet ./...
   golangci-lint run ./...
   bash scripts/audit.sh
   bash scripts/sync-docs-version.sh
   ```

5. Update the relevant English and Russian documentation when behavior or commands change.
6. Open a pull request with a clear description of the change and its verification.

Please do not add a person to this file without their permission. New contributors can be added after a merged contribution or by request.

## Licensing of contributions

Cardinal is licensed under the Apache License, Version 2.0 (see LICENSE).
By contributing you agree your contribution is made under the same
Apache-2.0 terms (inbound = outbound, per Section 5 of the license),
unless a separate written agreement says otherwise.

Note: the project migrated from MIT to Apache-2.0 with the consent of all
past third-party contributors on record.

## Branching model

Permanent branches:

- `main` — stable, releasable code. No everyday development here.
- `develop` — integration branch for current development. Finished changes land here first.

Temporary branches (created from `develop`, deleted after merge):

| Prefix      | Use                          | Example                |
|-------------|------------------------------|------------------------|
| `feature/`  | New functionality            | `feature/volume-limit` |
| `fix/`      | Regular bug fixes            | `fix/config-parser`    |
| `refactor/` | Refactoring without new feat | `refactor/cli`         |
| `docs/`     | Documentation only           | `docs/api`             |

`hotfix/*` is the only exception: it is created from `main` for urgent fixes to published code, merged into `main`, then back-merged into `develop` so the fix is never lost.

Flow:

```text
feature/* → develop → main
fix/*      → develop → main
refactor/* → develop → main
docs/*     → develop → main
hotfix/*   → main (+ back-merge into develop)
```

Pull requests to `main` are accepted only from `develop` or `hotfix/*` (plus `dependabot/*`, which targets the default branch by design). CI (`build`, `Linux E2E`) runs on both `main` and `develop`, and the `branch-guard` workflow flags PRs opened against the wrong base branch.

## Packaging for a Linux distribution we don't yet ship

The [`.goreleaser.yaml`](.goreleaser.yaml) handles five formats out
of the box: `.deb`, `.rpm`, `.apk`, `.pkg.tar.zst` (Arch Linux),
plus the generic `.tar.gz`. These cover ~95% of active Linux
installations.

For distributions we do not yet ship a native package for —
**NixOS / Nix**, **Gentoo / Funtoo**, **Void Linux**, Alpine
derivatives, etc. — drop a tiny build descriptor under
[`contrib/`](contrib/) that matches the upstream build matrix
exactly:

```
contrib/
├── nix/
│   ├── flake.nix        # buildGoModule + lib.fakeHash
│   └── default.nix      # legacy non-flake expression
├── gentoo/
│   └── cardinal-1.24.15.ebuild
└── void/
    └── template         # xbps-src / void-packages drop-in
```

Every existing contributor (under `contrib/`) is built with:

```
CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -buildid= -X cardinal/cmd.version=${version}"
```

That flag set is the upstream build matrix in entirety; matching
it ensures the binary produced by your distribution's tooling is
**byte-identical** to the goreleaser-produced `.deb`/`.rpm`/`.apk`
artefact attached to the GitHub Release.

Style guide for new `contrib/<distro>/` entries:

- One-line filename conventions matching the host community
  (e.g. ebuilds in `gentoo/`, `flake.nix` in `nix/`, `template`
  for xbps).
- Always include a `README.md` that documents `version` /
  `replacement strategy` (so the next bump is a one-line patch).
- A short walk-through in `docs/en/install-<distro>.md` plus the
  Russian mirror in `docs/ru/install-<distro>.md`.
- A pointer entry in [README.md](README.md) under the
  Install Guides table.
