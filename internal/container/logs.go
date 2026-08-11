//go:build linux

package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxLogFiles = 5

// OpenFreshLogFile opens the current log for a new run and rotates old logs.
func OpenFreshLogFile(path string, mode os.FileMode) (*os.File, error) {
	if err := rotateLogFile(path, maxLogFiles); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
}

func rotateLogFile(path string, maxFiles int) error {
	if maxFiles < 2 {
		return fmt.Errorf("max log files must be at least 2")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	for i := maxFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", path, i)
		newPath := fmt.Sprintf("%s.%d", path, i+1)
		if i == maxFiles-1 {
			_ = os.Remove(oldPath)
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, path+".1")
}

func (c *Container) Logs(follow bool, tail int) error {
	return c.LogsWithOptions(follow, tail, false, false)
}

func (c *Container) LogsWithOptions(follow bool, tail int, previous, all bool) error {
	if previous && all {
		return fmt.Errorf("--previous and --all cannot be used together")
	}
	if (all || previous) && follow {
		return fmt.Errorf("--previous and --all cannot be followed")
	}
	paths := c.logPaths(previous, all)
	if len(paths) == 0 {
		return fmt.Errorf("no logs found")
	}
	found := false
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("open log: %w", err)
		}
		if all {
			if found {
				fmt.Printf("\n--- %s ---\n", filepath.Base(path))
			}
			if _, err := io.Copy(os.Stdout, f); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
			found = true
			continue
		}
		if tail > 0 {
			if err := printTail(f, tail); err != nil {
				_ = f.Close()
				return err
			}
			if !follow {
				_ = f.Close()
				return nil
			}
			if _, err := f.Seek(0, io.SeekEnd); err != nil {
				_ = f.Close()
				return err
			}
		}
		if follow {
			err := c.followLogs(f)
			_ = f.Close()
			return err
		}
		if tail <= 0 {
			_, err = io.Copy(os.Stdout, f)
		}
		_ = f.Close()
		return err
	}
	if !found {
		return fmt.Errorf("no logs found")
	}
	return nil
}

func (c *Container) logPaths(previous, all bool) []string {
	base := c.LogFile()
	if previous {
		return []string{base + ".1"}
	}
	if !all {
		return []string{base}
	}
	paths := make([]string, 0, maxLogFiles)
	for i := maxLogFiles - 1; i >= 1; i-- {
		paths = append(paths, fmt.Sprintf("%s.%d", base, i))
	}
	paths = append(paths, base)
	return paths
}

func printTail(f *os.File, n int) error {
	const maxBuf = 4096
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	lines := make([]string, 0, n)
	offset := size
	leftover := ""

	for offset > 0 && len(lines) < n {
		readSize := int64(maxBuf)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		chunk := make([]byte, readSize)
		if _, err := io.ReadFull(f, chunk); err != nil {
			return err
		}
		data := string(chunk) + leftover
		parts := splitLines(data)
		if len(parts) > 0 {
			leftover = parts[0]
			for i := len(parts) - 1; i > 0 && len(lines) < n; i-- {
				lines = append(lines, parts[i])
			}
		}
	}
	if len(lines) < n && leftover != "" {
		lines = append(lines, leftover)
	}
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
	if _, err := r.Seek(0, io.SeekEnd); err != nil {
		return err
	}
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
