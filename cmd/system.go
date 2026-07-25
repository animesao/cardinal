//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

func System(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck system <command>")
		fmt.Println("Commands:")
		fmt.Println("  prune    Remove unused containers and images")
		os.Exit(1)
	}

	switch args[0] {
	case "prune":
		if err := container.SystemPrune(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown system command: %s\n", args[0])
		fmt.Println("Usage: dck system prune")
		os.Exit(1)
	}
}
