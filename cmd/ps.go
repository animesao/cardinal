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
	all := fs.Bool("a", false, "Show all containers")
	fs.Parse(args)

	containers, err := container.List(*all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(containers) == 0 {
		if *all {
			fmt.Println("No containers found")
		} else {
			fmt.Println("No running containers")
		}
		return
	}

	container.PrintContainers(containers)
}
