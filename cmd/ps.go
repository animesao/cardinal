//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/container"
)

func Ps(args []string, showAll bool) {
	containers, err := container.List(showAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(containers) == 0 {
		if showAll {
			fmt.Println("No containers found")
		} else {
			fmt.Println("No running containers")
		}
		return
	}

	container.PrintContainers(containers)
}
