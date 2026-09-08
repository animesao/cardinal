//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/image"
)

func Export(args []string) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	output := fs.String("o", "", "Output file path")
	mustParse(fs, args, "export")

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: cardinal export <image>[:tag] [-o output.tar.gz]")
		exitFunc(1)
	}

	ref := freeArgs[0]
	if err := image.Export(ref, *output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}

func Import(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal import <file.tar.gz>")
		exitFunc(1)
	}

	for _, path := range args {
		if err := image.Import(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
