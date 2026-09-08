//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Logs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	follow := fs.Bool("f", false, "Follow log output")
	tail := fs.Int("tail", 0, "Show only last N lines")
	previous := fs.Bool("previous", false, "Show the previous run log")
	all := fs.Bool("all", false, "Show current and rotated logs")
	mustParse(fs, args, "logs")

	if fs.NArg() < 1 {
		fmt.Println("Usage: cardinal logs [-f] [--tail <n>] <container>")
		exitFunc(1)
	}

	c, err := container.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}

	if err := c.LogsWithOptions(*follow, *tail, *previous, *all); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
