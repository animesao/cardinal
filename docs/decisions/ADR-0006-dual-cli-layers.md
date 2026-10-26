<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# ADR-0006: Cobra registry over legacy command functions

- **Status:** Accepted, with migration debt
- **Date:** 2026-10-26
- **Scope:** `cmd/`, CLI behavior, help and completion

## Context

The CLI began as a hand-written dispatcher with one `func Command(args []string)`
per command. It worked, but had no shell completion, inconsistent help, and no
uniform handling of global flags. A full rewrite of every command into cobra
`RunE` functions would have risked behavior changes across dozens of commands.

## Decision

Build a cobra command tree whose leaves are thin wrappers around the existing
legacy functions. Register the commands in one table
(`cmd/cobra_commands.go`) so the exposed surface is auditable. Keep legacy
`flag.NewFlagSet` parsing for most commands by disabling cobra flag parsing, and
migrate flags to cobra incrementally.

## Evidence

- `cmd/cobra.go` constructs the root command and documents the migration.
- `cmd/cobra_commands.go` defines `commandSpec`, `allCommands`, and `register`;
  every wrapper calls a legacy `runFn`.
- `register` sets `DisableFlagParsing = true` for almost every command, with
  `ps` kept on cobra parsing and `run`/`serve` explicitly documented.
- `stripGlobalFlags` and `extractRemoteFlags` compensate for cobra leaving
  global and remote flags in raw args.
- `cmd/cobra_test.go`, `cmd/smoke_test.go`, and command-specific tests cover the
  registration and legacy behavior.
- `Makefile` has a `verify-cobra` target (`go vet ./cmd/...`,
  `go build ./cmd/...`).

## Consequences

Positive:

- shell completion and structured help without rewriting every command;
- one auditable list of exposed commands;
- legacy behavior and tests remain valid during the migration;
- new commands can adopt cobra flags without forcing a flag day.

Negative:

- two parsing layers: cobra flags and stdlib `flag.FlagSet`;
- global flags must be stripped before legacy parsing, which is easy to get
  wrong;
- `--help` behavior differs between `run`/`serve` (disabled parsing) and other
  commands;
- root help text can drift from actual remote capabilities;
- adding a command requires updating both the registry and the legacy function.

## Rules

1. Register every new top-level command in `cmd/cobra_commands.go`.
2. If a command keeps legacy parsing, add a test that proves its flags reach the
   legacy parser.
3. If a command uses cobra parsing, do not mix in a second `flag.FlagSet`.
4. Keep `cardinal <cmd> --help` working on Linux; a compile check is not enough.
5. When a command becomes remote-capable, update the root help and
   `docs/architecture/api-and-orchestration.md`.

## Alternatives rejected

- **Big-bang rewrite to cobra `RunE`.** Rejected because it would change flag
  parsing and error behavior across the whole CLI at once.
- **Keep only the hand-written dispatcher.** Rejected because it blocks
  completion and uniform help.
- **One command per package with a shared interface.** Rejected as a larger
  refactor with no immediate user benefit.

## Related

- [`../architecture/build-and-release.md`](../architecture/build-and-release.md)
- [`../../cmd/AGENTS.md`](../../cmd/AGENTS.md)
- [`../status/known-issues.md`](../status/known-issues.md)
