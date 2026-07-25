//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dck/internal/log"
	"dck/internal/overlayutil"
)

func mountOverlay(lower, upper, work, merged string) error {
	if IsRootless() {
		return MountRootlessOverlay(lower, upper, work, merged)
	}

	if err := overlayutil.MountOverlay(lower, upper, work, merged); err != nil {
		return err
	}

	return nil
}

func unmountOverlay(merged string) {
	overlayutil.UnmountOverlay(merged)
}

func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), " "+path+" ")
}

func isOverlayMounted(merged string) bool {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), " "+merged+" ")
}

func SetupOverlay(rootfs, upper, work, merged string) error {
	for _, d := range []string{upper, work, merged} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return mountOverlay(rootfs, upper, work, merged)
}

func SetupDiskLimit(overlayBase, id string, limitBytes int64) error {
	if limitBytes <= 0 {
		return nil
	}
	imgPath := filepath.Join(overlayBase, id, "disk.img")
	mnt := filepath.Join(overlayBase, id, "data")
	_ = os.MkdirAll(filepath.Dir(imgPath), 0755)

	// Create disk image if it doesn't exist
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		f, err := os.Create(imgPath)
		if err != nil {
			return fmt.Errorf("create disk image: %w", err)
		}
		if err := f.Truncate(limitBytes); err != nil {
			f.Close()
			return fmt.Errorf("truncate disk image: %w", err)
		}
		f.Close()
		if out, err := exec.Command("mkfs.ext4", "-F", imgPath).CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}

	// Mount disk image to data dir
	if !isMounted(mnt) {
		os.MkdirAll(mnt, 0755)
		if out, err := exec.Command("mount", "-o", "loop", imgPath, mnt).CombinedOutput(); err != nil {
			return fmt.Errorf("mount disk: %s: %w", strings.TrimSpace(string(out)), err)
		}
		os.MkdirAll(filepath.Join(mnt, "upper"), 0755)
		os.MkdirAll(filepath.Join(mnt, "work"), 0755)
	}
	return nil
}

func TeardownDiskLimit(overlayBase, id string) {
	mnt := filepath.Join(overlayBase, id, "data")
	if isMounted(mnt) {
		if err := exec.Command("umount", mnt).Run(); err != nil {
			log.Warn("umount %s: %v", mnt, err)
		}
	}
}
