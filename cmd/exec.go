//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/container"
)

func Exec(args []string) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	interactive := fs.Bool("i", false, "Interactive mode")
	tty := fs.Bool("t", false, "Allocate TTY")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Println("Usage: dck exec [-i] [-t] <container> <cmd> [args...]")
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
