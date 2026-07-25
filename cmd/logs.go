//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/container"
)

func Logs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output")
	tail := fs.Int("tail", 0, "Show only last N lines")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: dck logs [-f] [--tail <n>] <container>")
		os.Exit(1)
	}

	c, err := container.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.Logs(*follow, *tail); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
