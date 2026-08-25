//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Rm(args []string) {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	// Cobra now whitelists unknown flags, but we still handle both -f and
	// -r for user convenience: `cardinal rm -r` as alias for force
	// (docker-style `rm -r` expectation). Both short and long forms are
	// supported.
	force := fs.Bool("f", false, "Force remove")
	forceR := fs.Bool("r", false, "Force remove (alias for -f)")
	forceLong := fs.Bool("force", false, "Force remove")
	// Suppress default flag error output — we handle errors ourselves
	// to keep consistent UX with other commands.
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing rm options: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: cardinal rm [-f] [-r] [--force] <container> [container...]")
		fmt.Println("  -f, -r, --force   Force remove (including running containers)")
		os.Exit(1)
	}

	isForce := *force || *forceR || *forceLong
	exitCode := 0
	for _, name := range fs.Args() {
		c, err := container.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			continue
		}
		if err := c.Remove(isForce); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			continue
		}
		fmt.Println(shortID(c.ID))
	}
	os.Exit(exitCode)
}
