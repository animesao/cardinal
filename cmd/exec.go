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
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	interactive := fs.Bool("i", false, "Interactive mode")
	tty := fs.Bool("t", false, "Allocate TTY")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing exec options: %v\n", err)
		os.Exit(1)
	}

	remaining := fs.Args()
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

	if err := c.ExecOpts(remaining[1:], *interactive, *tty); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
