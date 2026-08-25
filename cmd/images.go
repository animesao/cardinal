//go:build linux

package cmd

import (
	"fmt"
	"os"
	"strings"

	"cardinal/internal/image"
)

func Images(args []string) {
	images, err := image.ListImages()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(images) == 0 {
		fmt.Println("No images found")
		return
	}

	fmt.Println("REPOSITORY\tTAG")
	for _, img := range images {
		fmt.Printf("%s\t%s\n", img.Name, img.Tag)
	}
}

func Rmi(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal rmi <image>[:<tag>]")
		os.Exit(1)
	}

	name := args[0]
	tag := "latest"
	if i := strings.LastIndex(args[0], ":"); i > 0 {
		tag = args[0][i+1:]
		name = args[0][:i]
	}
	if !strings.Contains(name, "/") && !strings.HasPrefix(name, "library/") {
		name = "library/" + name
	}

	if err := image.RemoveImage(name, tag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Removed:", args[0])
}
