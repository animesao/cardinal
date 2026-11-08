//go:build linux

package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (c *Container) CopyFromContainer(srcPath string, w io.Writer) error {
	if err := c.validateNamespaceTarget(); err != nil {
		return err
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p", "-i", "-r",
		"--",
	}
	if err := c.ensureExecHelper(); err == nil {
		args = append(args, ExecHelperPath, "exec-helper", "--", "tar", "-cf", "-", "-C", "/", strings.TrimPrefix(srcPath, "/"))
	} else {
		args = append(args, "tar", "-cf", "-", "-C", "/", strings.TrimPrefix(srcPath, "/"))
	}

	cmd := exec.Command("nsenter", args...)
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"TERM=xterm",
	}
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c *Container) CopyToContainer(dstPath string, r io.Reader) error {
	if err := c.validateNamespaceTarget(); err != nil {
		return err
	}

	// Determine the extraction directory.
	// If dstPath looks like a file path (doesn't end with /), extract to its
	// parent directory so that tar creates/overwrites the file correctly.
	// This avoids the "mkdir: can't create directory: File exists" error when
	// the target is an existing file rather than a directory.
	extractDir := dstPath
	if !strings.HasSuffix(dstPath, "/") {
		ext := filepath.Ext(dstPath)
		// If the path has a file extension OR contains a dot in the last component,
		// treat it as a file and extract to parent dir.
		if ext != "" || strings.Contains(filepath.Base(dstPath), ".") {
			extractDir = filepath.Dir(dstPath)
		}
	}
	if extractDir == "" || extractDir == "." {
		extractDir = "/"
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p", "-r",
		"--",
	}
	shCmd := fmt.Sprintf("mkdir -p -- %s && tar -x -C %s", escapePath(extractDir), escapePath(extractDir))
	if err := c.ensureExecHelper(); err == nil {
		args = append(args, ExecHelperPath, "exec-helper", "--", "sh", "-c", shCmd)
	} else {
		args = append(args, "sh", "-c", shCmd)
	}

	cmd := exec.Command("nsenter", args...)
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"TERM=xterm",
	}
	cmd.Stdin = r
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func escapePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
}
