//go:build linux

package container

import (
	"os"
	"os/exec"
	"strconv"
)

import (
	"io"
	"os"
	"os/exec"
	"strconv"
)

func (c *Container) Exec(cmd []string) error {
	return c.ExecOpts(cmd, true, false)
}

func (c *Container) ExecOpts(cmd []string, interactive, tty bool) error {
	return c.ExecOptsIO(cmd, interactive, tty, os.Stdin, os.Stdout, os.Stderr)
}

// ExecOptsIO runs a command in the container with caller-supplied I/O.
// Pass nil for any stream to leave it disconnected.
func (c *Container) ExecOptsIO(cmd []string, interactive, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := c.validateNamespaceTarget(); err != nil {
		return err
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p", "-i", "-n", "-r",
		"--",
	}
	args = append(args, cmd...)

	ecmd := exec.Command("nsenter", args...)

	if interactive && stdin != nil {
		ecmd.Stdin = stdin
	} else {
		ecmd.Stdin = nil
	}
	if stdout != nil {
		ecmd.Stdout = stdout
	}
	if stderr != nil {
		ecmd.Stderr = stderr
	}

	_ = tty

	return ecmd.Run()
}
