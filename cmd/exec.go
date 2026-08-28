//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Exec(args []string) {
	// Normalize combined shorthands like -it / -ti before flag parsing
	// stdlib flag does not handle combined bools like pflag does.
	normalized := make([]string, 0, len(args)*2)
	for _, a := range args {
		if a == "-it" || a == "-ti" {
			normalized = append(normalized, "-i", "-t")
		} else {
			normalized = append(normalized, a)
		}
	}
	args = normalized

	// Manually extract -i and -t flags from ANY position (stdlib flag.Parse
	// stops at the first non-flag arg, so "exec <id> -i /bin/sh" would miss
	// the -i flag and leak it into the nsenter command).
	var interactive, tty bool
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-i" {
			interactive = true
			continue
		}
		if args[i] == "-t" {
			tty = true
			continue
		}
		remaining = append(remaining, args[i])
	}

	if len(remaining) < 2 {
		fmt.Println("Usage: cardinal exec [-i] [-t] <container> <cmd> [args...]")
		os.Exit(1)
	}

	c, err := container.Load(remaining[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if c.Status != container.Running {
		fmt.Fprintf(os.Stderr, "Container %s is not running\n", remaining[0])
		os.Exit(1)
	}

	if err := c.ExecOpts(remaining[1:], interactive, tty); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
