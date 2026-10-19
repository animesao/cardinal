//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cardinal/internal/overlayutil"
	"cardinal/internal/state"
)

// HelperMountDirect performs the privileged overlay mount in-process.
// It must run as root; the unprivileged CLI never calls it directly,
// it re-execs itself via sudo (see MountViaHelper).
func HelperMountDirect(lower, upper, work, merged string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("helper-mount must run as root (euid=0), got euid=%d: rerun via `sudo cardinal helper-mount ...`", os.Geteuid())
	}
	// Пути монтирования приходят из аргументов командной строки, а сам
	// helper запускается через sudo. Проверяем, что все они лежат внутри
	// каталога состояния cardinal: иначе через sudo-правило вида
	// «cardinal ALL=(root) NOPASSWD: cardinal helper-mount *» можно было
	// смонтировать произвольный overlay (в том числе поверх /etc) с
	// правами root.
	root := state.DataDir()
	for _, p := range []string{lower, upper, work, merged} {
		if err := requireInsideStateDir(root, p); err != nil {
			return err
		}
	}
	for _, d := range []string{upper, work, merged} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	if err := overlayutil.MountOverlay(lower, upper, work, merged); err != nil {
		return fmt.Errorf("helper mount overlay: %w", err)
	}
	return nil
}

// requireInsideStateDir проверяет, что путь лежит внутри каталога
// состояния cardinal.
func requireInsideStateDir(root, p string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside the cardinal state directory", p)
	}
	return nil
}

// HelperAvailable reports whether the E path can work: sudo exists.
func HelperAvailable() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// MountViaHelper re-execs the current binary as root via passwordless sudo:
// sudo -n <cardinal> helper-mount <lower> <upper> <work> <merged>.
// The helper-mount cobra command must stay hidden and stable.
func MountViaHelper(lower, upper, work, merged string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve cardinal binary for helper: %w", err)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo not found in $PATH, helper unavailable")
	}
	cmd := exec.Command("sudo", "-n", exe, "helper-mount", lower, upper, work, merged)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "no output (passwordless sudo probably not configured)"
		}
		return fmt.Errorf("sudo helper-mount: %s: %w (allow with `sudo visudo`: `%s ALL=(root) NOPASSWD: %s helper-mount *`)", msg, err, os.Getenv("USER"), exe)
	}
	return nil
}
