//go:build linux

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"cardinal/internal/container"
	"cardinal/internal/state"
)

const (
	consoleBufSize = 65536
	consoleTailMax = 65536
)

func ConsoleServe(args []string) {
	if len(args) < 1 {
		exitFunc(1)
	}

	id := args[0]
	logPath := state.LogPath(id)

	logFile, err := container.OpenFreshLogFile(logPath, 0644)
	if err != nil {
		exitFunc(1)
		return
	}
	defer func() { _ = logFile.Close() }()

	stdinW := os.NewFile(3, "stdinW")
	stdoutR := os.NewFile(4, "stdoutR")
	if stdinW == nil || stdoutR == nil {
		exitFunc(1)
		return
	}
	defer func() { _ = stdinW.Close() }()
	defer func() { _ = stdoutR.Close() }()

	sockPath := state.ConsolePath(id)
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		exitFunc(1)
	}
	defer os.Remove(sockPath)
	defer listener.Close()

	var mu sync.Mutex
	clients := make(map[net.Conn]struct{})

	// logMu serializes log file writes: the stdout pump and every stdin
	// tee below share this fd (opened without O_APPEND), so unlocked
	// concurrent writes would interleave at the shared offset.
	var logMu sync.Mutex

	go func() {
		buf := make([]byte, consoleBufSize)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				logMu.Lock()
				_, _ = logFile.Write(buf[:n])
				logMu.Unlock()

				mu.Lock()
				for c := range clients {
					if err := c.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
						delete(clients, c)
						_ = c.Close()
						continue
					}
					if _, err := c.Write(buf[:n]); err != nil {
						delete(clients, c)
						_ = c.Close()
						continue
					}
					_ = c.SetWriteDeadline(time.Time{})
				}
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}

		// Send log tail before adding to broadcast list (no lock needed).
		// When seeking into the middle of the file, skip to the next line
		// boundary first: otherwise the new client starts with half a line,
		// which the panel cannot dedupe against its replay guard and which
		// breaks command/answer adjacency in the view.
		if fi, err := os.Stat(logPath); err == nil {
			f, err := os.Open(logPath)
			if err == nil {
				offset := fi.Size() - consoleTailMax
				if offset < 0 {
					offset = 0
				}
				seeked := false
				if offset > 0 {
					if _, err := f.Seek(offset, io.SeekStart); err != nil {
						_ = f.Close()
						_ = conn.Close()
						continue
					}
					seeked = true
				}
				if seeked {
					br := bufio.NewReader(f)
					if _, err := br.ReadBytes('\n'); err != nil {
						_ = f.Close()
						_ = conn.Close()
						continue
					}
					_, _ = io.Copy(conn, br)
				} else {
					_, _ = io.Copy(conn, f)
				}
				_ = f.Close()
			}
		}
		mu.Lock()
		clients[conn] = struct{}{}
		mu.Unlock()

		go func(c net.Conn) {
			_, _ = io.Copy(&consoleInputLogger{w: stdinW, mu: &logMu, log: logFile}, c)
			_ = c.Close()

			mu.Lock()
			delete(clients, c)
			mu.Unlock()
		}(conn)
	}
}

// consoleInputLogger tees one attach client's stdin into the container log,
// prefixing complete lines with "> ". The game log only carries output, so
// without this every reconnect replay restores answers without the commands
// that produced them. Bytes reach the container unchanged and immediately;
// only a copy is buffered for line assembly. Unbounded lines (binary paste
// without newlines) are dropped past 1 MiB rather than buffered forever.
type consoleInputLogger struct {
	w   io.Writer
	mu  *sync.Mutex
	log *os.File
	buf []byte
}

func (e *consoleInputLogger) Write(p []byte) (int, error) {
	n, err := e.w.Write(p)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buf = append(e.buf, p[:n]...)
	for {
		i := bytes.IndexByte(e.buf, '\n')
		if i < 0 {
			break
		}
		e.logInputLine(e.buf[:i])
		e.buf = e.buf[i+1:]
	}
	if len(e.buf) > 1<<20 {
		e.buf = nil
	}
	return n, err
}

func (e *consoleInputLogger) logInputLine(raw []byte) {
	line := strings.TrimRight(string(raw), "\r")
	if strings.TrimSpace(line) == "" {
		return
	}
	if len(line) > 4096 {
		line = line[:4096]
	}
	_, _ = fmt.Fprintln(e.log, "> "+line)
}
