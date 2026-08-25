//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Rm(args []string) {
	fs := flag.NewFlagSet("rm", flag.ExitOnError)
	force := fs.Bool("f", false, "Force remove")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing rm options: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: cardinal rm [-f] <container> [container...]")
		os.Exit(1)
	}

	exitCode := 0
	for _, name := range fs.Args() {
		c, err := container.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			continue
		}
		if err := c.Remove(*force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
			continue
		}
		fmt.Println(shortID(c.ID))
	}
	os.Exit(exitCode)
}
