<!-- cardinal-version:start -->
**Documentation version:** `2.4.10`
**Project release:** `v2.4.10`
<!-- cardinal-version:end -->

# Task: <short title>

- **Status:** `Planned` | `In progress` | `Blocked`
- **Owner:** <name or agent>
- **Started:** <YYYY-MM-DD>
- **Branch / commit:** <branch or `main@<sha>`>
- **Related issue / plan:** <link or "none">

## Goal

One paragraph. What observable outcome should exist when this is done?

## Scope

- <in scope>

## Non-goals

- <explicitly out of scope>

## Current behavior and evidence

State what the code does today, not what older docs claim. Use the
[`context map`](../../ai/context-map.md) to find the right files.

| Claim | Evidence (file, test, command) | Fact label |
|---|---|---|
| <claim> | `<path>:<line>` or `<command>` | `Implemented` |

## Constraints

- Security controls from the root [`AGENTS.md`](../../../AGENTS.md) must not be
  weakened.
- Persisted JSON names and Docker-compatible response fields are contracts.
- <other constraints>

## Plan

1. <step> — verify with `<command>`.
2. <step> — verify with `<command>`.

## Verification

| Check | Command | Expected | Actual |
|---|---|---|---|
| Format | `gofmt -l <files>` | no output | |
| Unit tests | `go test <packages> -count=1` | pass | |
| Cross-build (Windows host) | `$env:GOOS='linux'; go build ./...` | pass | |
| Docs | `sh scripts/sync-docs-version.sh --check` | synchronized | |
| Audit | `sh scripts/audit.sh` | `FAIL: 0` | |

Mark any check that cannot run in this environment as "not run" with the reason.

## Open questions

- <question> — owner, needed by, impact.

## Done criteria

- [ ] Behavior matches the goal.
- [ ] Tests cover the changed behavior.
- [ ] Documentation and both language mirrors are updated if user-facing.
- [ ] `known-issues.md` is updated for newly found or fixed drift.
- [ ] Handoff records exact results, including failures.
