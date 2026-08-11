//go:build linux

package container

import (
	"os"
	"os/exec"
	"strconv"
)

func (c *Container) Exec(cmd []string) error {
	return c.ExecOpts(cmd, true, false)
}

func (c *Container) ExecOpts(cmd []string, interactive, tty bool) error {
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

	if interactive {
		ecmd.Stdin = os.Stdin
		ecmd.Stdout = os.Stdout
		ecmd.Stderr = os.Stderr
	} else {
		ecmd.Stdin = nil
		ecmd.Stdout = os.Stdout
		ecmd.Stderr = os.Stderr
	}

	_ = tty

	return ecmd.Run()
}
