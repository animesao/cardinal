//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/config"
	"cardinal/internal/container"
)

func Down(args []string) {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	configPath := fs.String("f", "", "Path to config file")
	all := fs.Bool("a", false, "Remove all containers (ignore config)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing down options: %v\n", err)
		os.Exit(1)
	}

	freeArgs := fs.Args()
	var filter string
	if len(freeArgs) > 0 {
		filter = freeArgs[0]
	}

	if *all {
		allContainers, err := container.List(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
			os.Exit(1)
		}
		removed := 0
		for _, c := range allContainers {
			fmt.Printf("  Removing %s (%s)...\n", shortID(c.ID), c.Name)
			if err := c.Remove(true); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				continue
			}
			removed++
		}
		fmt.Printf("Down complete: %d containers removed\n", removed)
		return
	}

	cfg, path, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using config: %s\n", path)

	removed := 0
	for name := range cfg.Container {
		if filter != "" && name != filter {
			continue
		}

		c := container.FindByName(name)
		if c == nil {
			fmt.Printf("  %s: not found\n", name)
			continue
		}

		if err := c.Remove(true); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error removing: %v\n", name, err)
			continue
		}
		fmt.Printf("  %s: removed (%s)\n", name, shortID(c.ID))
		removed++
	}

	fmt.Printf("Down complete: %d containers removed\n", removed)
}
