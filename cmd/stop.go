//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Stop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	all := fs.Bool("all", false, "Stop all running containers")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing stop options: %v\n", err)
		os.Exit(1)
	}

	if *all {
		containers, err := container.List(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, c := range containers {
			if err := c.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping %s: %v\n", shortID(c.ID), err)
				continue
			}
			fmt.Println(shortID(c.ID))
		}
		return
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Println("Usage: cardinal stop [--all] <container>")
		os.Exit(1)
	}

	c, err := container.Load(remaining[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(shortID(c.ID))
}
