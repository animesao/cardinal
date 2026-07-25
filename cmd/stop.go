//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/container"
)

func Stop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	all := fs.Bool("all", false, "Stop all running containers")
	fs.Parse(args)

	if *all {
		containers, err := container.List(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, c := range containers {
			if err := c.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping %s: %v\n", c.ID[:12], err)
				continue
			}
			fmt.Println(c.ID[:12])
		}
		return
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Println("Usage: dck stop [--all] <container>")
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

	fmt.Println(c.ID[:12])
}
