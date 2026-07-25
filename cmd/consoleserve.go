//go:build linux

package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"dck/internal/container"
	"dck/internal/state"
)

const (
	consoleBufSize = 65536
	consoleTailMax = 65536
)

func ConsoleServe(args []string) {
	if len(args) < 1 {
		os.Exit(1)
	}

	id := args[0]
	logPath := state.LogPath(id)

	container.RotateLogFile(logPath)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		os.Exit(1)
	}
	defer logFile.Close()

	stdinW := os.NewFile(3, "stdinW")
	stdoutR := os.NewFile(4, "stdoutR")
	if stdinW == nil || stdoutR == nil {
		logFile.WriteString("[console-serve] failed: missing FDs\n")
		os.Exit(1)
	}
	defer stdinW.Close()
	defer stdoutR.Close()

	sockPath := state.ConsolePath(id)
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		logFile.WriteString(fmt.Sprintf("[console-serve] listen error: %v\n", err))
		os.Exit(1)
	}
	defer os.Remove(sockPath)
	defer listener.Close()

	logFile.WriteString("[console-serve] started\n")

	var mu sync.Mutex
	clients := make(map[net.Conn]struct{})

	go func() {
		buf := make([]byte, consoleBufSize)
		for {
			n, err := stdoutR.Read(buf)
			if err != nil {
				logFile.WriteString(fmt.Sprintf("[console-serve] stdout read done: %v\n", err))
				return
			}

			logFile.Write(buf[:n])

			mu.Lock()
			for c := range clients {
				if err := c.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
					delete(clients, c)
					c.Close()
					continue
				}
				if _, err := c.Write(buf[:n]); err != nil {
					delete(clients, c)
					c.Close()
				}
				c.SetWriteDeadline(time.Time{})
			}
			mu.Unlock()
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logFile.WriteString(fmt.Sprintf("[console-serve] listener done: %v\n", err))
			break
		}

		logFile.WriteString("[console-serve] client connected\n")

		// Send log tail before adding to broadcast list (no lock needed)
		if fi, err := os.Stat(logPath); err == nil {
			f, err := os.Open(logPath)
			if err == nil {
				offset := fi.Size() - consoleTailMax
				if offset < 0 {
					offset = 0
				}
				if offset > 0 {
					f.Seek(offset, io.SeekStart)
				}
				io.Copy(conn, f)
				f.Close()
			}
		}
		mu.Lock()
		clients[conn] = struct{}{}
		mu.Unlock()

		go func(c net.Conn) {
			io.Copy(stdinW, c)
			c.Close()

			mu.Lock()
			delete(clients, c)
			mu.Unlock()
		}(conn)
	}

	logFile.WriteString("[console-serve] exiting\n")
}
