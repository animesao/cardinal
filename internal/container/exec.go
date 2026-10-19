//go:build linux

package container

import (
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/creack/pty"
)

// ExecOpts настройки процесса exec.
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
//
// Безопасность: команда выполняется НЕ напрямую. Сначала nsenter входит в
// namespace'ы контейнера, затем внутри контейнера запускается helper
// (cardinal exec-helper), который применяет к процессу capability-набор
// контейнера, пользователя контейнера, no_new_privs и seccomp-фильтр — и
// только после этого делает exec целевой команды.
//
// Без helper'а процесс наследовал capability демона (CAP_SYS_ADMIN +
// CAP_MKNOD) и uid 0, а seccomp не ставился вовсе: этого хватало, чтобы
// выйти из контейнера через mknod + mount блочного устройства хоста.
// Подробности — в internal/container/exec_helper.go.
func (c *Container) ExecOptsIO(cmd []string, interactive, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := c.validateNamespaceTarget(); err != nil {
		return err
	}
	if len(cmd) == 0 {
		return os.ErrInvalid
	}
	// Без helper'а команда выполнилась бы вообще без ограничений —
	// поэтому падаем явно, а не «как раньше».
	if err := c.ensureExecHelper(); err != nil {
		return err
	}

	// Внутри контейнера: helper -> exec целевой команды.
	inner := append([]string{"exec-helper", "--"}, cmd...)

	args := []string{
		"-t", strconv.Itoa(c.PID),
		"-m", "-p", "-i", "-n", "-r",
		"--",
	}
	args = append(args, ExecHelperPath)
	args = append(args, inner...)

	ecmd := exec.Command("nsenter", args...)

	// Окружение демона намеренно НЕ наследуется: в нём живут
	// CARDINAL_TOKEN (root-эквивалентный токен API демона),
	// CARDINAL_BACKUP_KEY (ключ шифрования бэкапов) и DOCKER_PASSWORD
	// (креды реестра). Внутрь контейнера они попадать не должны —
	// helper подставляет окружение самого контейнера.
	ecmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"TERM=xterm",
	}

	// When interactive + tty: allocate a PTY so the shell gets a real
	// terminal with echo, prompts, and signal handling.
	if interactive && tty {
		ptmx, err := pty.Start(ecmd)
		if err != nil {
			return err
		}
		defer ptmx.Close() //nolint:errcheck

		// Pipe caller I/O <-> PTY
		done := make(chan struct{}, 2)
		if stdin != nil {
			go func() { _, _ = io.Copy(ptmx, stdin); done <- struct{}{} }()
		} else {
			done <- struct{}{}
		}
		if stdout != nil {
			go func() { _, _ = io.Copy(stdout, ptmx); done <- struct{}{} }()
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
