//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

func StartCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck start <container>")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	c.Status = container.Created
	if err := c.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(c.ID[:12])
}
