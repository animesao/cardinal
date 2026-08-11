//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

func Restart(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck restart <container>")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if c.Status == container.Running {
		if err := c.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping: %v\n", err)
			os.Exit(1)
		}
	}

	if err := c.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(shortID(c.ID))
}
