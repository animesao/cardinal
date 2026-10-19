//go:build linux

package container

// ExecHelper — процесс-лаунчер, который выполняется ВНУТРИ контейнера перед
// запуском пользовательской команды.
//
// Зачем он нужен: namespace'ы контейнера (mount/pid/ipc/net/uts) входят
// только через setns, а setns требует CAP_SYS_ADMIN. Значит, ограничить
// capability-набор можно только ПОСЛЕ входа в namespace'ы — то есть в
// отдельном процессе, который затем делает exec целевой команды.
//
// Раньше этого шага не было: `cardinal exec` запускал nsenter с полным
// набором capability демона, uid 0 и без seccomp. Внутри контейнера этого
// хватало, чтобы выйти наружу:
//
//	mknod /tmp/x b 8 0 && mount /tmp/x /mnt && cat /mnt/etc/shadow
//
// (CAP_MKNOD создаёт ноду блочного устройства, CAP_SYS_ADMIN в начальном user
// namespace позволяет его смонтировать — device-cgroup в cardinal нет).
//
// Порядок здесь повторяет контейнерный init-процесс: capability -> uid ->
// no_new_privs -> seccomp -> exec. Seccomp ставится последним, перед exec:
// фильтр переживает execve, а mount(2) в deny-list'е и не может понадобиться
// после его установки.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"cardinal/internal/log"
)

// ExecHelperMain — точка входа helper'а. Возвращает код выхода.
//
// Использование: cardinal exec-helper [--profile <path>] -- <команда> [арг...]
func ExecHelperMain(args []string) int {
	profilePath := ExecProfilePath
	argv := args

	// Разбираем наш флаг --profile (он же используется в тестах).
	for len(argv) > 0 {
		if argv[0] == "--profile" {
			if len(argv) < 2 {
				fmt.Fprintln(os.Stderr, "exec-helper: --profile requires an argument")
				return 2
			}
			profilePath = argv[1]
			argv = argv[2:]
			continue
		}
		if argv[0] == "--" {
			argv = argv[1:]
			break
		}
		break
	}

	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "exec-helper: no command given")
		return 2
	}

	profile, err := loadExecProfile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exec-helper: %v\n", err)
		return 1
	}

	if err := applyExecProfile(profile); err != nil {
		fmt.Fprintf(os.Stderr, "exec-helper: %v\n", err)
		return 1
	}

	env := execEnv(profile)

	// syscall.Exec не делает PATH-lookup (в отличие от exec.Command и
	// nsenter, которые запускали целевую команду раньше). Короткие имена
	// вроде "sh" надо найти сами, иначе console/exec сломались бы.
	bin, err := resolveCommand(argv[0], env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exec-helper: %v\n", err)
		return 127
	}

	if err := syscall.Exec(bin, argv, env); err != nil {
		fmt.Fprintf(os.Stderr, "exec-helper: exec %s: %v\n", bin, err)
		return 127
	}
	return 0
}

// resolveCommand ищет команду в PATH, если путь не содержит разделителя.
func resolveCommand(name string, env []string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name, nil
	}
	path := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			path = kv[5:]
			break
		}
	}
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	for _, dir := range strings.Split(path, ":") {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("command %q not found in PATH", name)
}

// applyExecProfile применяет к текущему процессу политику безопасности
// контейнера. Порядок важен и повторяет init-процесс.
func applyExecProfile(p *ExecProfile) error {
	if err := applyCapabilityNames(p.Capabilities); err != nil {
		return fmt.Errorf("apply capabilities: %w", err)
	}

	if p.HasUser {
		if p.GID > 0 {
			if err := syscall.Setgid(p.GID); err != nil {
				return fmt.Errorf("setgid: %w", err)
			}
		}
		if err := syscall.Setuid(p.UID); err != nil {
			return fmt.Errorf("setuid: %w", err)
		}
	}

	if err := setNoNewPrivileges(); err != nil {
		return fmt.Errorf("set no-new-privileges: %w", err)
	}

	if p.WorkingDir != "" {
		if err := syscall.Chdir(p.WorkingDir); err != nil {
			// Не критично: рабочий каталог может отсутствовать в образе.
			log.Debug("exec-helper: chdir %s: %v", p.WorkingDir, err)
		}
	}

	// Seccomp — последним, непосредственно перед exec, и fail-closed.
	if p.SeccompPath != "" {
		profile, err := LoadSeccompProfile(p.SeccompPath)
		if err != nil {
			return fmt.Errorf("load seccomp profile: %w", err)
		}
		if err := ApplySeccompFilter(profile); err != nil {
			return fmt.Errorf("apply seccomp profile: %w", err)
		}
	} else if err := ApplyDefaultSeccompProfile(); err != nil {
		return fmt.Errorf("apply default seccomp profile: %w", err)
	}
	return nil
}

// applyCapabilityNames выставляет ровно указанный набор capability и убирает
// все остальные из bounding set (иначе потомок может вернуть dropped capability
// через set-capable или exec файла с file capabilities).
func applyCapabilityNames(names []string) error {
	keep := make(map[string]bool, len(names))
	for _, n := range names {
		name := strings.ToUpper(strings.TrimPrefix(n, "CAP_"))
		if _, ok := capMap[name]; !ok {
			return fmt.Errorf("unknown capability: %s", n)
		}
		keep[name] = true
	}

	var data [2]unix.CapUserData
	for name := range keep {
		value := capMap[name]
		word := value / 32
		bit := value % 32
		data[word].Effective |= 1 << bit
		data[word].Permitted |= 1 << bit
	}
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	if err := unix.Capset(&header, &data[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}

	for name, value := range capMap {
		if keep[name] {
			continue
		}
		if err := prctl(PR_CAPBSET_DROP, value, 0, 0, 0); err != nil {
			return fmt.Errorf("drop capability %s: %w", name, err)
		}
	}
	return nil
}

// execEnv собирает окружение для exec-процесса.
//
// Важно: это окружение КОНТЕЙНЕРА, а не окружение демона. Раньше exec
// наследовал os.Environ() демона, то есть внутрь контейнера попадали
// CARDINAL_TOKEN (root-эквивалентный токен API демона), CARDINAL_BACKUP_KEY
// (ключ шифрования бэкапов) и DOCKER_PASSWORD (креды реестра).
func execEnv(p *ExecProfile) []string {
	env := make([]string, 0, len(p.Env)+4)
	hasPath := false
	hasHome := false
	hasTerm := false
	for _, kv := range p.Env {
		switch {
		case strings.HasPrefix(kv, "PATH="):
			hasPath = true
		case strings.HasPrefix(kv, "HOME="):
			hasHome = true
		case strings.HasPrefix(kv, "TERM="):
			hasTerm = true
		}
		env = append(env, kv)
	}
	if !hasPath {
		env = append(env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}
	if !hasHome {
		env = append(env, "HOME=/root")
	}
	if !hasTerm {
		env = append(env, "TERM=xterm")
	}
	return env
}

// ExecHelperPathForTest — путь к helper'у для тестов.
func ExecHelperPathForTest() string { return ExecHelperPath }
