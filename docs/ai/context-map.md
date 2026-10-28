<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# Context map

This file is the routing layer between the root [`AGENTS.md`](../../AGENTS.md)
and the rest of the repository. Its job is to keep an agent's context small and
correct: read one routing row, open the smallest set of files that can prove or
disprove the task, and stop.

It is not a tutorial and it is not a second copy of the architecture docs.

## Context tiers

| Tier | Read | When |
|---|---|---|
| 0 | [`AGENTS.md`](../../AGENTS.md) | Always. It contains the non-negotiables and the verification matrix. |
| 1 | This map + one local `AGENTS.md` | Before the first edit in an area. |
| 2 | One architecture document + [`docs/status/known-issues.md`](../status/known-issues.md) | Before trusting older docs or changing a contract. |
| 3 | Source files and tests that the task touches | Before editing. Code wins over every document. |
| 4 | An ADR in [`docs/decisions/`](../decisions/) | Only when the question is "why is it like this?". |
| 5 | [`docs/plans/`](../plans/) and release notes | Only for historical intent. Never as current-state evidence. |

Read budget: one routing row + one architecture doc + the source files named by
that doc. If the task still feels unclear, read a test, not more prose.

## Task router

| Task | Read first | Then open | Do not read |
|---|---|---|---|
| Add or change a CLI command/flag | [`cmd/AGENTS.md`](../../cmd/AGENTS.md) | `cmd/cobra_commands.go`, `cmd/cobra.go`, the legacy `func Command(args []string)`, `docs/en/commands.md` + `docs/ru/commands.md` | Every other `cmd/*.go` |
| Container lifecycle, namespaces, overlay, cgroups | [`internal/container/AGENTS.md`](../../internal/container/AGENTS.md) | [`architecture/runtime.md`](../architecture/runtime.md), `internal/container/start.go`, `init.go` | `vendor/` |
| Security policy, caps, seccomp, API auth | [`architecture/security.md`](../architecture/security.md) | `internal/container/security_policy.go`, `seccomp.go`, `hostpath.go`, `internal/api/server.go` | Historical audit snapshots |
| HTTP API, Docker compatibility, remote mode | [`internal/api/AGENTS.md`](../../internal/api/AGENTS.md) | [`architecture/api-and-orchestration.md`](../architecture/api-and-orchestration.md), `internal/api/types.go`, `internal/client/client.go` | `cmd/*` unrelated commands |
| State directory, prune, volumes, backups | [`architecture/state-and-storage.md`](../architecture/state-and-storage.md) | `internal/state/state.go`, `internal/container/prune.go`, `cmd/backup.go` | Cluster/FaaS code |
| Cluster, services, FaaS, DNS | [`architecture/api-and-orchestration.md`](../architecture/api-and-orchestration.md) | `internal/orchestrator/cluster.go`, `service.go`, `faas.go`, `dns.go`, `scheduler.go`, `cmd/cluster.go`, `cmd/service.go`, `cmd/fn.go` | Container internals, unless scheduling fails |
| Supervisor, restart policy, boot re-attach | [`architecture/runtime.md`](../architecture/runtime.md) | `cmd/supervisor.go`, `internal/container/restart_guard.go` | Container internals unrelated to restarts |
| Compose `up`/`down` | [`architecture/runtime.md`](../architecture/runtime.md) | `internal/config/compose.go`, `internal/config/interpolate.go`, `cmd/up.go`, `cmd/down.go` | Docker Compose upstream docs |
| Image pull/store/build | [`architecture/runtime.md`](../architecture/runtime.md) | `internal/image/pull.go`, `internal/image/store.go`, `internal/builder/builder.go` | Registry API docs |
| Build, CI, release, versioning | [`architecture/build-and-release.md`](../architecture/build-and-release.md) | `.github/workflows/build.yml`, `Makefile`, `scripts/sync-docs-version.sh` | Release notes |
| User-facing documentation | [`docs/AGENTS.md`](../AGENTS.md) | `docs/README.md`, both language mirrors | Architecture docs, unless behavior is in question |
| Large multi-step change | [`docs/work/README.md`](../work/README.md) | `docs/work/templates/task.md`, then `handoff.md` | Old plans |

## Source-of-truth order

1. Runtime behavior: code and tests.
2. Public contracts: `internal/api/types.go`, route handlers, `internal/client`,
   persisted JSON field names.
3. Build and release: `go.mod`, `Makefile`, `.github/workflows/*.yml`,
   `scripts/*.sh`.
4. Generated version data: `VERSION` + `scripts/sync-docs-version.sh`.
5. Maintainer docs: `README.md`, `docs/`, `SECURITY.md`.
6. Historical intent: `docs/plans/`, release notes, old audit snapshots.

If two sources disagree, do not average them. Use the higher source and record
the drift in [`docs/status/known-issues.md`](../status/known-issues.md).

## How to prove a claim

Prefer a command or a file:line reference over an assertion.

```sh
git status --short --branch
git log -5 --oneline
go list ./...
grep -R "SymbolName" internal cmd
go vet ./...
```

On Windows without Linux runtime support:

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; go vet ./...
```

A successful cross-compile proves that the tree type-checks for Linux. It does
not prove that namespaces, cgroups, overlayfs, iptables, or seccomp work at
runtime. Say so explicitly.

## Fact labels

- `Implemented` — code exists and a test or runtime check proves it.
- `Partial` — code exists but a known gap remains.
- `Planned` — accepted intent, no working implementation.
- `Deprecated` — still present but should not be used for new work.
- `Unknown` — not verified; say so instead of guessing.

Examples: the default seccomp filter is `Implemented`; custom seccomp profiles
are `Planned` and rejected fail-closed; AppArmor is `Planned` and rejected
fail-closed; rootless user-defined bridge networks are unsupported and fail
closed at start.

## Stop and escalate

Stop before editing when:

- the task requires weakening a control listed in root `AGENTS.md`;
- a persisted JSON name, Docker response field, or HTTP status semantic would
  change and the panel/wings impact is unknown;
- the only evidence is an old plan, audit snapshot, or release note;
- a destructive git command or secret handling is involved;
- the change cannot be verified in this environment.

Report the uncertainty. "Unknown on this host" is a valid result.

## Maintenance rules

- Keep this map short. Add a route only when a real task repeatedly needs it.
- Put durable subsystem detail in `docs/architecture/`, not here.
- Put durable "why" in an ADR, not in a changelog.
- Put verified contradictions in `docs/status/known-issues.md`, not in code
  comments.
- Remove a route when its target stops existing.
- Run the docs gates after editing Markdown:
  `sh scripts/sync-docs-version.sh --check`,
  `python3 scripts/check-docs-links.py`, and `sh scripts/audit.sh`.
  `audit.sh` is not a link checker; the Python script checks relative link
  targets but not heading anchors.

## Fresh-session exercise

A new session should be able to answer these questions using only root
`AGENTS.md` and this map, without opening the whole repository:

1. Is this repository runnable on Windows? (No; the root package is Linux-only.
   `go build ./...` succeeds only because Linux-tagged packages are skipped;
   `go build .` fails, so cross-compile instead.)
2. Where is container state? (`CARDINAL_DATA_DIR` if non-empty; otherwise uid 0
   uses `/root/.cardinal`, other users use `$HOME/.cardinal`, falling back to
   `/root` if the home lookup fails. See `internal/state/state.go`.)
3. Which security controls must not be weakened? (The root `AGENTS.md`
   "Non-negotiables" list is authoritative; the security architecture doc
   explains how each is enforced.)
4. Where does a CLI flag get registered? (Both layers: the cobra registry in
   `cmd/cobra_commands.go` and the legacy `func Command(args []string)`
   handler. Every top-level command except `ps` sets `DisableFlagParsing`, so
   the legacy `flag.NewFlagSet` binds its flags; `ps` parses through cobra.)
5. How do I verify a docs-only change? (Version check,
   `python3 scripts/check-docs-links.py`, and `audit.sh`; `audit.sh` is not a
   link checker.)
6. What is the current seccomp behavior? (Default profile applied in
   `internal/container/init.go` before `execve`; custom profiles rejected in the
   `startInternal` check; any AppArmor profile rejected in the same check, and
   AppArmor confinement is not applied. See the security doc and known issues.)
7. Which document is allowed to override an old plan? (None. Code and tests
   win; use the source-of-truth order and record the drift instead of treating
   a plan as current state.)

This exercise is normative: if a fresh session cannot answer from these two
files, fix the map or the entry point, not the plan.
