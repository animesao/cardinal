//go:build linux

package cmd

import (
	"bufio"
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

	conn, err := net.Dial("unix", state.ConsolePath(c.ID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "console: %v\n", err)
		exitFunc(1)
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

	go forwardStdin(conn, os.Stdin)
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

// forwardStdin pipes user input to the console socket. The container's
// stdin is a pipe, not a PTY, so nothing echoes typed lines back — without
// a local echo the operator sees responses but never what they typed.
// When stdin is a terminal the kernel already echoes keystrokes, so we only
// echo for piped/redirected input (where there would otherwise be silence).
// Lines go over the socket unchanged; only the local display copy is added.
func forwardStdin(conn io.Writer, stdin *os.File) {
	if fi, err := stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		_, _ = io.Copy(conn, stdin)
		return
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(os.Stdout, line)
		fmt.Fprintln(conn, line)
	}
}
