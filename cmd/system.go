//go:build linux

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"dck/internal/container"
	"dck/internal/state"
)

func System(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck system <command>")
		fmt.Println("Commands:")
		fmt.Println("  df       Show disk usage by images, containers, and volumes")
		fmt.Println("  prune    Remove unused containers and images")
		os.Exit(1)
	}

	switch args[0] {
	case "prune":
		if err := container.SystemPrune(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "df":
		systemDF()
	default:
		fmt.Printf("unknown system command: %s\n", args[0])
		fmt.Println("Usage: dck system [df|prune]")
		os.Exit(1)
	}
}

func systemDF() {
	type category struct {
		name  string
		path  string
		count int
	}
	root := state.DataDir()
	categories := []category{
		{name: "Images", path: state.ImagesDir()},
		{name: "Containers", path: state.ContainersDir()},
		{name: "Overlay", path: state.OverlayDir()},
		{name: "Volumes", path: state.VolumesDir()},
		{name: "Logs", path: state.LogsDir()},
		{name: "Backups", path: filepath.Join(root, "backups")},
		{name: "Cache", path: state.CacheDir()},
	}

	fmt.Println("TYPE            TOTAL     SIZE      PATH")
	var totalSize int64
	for _, cat := range categories {
		size, count := dirSizeAndCount(cat.path)
		totalSize += size
		label := cat.name
		if count > 0 {
			label = fmt.Sprintf("%s (%d)", cat.name, count)
		}
		fmt.Printf("%-16s%s  %-10s %s\n", label, humanSize(size), fmt.Sprintf("%d items", count), cat.path)
	}
	fmt.Printf("\nTotal disk usage: %s\n", humanSize(totalSize))
	if root != "" {
		fmt.Printf("Data directory:   %s\n", root)
	}
}

func dirSizeAndCount(path string) (int64, int) {
	var totalSize int64
	var count int
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			count++
		}
		return nil
	})
	return totalSize, count
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
