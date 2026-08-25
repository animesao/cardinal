//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/image"
)

func Export(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	output := fs.String("o", "", "Output file path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing export options: %v\n", err)
		os.Exit(1)
	}

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: cardinal export <image>[:tag] [-o output.tar.gz]")
		os.Exit(1)
	}

	ref := freeArgs[0]
	if err := image.Export(ref, *output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Import(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal import <file.tar.gz>")
		os.Exit(1)
	}

	for _, path := range args {
		if err := image.Import(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
