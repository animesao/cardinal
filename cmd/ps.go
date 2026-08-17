//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/container"
)

func Ps(args []string) {
	fs := flag.NewFlagSet("ps", flag.ExitOnError)
	all := fs.Bool("a", false, "Show all containers (running + stopped)")
	allLong := fs.Bool("all", false, "Show all containers (running + stopped)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing ps options: %v\n", err)
		os.Exit(1)
	}
	showAll := *all || *allLong

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
