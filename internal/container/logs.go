//go:build linux

package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

const MaxLogSize = 10 * 1024 * 1024

func RotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < MaxLogSize {
		return
	}
	rotated := path + ".1"
	os.Remove(rotated)
	os.Rename(path, rotated)
}

func (c *Container) Logs(follow bool, tail int) error {
	f, err := os.Open(c.LogFile())
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	if tail > 0 {
		if err := printTail(f, tail); err != nil {
			return err
		}
		if !follow {
			return nil
		}
		// Seek to end for follow
		f.Seek(0, io.SeekEnd)
	}

	if follow {
		return c.followLogs(f)
	}

	if tail <= 0 {
		_, err = io.Copy(os.Stdout, f)
	}
	return err
}

func printTail(f *os.File, n int) error {
	const maxBuf = 4096
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()

	// Read from end, buffer chunks, count lines, then print
	lines := make([]string, 0, n)
	offset := size
	leftover := ""

	for offset > 0 && len(lines) < n {
		readSize := int64(maxBuf)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		_, err := f.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		chunk := make([]byte, readSize)
		_, err = io.ReadFull(f, chunk)
		if err != nil {
			return err
		}

		// Prepend leftover to this chunk
		data := string(chunk) + leftover
		parts := splitLines(data)
		// Last element is the partial line that continues backwards
		if len(parts) > 0 {
			leftover = parts[0]
			for i := len(parts) - 1; i > 0 && len(lines) < n; i-- {
				lines = append(lines, parts[i])
			}
		}
	}

	// If we still have room, include the leftover (first line)
	if len(lines) < n && leftover != "" {
		lines = append(lines, leftover)
	}

	// Print in order
	for i := len(lines) - 1; i >= 0; i-- {
		fmt.Println(lines[i])
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func (c *Container) followLogs(r io.ReadSeeker) error {
	r.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(r)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				c.dataMu.RLock()
				running := c.Status == Running
				c.dataMu.RUnlock()
				if !running {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		fmt.Print(line)
	}
}
