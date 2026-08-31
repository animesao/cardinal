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

	"cardinal/internal/container"
	"cardinal/internal/state"
)

func Attach(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal attach <container>")
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

	var closeOnce sync.Once
	closeConn := func() {
		closeOnce.Do(func() { _ = conn.Close() })
	}
	defer closeConn()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Closing either direction must stop the other direction too. Without this,
	// attach could remain blocked forever in io.Copy(os.Stdin -> socket) after
	// the container's console socket had already closed.
	done := make(chan struct{}, 2)
	copyStream := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		closeConn()
		done <- struct{}{}
	}

	go copyStream(conn, os.Stdin)
	go copyStream(os.Stdout, conn)
	go func() {
		select {
		case <-sigCh:
			closeConn()
		case <-done:
		}
	}()

	<-done
	closeConn()
}
