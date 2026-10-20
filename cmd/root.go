//go:build linux

// Package cmd hosts the cobra command tree for cardinal.
//
// The actual command-line dispatch is implemented in cobra.go (NewRoot,
// Execute, applyLogOptions) and cobra_commands.go (per-command wrappers
// around the legacy free functions). The package-level URL constants live in
// release.go, which is platform-independent.
package cmd
