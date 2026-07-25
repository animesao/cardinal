//go:build linux

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/container"
	"dck/internal/image"
	"dck/internal/state"
)

func Commit(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: dck commit <container> <image>[:<tag>]")
		os.Exit(1)
	}

	ref := args[1]
	name := ref
	tag := "latest"
	if i := strings.LastIndex(ref, ":"); i > 0 {
		tag = ref[i+1:]
		name = ref[:i]
	}
	if !strings.Contains(name, "/") {
		name = "library/" + name
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var rootfsDir string
	if c.Status == container.Running {
		_, _, merged := c.OverlayDirs()
		rootfsDir = merged
	} else {
		_, _, merged := c.OverlayDirs()
		if _, err := os.Stat(merged); err == nil {
			rootfsDir = merged
		} else {
			rootfsDir = state.ImageRootfsDir(c.ImageName, c.ImageTag)
		}
	}

	fmt.Printf("Committing %s to %s:%s...\n", c.ID[:12], name, tag)

	img, err := image.CommitContainer(rootfsDir, name, tag, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	shortName := filepath.Base(img.Name)
	digest := img.Digest
	if len(digest) > 19 {
		digest = digest[:19]
	}
	fmt.Printf("Created image %s:%s (%s)\n", shortName, img.Tag, digest)
}
