//go:build linux

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"cardinal/internal/container"
)

func Cp(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cardinal cp <src> <dst>")
		fmt.Println("  cardinal cp <container>:<path> <host-path>")
		fmt.Println("  cardinal cp <host-path> <container>:<path>")
		os.Exit(1)
	}

	src := args[0]
	dst := args[1]

	srcContainer, srcPath := parseCpRef(src)
	dstContainer, dstPath := parseCpRef(dst)

	if srcContainer != "" && dstContainer != "" {
		fmt.Println("Copying between containers is not supported")
		os.Exit(1)
	}

	if srcContainer != "" {
		c, err := container.Load(srcContainer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		outFile, err := os.Create(dst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = outFile.Close() }()

		if err := c.CopyFromContainer(srcPath, outFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		c, err := container.Load(dstContainer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var inReader io.Reader
		if src == "-" {
			// Read from stdin
			inReader = os.Stdin
		} else {
			inFile, err := os.Open(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			defer func() { _ = inFile.Close() }()
			inReader = inFile
		}

		if err := c.CopyToContainer(dstPath, inReader); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Done")
}

func parseCpRef(ref string) (containerID, path string) {
	if i := strings.Index(ref, ":"); i > 0 {
		return ref[:i], ref[i+1:]
	}
	return "", ref
}
