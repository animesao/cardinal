<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# docs — user and maintainer documentation

Read the root [`AGENTS.md`](../AGENTS.md) and
[`ai/context-map.md`](ai/context-map.md) first. This file covers the rules that
keep the documentation set from drifting.

## Layout

| Path | Audience | Notes |
|---|---|---|
| [`README.md`](README.md) | Users | Landing page and command index. Its top version block is manual; see `D-08`. |
| [`en/`](en/) and [`ru/`](ru/) | Users | Mirrored file-for-file. Update both or neither. |
| [`COMMANDS.md`](COMMANDS.md) | Users | Legacy command table. Some entries are stale (`D-01`, `D-02`). |
| [`build.md`](build.md) | Contributors | Build/CI guide aligned with the current toolchain and workflows. Use [`architecture/build-and-release.md`](architecture/build-and-release.md) for deeper release facts. |
| [`AUDIT.md`](AUDIT.md) | Maintainers | Historical audit snapshot, not current-state evidence (`D-06`). |
| [`plans/`](plans/) | Maintainers | Historical intent only. Never cite a plan as proof of current behavior. |
| [`architecture/`](architecture/) | Maintainers/agents | Current subsystem behavior with file evidence. |
| [`decisions/`](decisions/) | Maintainers/agents | Durable "why" answers (ADRs). |
| [`status/known-issues.md`](status/known-issues.md) | Maintainers/agents | Verified drift and technical gaps. Read before trusting older docs. |
| [`ai/context-map.md`](ai/context-map.md) | Agents | Routing layer and context budget. |
| [`work/`](work/) | Agents/maintainers | Task and handoff records for multi-step work. |

## Version markers

Every Markdown file except a basename `README.md` or `CHANGELOG.md` must contain
exactly one pair of the `cardinal-version` start/end HTML comments at the top.
The exact block is visible at the top of this file; do not copy it into a fenced
code sample in another page, because the check counts every literal occurrence.

`scripts/sync-docs-version.sh` inserts the block at the top of a new file and
updates stale values. `--check` fails on a missing, duplicated, or out-of-order
pair. Do not hand-edit the text between the markers; run the script.

`README.md` basenames are excluded from that generic scan and have their own
handling. Root `README.md` has a badge and a release block; `--check` validates
those, but not the top marker block. `docs/README.md` and the
`contrib/*/README.md` top markers are manual sync points; see `D-08`.

## Content rules

- Code and tests override every document here. If a page contradicts the code,
  fix the page or record the drift; do not "fix" the code to match prose.
- Do not duplicate architecture detail in user guides. Link to
  [`architecture/`](architecture/) instead.
- Update both `docs/en/<name>.md` and `docs/ru/<name>.md`. The two directories
  currently have identical file sets; keep it that way.
- Command docs must match `cmd/cobra_commands.go` and the actual `--help` output.
  A command that is registered but undocumented is a bug in the docs.
- Security claims must match [`architecture/security.md`](architecture/security.md):
  the default seccomp profile is applied, custom seccomp and AppArmor are
  rejected fail-closed, and `no_new_privs` is always set.
- Relative links are preferred. External links should point to stable upstream
  pages, not search results or temporary URLs.
- Never paste tokens, passwords, private keys, or real customer data into docs
  or examples. Use `$CARDINAL_TOKEN`, placeholders, or env indirection.
- Historical documents (`plans/`, `AUDIT.md`, release notes) stay historical.
  Add a date or a superseded note instead of rewriting them to look current.

## Verification

```sh
sh scripts/sync-docs-version.sh --check
python3 scripts/check-docs-links.py
sh scripts/audit.sh
```

`audit.sh` checks for plain-looking credentials in `docs/en` and `docs/ru` and
for tracked key material. It is not a link checker. `check-docs-links.py` scans
Markdown visible to git outside `vendor/` (tracked plus untracked, excluding
ignored files), ignores fenced code blocks and external URLs, and reports
relative link targets that do not exist. It checks file existence only, not
heading anchors. On Windows, use `python` if `python3` is not on `PATH`.

On Windows without `bash`, call the POSIX scripts with `sh` as shown above.
`make docs`, `make docs-links`, and `make docs-check` work without `bash`;
`make audit` does not, because its recipe calls `bash`.

## Before closing a documentation change

- [ ] `sync-docs-version.sh --check` passes.
- [ ] Both language mirrors are updated, or the change is maintainer-only.
- [ ] `python3 scripts/check-docs-links.py` passes.
- [ ] Command examples match the current registry and flags.
- [ ] New drift is recorded in [`status/known-issues.md`](status/known-issues.md).
- [ ] Security statements match the code, not an older audit.
