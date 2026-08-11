//go:build linux

package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"dck/internal/container"
	"dck/internal/state"
)

func Attach(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck attach <container>")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if c.Status != container.Running {
		fmt.Fprintf(os.Stderr, "Container %s is not running\n", args[0])
		os.Exit(1)
	}

	conn, err := net.Dial("unix", state.ConsolePath(c.ID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "console: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		wg.Done()
	}()

	go func() {
		_, _ = io.Copy(os.Stdout, conn)
		wg.Done()
	}()

	go func() {
		<-sigCh
		_ = conn.Close()
	}()

	wg.Wait()
}
