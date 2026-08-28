//go:build linux

package container

import (
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/creack/pty"
)

func (c *Container) Exec(cmd []string) error {
	return c.ExecOpts(cmd, true, false)
}

func (c *Container) ExecOpts(cmd []string, interactive, tty bool) error {
	return c.ExecOptsIO(cmd, interactive, tty, os.Stdin, os.Stdout, os.Stderr)
}

// ExecOptsIO runs a command in the container with caller-supplied I/O.
// Pass nil for any stream to leave it disconnected.
// When interactive=true and tty=true, a pseudo-terminal (PTY) is allocated
// so the shell gets proper echo, prompts, and line editing.
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

	// When interactive + tty: allocate a PTY so the shell gets a real
	// terminal with echo, prompts, and signal handling.
	if interactive && tty {
		ptmx, err := pty.Start(ecmd)
		if err != nil {
			return err
		}
		defer ptmx.Close()

		// Pipe caller I/O ↔ PTY
		done := make(chan struct{}, 2)
		if stdin != nil {
			go func() { io.Copy(ptmx, stdin); done <- struct{}{} }()
		} else {
			done <- struct{}{}
		}
		if stdout != nil {
			go func() { io.Copy(stdout, ptmx); done <- struct{}{} }()
		} else {
			done <- struct{}{}
		}

		<-done
		<-done
		return ecmd.Wait()
	}

	// Non-TTY path (pipe-based I/O)
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

	return ecmd.Run()
}
