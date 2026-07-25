//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

func Top(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck top <container>")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.Top(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
