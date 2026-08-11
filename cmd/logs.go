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
	previous := fs.Bool("previous", false, "Show the previous run log")
	all := fs.Bool("all", false, "Show current and rotated logs")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing logs options: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: dck logs [-f] [--tail <n>] <container>")
		os.Exit(1)
	}

	c, err := container.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.LogsWithOptions(*follow, *tail, *previous, *all); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
