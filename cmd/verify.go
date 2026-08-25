//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/image"
)

func Verify(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal verify <image>[:tag]")
		os.Exit(1)
	}
	if err := image.VerifyReference(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Image verification failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Image verified: %s\n", args[0])
}
