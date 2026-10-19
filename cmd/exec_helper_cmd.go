//go:build linux

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cardinal/internal/container"
)

// execHelperUsage печатает подсказку по usage.
func execHelperUsage() {
	fmt.Fprintln(os.Stderr, "Usage: cardinal exec-helper [--profile <path>] -- <command> [args...]")
}

// newExecHelperCommand — внутренняя команда `cardinal exec-helper`.
//
// Запускается ВНУТРИ контейнера (через bind-mount бинарника cardinal) и
// применяет к процессу capability-набор, пользователя, no_new_privs и
// seccomp-фильтр контейнера, после чего делает exec целевой команды.
//
// Вручную её вызывать не нужно: `cardinal exec` делает это сам через nsenter.
// Команда скрыта из help, но её реализация живёт в internal/container, чтобы
// политика безопасности exec'а и контейнерного init'а не расходилась.
//
// Флаги разбираются вручную (DisableFlagParsing), потому что аргументы
// целевой команды должны попасть сюда дословно: extractRemoteFlags вырезал
// бы из них --host/--token.
func newExecHelperCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "exec-helper",
		Short:              "Run a command inside a container with the container's security profile (internal)",
		Hidden:             true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				execHelperUsage()
				exitFunc(2)
				return
			}
			exitFunc(container.ExecHelperMain(args))
		},
	}
}
