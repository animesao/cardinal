<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# cmd — CLI command surface

Read the root [`AGENTS.md`](../AGENTS.md) and
[`docs/ai/context-map.md`](../docs/ai/context-map.md) first. This file only
covers traps specific to the command layer.

## Shape

- `cmd/cobra.go` builds the root command, global flags, logging setup, and
  `Execute()`.
- `cmd/cobra_commands.go` is the single registry: `commandSpec`, `allCommands`,
  `register()`, the `init()` table, and help-only subcommands.
- Each registered command delegates to a legacy `func Command(args []string)`
  handler in another file. The legacy handler parses its own flags with the
  standard library `flag` package.
- `cmd/remote.go` injects `--host/-H/--token` into package variables for
  commands that use `internal/client`.
- `main.go` calls `cmd.Execute()`; the package is Linux-only.

Both layers must stay in sync. A cobra command without a matching
`func` implementation fails `scripts/audit.sh`.

## Adding or changing a command

1. Define or update the legacy handler first.
2. Register it once in the `init()` table in `cmd/cobra_commands.go`.
3. If the command takes flags, decide which parser owns them:
   - every top-level command except `ps` sets `DisableFlagParsing` and forwards
     raw args to the legacy `flag.NewFlagSet`;
   - `ps` is the exception that parses `-a` through cobra;
   - `run` and `serve` have extra handling (global-flag stripping, `--help`,
     and, for `serve`, the server's own `--host/--token` flags).
4. Add or update tests in `cmd/*_test.go`.
5. Update the user-facing surface: `docs/en/commands.md`,
   `docs/ru/commands.md`, the tables in `docs/README.md`, and the root help
   string in `cmd/cobra.go` when the change is visible there.
6. Run the CLI docs checks.

Do not add a cobra flag to a `DisableFlagParsing` command and assume it works.
Cobra never sees those arguments.

## Flag and argument traps

- `stripGlobalFlags` removes `--json`, `--quiet`, and `--log-level` before
  forwarding raw args. It does not remove every global flag.
- `extractRemoteFlags` removes `--host/-H/--token` and stores them in
  `remoteHost`/`remoteToken`. `serve` is deliberately excluded because those
  names are the server's own bind/auth flags.
- `serve` and `run` keep their legacy stdlib flag parsing. The serve token must
  reach that parser; do not route it through the remote extractor.
- `hasLongHelpArgument` makes `--help`/`-h` work for commands that bypass cobra
  parsing. `run` uses `hasRunHelpArgument` instead: `-h` is the container
  hostname flag there, and scanning stops at the image/container command so
  container flags such as `httpd -h /` are forwarded verbatim. Subcommands such
  as `backup create` use the same delegation pattern.
- `SilenceErrors` is set on registered commands. `Execute()` prints the final
  error and exits 1; do not add a second printer.
- Legacy handlers may call `os.Exit` directly. New code should return errors
  where the surrounding handler already supports it.

## Remote mode

Remote commands are selected by `remoteHostResolved()` and authenticated by
`remoteTokenResolved()`. Environment fallbacks are `CARDINAL_REMOTE_HOST` and
`CARDINAL_TOKEN`.

Currently remote-capable: `ps`, `start`, `stop`, `restart`, `logs`, `exec`,
`inspect`, `info`, `top`, `stats`, and `events`. The client calls only the HTTP
endpoints implemented by `internal/api`; there is no generic passthrough. Do not
document a command as remote-capable without a handler and a test.

The root help still says "read-only remote mode"; that text is stale. Track it
in [`docs/status/known-issues.md`](../docs/status/known-issues.md) rather than
copying the phrase.

## Verification

On Linux:

```sh
gofmt -l cmd
go test ./cmd/... -count=1
go vet ./cmd/...
go run . <command> --help
```

On Windows, cross-compile only:

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; go vet ./...
```

A cross-build does not prove that a command works. Say that explicitly.

Useful tests: `TestNewRoot_RegistersAllExpectedCommands`,
`TestRunCommand_DisablesFlagParsing`, `TestLegacyFlagCommands_LongHelpDoesNotExit`,
`TestExtractRemoteFlags`, and the `httptest`-based remote tests in
`cmd/remote_exec_test.go`.

Docs gate after changing command text or Markdown:

```sh
sh scripts/sync-docs-version.sh --check
python3 scripts/check-docs-links.py
sh scripts/audit.sh
```
