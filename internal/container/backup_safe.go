//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SafeBackupPath(base, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("unsafe backup path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup path escapes destination: %q", name)
	}
	return filepath.Join(base, clean), nil
}

func RejectBackupSymlinkAncestors(base, target string) error {
	if info, err := os.Lstat(base); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup destination is a symlink: %q", base)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("backup target escapes destination: %q", target)
	}
	current := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup target contains symlink: %q", current)
		}
	}
	return nil
}
