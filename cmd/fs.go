//go:build linux

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/container"
)

func Fs(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  dck fs ls <container> [path]")
		fmt.Println("  dck fs cat <container> <path>")
		fmt.Println("  dck fs tree <container> [path]")
		fmt.Println("  dck fs find <container> [path] [--name <pattern>] [--grep <text>] [--type f|d] [--max-depth <n>]")
		fmt.Println("  dck fs find [--name <pattern>] [--grep <text>] [--type f|d] [--max-depth <n>]  (search all containers)")
		os.Exit(1)
	}

	sub := args[0]

	// If second arg is a flag, search all containers (no container specified)
	if len(args) > 1 && strings.HasPrefix(args[1], "--") {
		if sub != "find" {
			fmt.Fprintf(os.Stderr, "Error: container required for 'dck fs %s'\n", sub)
			os.Exit(1)
		}
		containers, err := container.List(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(containers) == 0 {
			fmt.Fprintln(os.Stderr, "No containers found")
			os.Exit(1)
		}
		for _, c := range containers {
			_, _, merged := c.OverlayDirs()
			if _, err := os.Stat(merged); err != nil {
				continue
			}
			label := shortID(c.ID)
			if c.Name != "" {
				label = shortID(c.ID) + "  " + c.Name
			}
			fsFind(merged, label, args[1:])
		}
		return
	}

	id := args[1]

	c, err := container.Load(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	_, _, merged := c.OverlayDirs()

	switch sub {
	case "ls":
		fsLs(merged, args[2:])
	case "cat":
		fsCat(merged, args[2:])
	case "tree":
		fsTree(merged, args[2:])
	case "find":
		label := shortID(c.ID)
		if c.Name != "" {
			label = shortID(c.ID) + "  " + c.Name
		}
		fsFind(merged, label, args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown fs command: %s\n", sub)
		os.Exit(1)
	}
}

func fsLs(merged string, args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	fullPath := filepath.Join(merged, path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		mode := info.Mode()
		size := info.Size()
		modTime := info.ModTime().Format("Jan _2 15:04")
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		fmt.Printf("%s %8d %s %s\n", mode, size, modTime, name)
	}
}

func fsCat(merged string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: path required")
		os.Exit(1)
	}
	fullPath := filepath.Join(merged, args[0])

	data, err := os.ReadFile(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}

func fsTree(merged string, args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	fullPath := filepath.Join(merged, path)

	err := filepath.Walk(fullPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(fullPath, p)
		if rel == "." {
			fmt.Println(".")
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		prefix := ""
		if depth > 0 {
			prefix = strings.Repeat("│   ", depth-1) + "├── "
		}
		name := fi.Name()
		if fi.IsDir() {
			name += "/"
		}
		fmt.Printf("%s%s\n", prefix, name)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type findOpts struct {
	name     string
	grep     string
	typ      string
	maxDepth int
}

func fsFind(merged, label string, args []string) {
	opts := findOpts{maxDepth: -1}
	path := "."

	// Parse flags
	i := 0
	for i < len(args) {
		switch {
		case args[i] == "--name" && i+1 < len(args):
			opts.name = args[i+1]
			i += 2
		case args[i] == "--grep" && i+1 < len(args):
			opts.grep = args[i+1]
			i += 2
		case args[i] == "--type" && i+1 < len(args):
			opts.typ = args[i+1]
			i += 2
		case args[i] == "--max-depth" && i+1 < len(args):
			fmt.Sscanf(args[i+1], "%d", &opts.maxDepth)
			i += 2
		default:
			if !strings.HasPrefix(args[i], "--") && i == 0 {
				path = args[i]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	fullPath := filepath.Join(merged, path)

	err := filepath.Walk(fullPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(fullPath, p)
		if rel == "." {
			return nil
		}

		depth := strings.Count(rel, string(os.PathSeparator))
		if opts.maxDepth >= 0 && depth > opts.maxDepth {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by type
		switch opts.typ {
		case "f":
			if fi.IsDir() {
				return nil
			}
		case "d":
			if !fi.IsDir() {
				return nil
			}
		}

		// Filter by name pattern (substring match)
		if opts.name != "" {
			if !strings.Contains(fi.Name(), opts.name) {
				return nil
			}
		}

		// Filter by grep content
		if opts.grep != "" {
			if fi.IsDir() {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if !strings.Contains(string(data), opts.grep) {
				return nil
			}
		}

		containerRel, _ := filepath.Rel(merged, p)
		containerPath := "/" + strings.ReplaceAll(containerRel, "\\", "/")
		fmt.Printf("%s  %s\n", label, containerPath)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
