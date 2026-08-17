//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

// psShowAll is set by the cobra wrapper before calling Ps.
var psShowAll bool

// Ps lists containers.
func Ps(args []string) {
	containers, err := container.List(psShowAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(containers) == 0 {
		if psShowAll {
			fmt.Println("No containers found")
		} else {
			fmt.Println("No running containers")
		}
		return
	}

	container.PrintContainers(containers)
}
