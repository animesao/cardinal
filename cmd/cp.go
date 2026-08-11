//go:build linux

package cmd

import (
	"fmt"
	"os"
	"strings"

	"dck/internal/container"
)

func Cp(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: dck cp <src> <dst>")
		fmt.Println("  dck cp <container>:<path> <host-path>")
		fmt.Println("  dck cp <host-path> <container>:<path>")
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

		inFile, err := os.Open(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = inFile.Close() }()

		if err := c.CopyToContainer(dstPath, inFile); err != nil {
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
