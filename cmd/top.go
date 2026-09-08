//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Top(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal top <container>")
		exitFunc(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}

	if err := c.Top(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
