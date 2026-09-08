//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Rename(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cardinal rename <container> <new-name>")
		exitFunc(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}

	if err := c.Rename(args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}

	fmt.Printf("%s renamed to %s\n", shortID(c.ID), args[1])
}
