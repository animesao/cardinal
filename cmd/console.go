//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/container"
)

func Console(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal console <container>")
		exitFunc(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}

	if c.Status != container.Running {
		fmt.Fprintf(os.Stderr, "Container %s is not running\n", args[0])
		exitFunc(1)
	}

	err = c.ExecOpts([]string{"sh", "-c", "exec bash 2>/dev/null || exec sh"}, true, true)
	if err != nil {
		err = c.ExecOpts([]string{"/bin/sh"}, true, true)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
