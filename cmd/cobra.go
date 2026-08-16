//go:build linux

// Package cmd hosts the cobra command tree for dck.
//
// Cobra replaces the legacy hand-rolled dispatcher in root.go (a single
// 130-line switch statement) so we get free shell completion, structured
// help, and uniform --json / --quiet / --log-level global flags. Each
// existing `func X(args []string)` in this package is wired into cobra as
// the `Run` action of its sub-command; this preserves the behaviour of
// every previously-working invocation while gaining the standard CLI UX.
//
// The package-internal `Execute` function is called from main.go.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"dck/internal/log"
)

// NewRoot constructs the cobra root command. Splitting construction from
// execution makes unit testing trivial: tests can build the tree with a
// custom IO buffer and assert on the produced output.
func NewRoot() *cobra.Command {
	// Global options surfaced on the root command so every sub-command
	// can inspect them via cmd.Root().PersistentFlags() lookups.
	var (
		logLevel string
		jsonOut  bool
		quiet    bool
	)

	rootCmd := &cobra.Command{
		Use:   "dck",
		Short: "Lightweight Linux container runtime",
		Long: `dck — Lightweight Linux container runtime.

A daemonless, OCI-compatible runtime for Linux that uses namespaces,
overlayfs, cgroups, capability dropping and seccomp filtering to isolate
untrusted workloads. Mirrors the docker CLI surface where it is useful,
and adds tooling for cluster orchestration, FaaS, blueprints and
Docker-Compose-style up/down/up commands.`,
		Version:      version,
		SilenceUsage: true,
		// Run the help when no args are provided; the legacy root used to
		// print help on `dck` with no args and exit 1, but `SilenceUsage`
		// plus the explicit Run keeps the same UX without an extra exit
		// code on the success path.
		RunE: func(cmd *cobra.Command, args []string) error {
			applyLogOptions(logLevel, jsonOut, quiet)
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyLogOptions(logLevel, jsonOut, quiet)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log verbosity (debug|info|warn|error)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output where supported")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")

	// All commands are registered via init() in this package so the order
	// does not depend on file compilation order.
	for _, c := range allCommands {
		rootCmd.AddCommand(c)
	}
	return rootCmd
}

// applyLogOptions funnels global logging flags into the log package. The
// package-level state is sufficient because dck is a single-process CLI
// (no concurrent goroutines that need to race on the logger).
func applyLogOptions(level string, jsonOut, quiet bool) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.LevelDebug)
	case "warn", "warning":
		log.SetLevel(log.LevelWarn)
	case "error", "err":
		log.SetLevel(log.LevelError)
	default:
		log.SetLevel(log.LevelInfo)
	}
	if quiet {
		log.SetLevel(log.LevelError)
	}
	log.SetJSON(jsonOut)
}

// Execute is the entry point invoked from main.go. It sets up the cobra
// command tree and runs it against the process arguments.
func Execute() {
	cmd := NewRoot()
	if err := cmd.Execute(); err != nil {
		// Every registered sub-command sets SilenceErrors=true so legacy
		// command implementations can surface errors in their own way
		// (most of them call os.Exit(1) directly after printing to
		// stderr). That setting also causes cobra NOT to print the
		// error itself, so we must replicate that printing here — without
		// this line, an unknown flag leaves the user with an empty
		// stderr and exit code 1, which is impossible to debug.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
