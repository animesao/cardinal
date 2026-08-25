//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/container"
)

func initContainer(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: cardinal init <container-id> <merged-path>\n")
		os.Exit(1)
	}

	if err := container.InitContainer(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
		os.Exit(1)
	}
}
