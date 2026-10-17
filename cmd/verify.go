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
		exitFunc(1)
	}
	// Local consistency only: config + layer hashes vs the stored manifest.
	// This does NOT prove the image came from a trusted registry or that
	// the tag was not moved — there is no signature/cosign verification
	// (yet). A malicious but self-consistent image passes this check.
	if err := image.VerifyReference(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Image verification failed: %v\n", err)
		exitFunc(1)
	}
	fmt.Printf("Image locally consistent: %s (no registry/signature check)\n", args[0])
}
