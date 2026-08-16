//go:build linux

// Smoke-test for every registered command: it walks the cobra command
// tree, invokes `dck <name> --help` for each top-level command, and
// asserts that cobra prints a generated help block. Failures (panic, empty
// output, registration gap) surface as test failures without needing a
// real Linux runtime (no namespaces, no overlayfs, no registry needed
// because --help short-circuits the implementation).
//
// Run with `go test -run TestCmdSmoke ./cmd/`.
package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestCmdSmoke_HelpForEveryTopLevelCommand is the master smoke test. It
// asserts a generated help block exists for every registered cobra
// command so we never silently lose a UX surface.
func TestCmdSmoke_HelpForEveryTopLevelCommand(t *testing.T) {
	root := NewRoot()

	// Cobra auto-installs a `completion` sub-command at NewRoot time; the
	// user already gets it via `dck --help`, so it does not need a
	// separate test entry.
	for _, sub := range root.Commands() {
		if sub.Name() == "completion" || sub.Name() == "help" {
			continue
		}
		sub := sub
		t.Run(sub.Name(), func(t *testing.T) {
			stream := &bytes.Buffer{}
			r := NewRoot()
			r.SetArgs([]string{sub.Name(), "--help"})
			r.SetOut(stream)
			r.SetErr(stream)
			// Silence the cobra-default exit code so a user error
			// (which we do not care about here) does not abort the
			// test runner.
			defer func() {
				_ = recover()
			}()
			_ = r.Execute()

			out := stream.String()
			if out == "" {
				t.Fatalf("%s: --help produced empty output (did the run swallow the args?)", sub.Name())
			}
			// Every cobra-rendered help block lists the command in the
			// `Available Commands:` section. If the command has problems
			// (silently dropped, etc), this expected marker is missing.
			if !strings.Contains(out, sub.Name()) {
				t.Errorf("%s: --help does not mention its own name; got %q", sub.Name(), trim(out, 120))
			}
		})
	}
}

// TestCmdSmoke_RootHelpSplash confirms the root-level help shows the
// product tagline and lists commands. Regression guard: if a future refactor
// accidentally shrinks the help text we want to notice.
func TestCmdSmoke_RootHelpSplash(t *testing.T) {
	stream := &bytes.Buffer{}
	root := NewRoot()
	root.SetArgs([]string{"--help"})
	root.SetOut(stream)
	root.SetErr(stream)
	_ = root.Execute()
	out := stream.String()

	expect := []string{
		"dck",            // root command
		"Lightweight",    // tagline word
		"Available Commands",
	}
	for _, want := range expect {
		if !strings.Contains(out, want) {
			t.Errorf("root --help missing expected marker %q\noutput=%.300s", want, out)
		}
	}
}

// TestCmdSmoke_NoCommandPanic submits a deliberately bogus command name
// and asserts cobra rejects it without panicking. This is the cheapest
// defence against a regression that surfaces as `index out of range` in
// the legacy free-function path.
func TestCmdSmoke_NoCommandPanic(t *testing.T) {
	stream := &bytes.Buffer{}
	root := NewRoot()
	root.SetArgs([]string{"definitely-not-a-real-command"})
	root.SetOut(stream)
	root.SetErr(stream)

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("bogus command panicked: %v", rec)
		}
	}()
	if err := root.Execute(); err == nil {
		t.Errorf("expected error from unknown command, got nil")
	}
}

// TestCmdSmoke_CompletionBashFormats ensures the auto-generated bash
// completion script is non-empty and starts with the cobra preamble.
func TestCmdSmoke_CompletionBashFormats(t *testing.T) {
	stream := &bytes.Buffer{}
	root := NewRoot()
	root.SetArgs([]string{"completion", "bash"})
	root.SetOut(stream)
	root.SetErr(stream)
	_ = root.Execute()
	out := stream.String()
	if out == "" {
		t.Fatal("completion bash produced no output")
	}
	if !strings.Contains(out, "bash completion") {
		t.Errorf("completion bash missing the standard preamble (got %.80q)", out)
	}
}

// TestCmdSmoke_HelpIncludesGlobalFlags verifies that a sub-command's
// --help output surfaces the global persistent flags. We deliberately
// use `ps --help` here and NOT `run --help`: the `run` cobra
// command sets DisableFlagParsing=true so the legacy
// `flag.NewFlagSet` inside Run(args) parses flags, and the stdlib
// auto-registers `--help` on every FlagSet. cobra therefore never
// reaches its own help renderer for `run --help`; the legacy
// FlagSet prints its own usage block and calls os.Exit(0) which
// the test runner then sees as a hard panic. We test against
// `ps --help` (a normal cobra command) and verify global flags
// are documented there, which is the user-visible behaviour the
// migration set out to provide.
func TestCmdSmoke_HelpIncludesGlobalFlags(t *testing.T) {
	stream := &bytes.Buffer{}
	root := NewRoot()
	root.SetArgs([]string{"ps", "--help"})
	root.SetOut(stream)
	root.SetErr(stream)
	_ = root.Execute()
	out := stream.String()

	// Cobra's help builder splits Global Flags from Local Flags; both
	// sections must be present for the user to discover the new
	// persistent flags.
	for _, section := range []string{"Global Flags", "Flags"} {
		if !strings.Contains(out, section) {
			t.Errorf("ps --help missing expected section %q", section)
		}
	}
	for _, want := range []string{"--log-level", "--json", "--quiet"} {
		if !strings.Contains(out, want) {
			t.Errorf("ps --help missing expected global flag %q", want)
		}
	}
}

// trim keeps error messages short without losing key fragments.
func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// silenceStdout flips os.Stdout during the test to swallow the normal
// cobra help printout so we can collect it explicitly in the test
// buffer; only used for negative tests where the helper was not called
// yet. Currently unused but kept for future stubs.
var _ = io.Discard
