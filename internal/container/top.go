//go:build linux

package container

import (
	"fmt"
	"os/exec"
	"strconv"
)

func (c *Container) Top() error {
	out, err := c.TopString("aux")
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func (c *Container) TopString(psArgs string) (string, error) {
	if c.Status != Running {
		return "", fmt.Errorf("container %s is not running", c.ID)
	}

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-p",
		"--",
		"ps", psArgs,
	}

	cmd := exec.Command("nsenter", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
