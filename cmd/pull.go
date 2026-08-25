//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/image"
)

func Pull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	platform := fs.String("platform", "", "Platform (e.g. linux/amd64, linux/arm64)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing pull options: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: cardinal pull [--platform linux/amd64] <image>[:<tag>]")
		os.Exit(1)
	}

	ref := fs.Arg(0)

	var platformOS, platformArch string
	if *platform != "" {
		parts := splitPlatform(*platform)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: invalid platform format %q (expected os/arch, e.g. linux/amd64)\n", *platform)
			os.Exit(1)
		}
		platformOS = parts[0]
		platformArch = parts[1]
	}

	_, err := image.PullWithPlatform(ref, platformOS, platformArch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func splitPlatform(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
