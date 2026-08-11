//go:build linux

package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func (c *Container) CopyFromContainer(srcPath string, w io.Writer) error {
	if c.Status != Running {
		return fmt.Errorf("container %s is not running", c.ID)
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p", "-i",
		"--",
		"tar", "-cf", "-", "-C", "/", strings.TrimPrefix(srcPath, "/"),
	}

	cmd := exec.Command("nsenter", args...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c *Container) CopyToContainer(dstPath string, r io.Reader) error {
	if c.Status != Running {
		return fmt.Errorf("container %s is not running", c.ID)
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p",
		"--",
		"sh", "-c", fmt.Sprintf("mkdir -p %s && tar -x -C %s", escapePath(dstPath), escapePath(dstPath)),
	}

	cmd := exec.Command("nsenter", args...)
	cmd.Stdin = r
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func escapePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
}
