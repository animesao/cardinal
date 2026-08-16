//go:build linux

// Package cmd hosts the cobra command tree for dck.
//
// The actual command-line dispatch is implemented in cobra.go (NewRoot,
// Execute, applyLogOptions) and cobra_commands.go (per-command wrappers
// around the legacy free functions). This file holds only the package-level
// URL constants that other command files rely on.
package cmd

var (
	repoURL          = "https://raw.githubusercontent.com/animesao/dck"
	releaseURL       = "https://github.com/animesao/dck"
	blueprintRepoURL = "https://raw.githubusercontent.com/dck-organization/dck-blueprints"
)
