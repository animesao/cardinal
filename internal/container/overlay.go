//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cardinal/internal/log"
	"cardinal/internal/overlayutil"
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
	if st, err := os.Stat(imgPath); os.IsNotExist(err) {
		f, err := os.Create(imgPath)
		if err != nil {
			return fmt.Errorf("create disk image: %w", err)
		}
		if err := f.Truncate(limitBytes); err != nil {
			_ = f.Close()
			return fmt.Errorf("truncate disk image: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close disk image: %w", err)
		}
		if out, err := exec.Command("mkfs.ext4", "-F", imgPath).CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4: %s: %w", strings.TrimSpace(string(out)), err)
		}
	} else if err == nil && limitBytes > 0 && st.Size() != limitBytes {
		// Limit changed for an existing image. The image (and its filesystem)
		// is not mounted yet at this point, so we can resize it in place — the
		// data inside is fully preserved because it's the same <id>/disk.img.
		orig := st.Size()
		if limitBytes > orig {
			// Grow: enlarge the backing file, then the filesystem.
			f, ferr := os.OpenFile(imgPath, os.O_WRONLY, 0)
			if ferr == nil {
				ferr = f.Truncate(limitBytes)
				_ = f.Close()
			}
			if ferr != nil {
				return fmt.Errorf("grow disk image file: %w", ferr)
			}
			if out, rerr := exec.Command("resize2fs", imgPath).CombinedOutput(); rerr != nil {
				// Undo the enlargement so the image stays consistent.
				if uf, uerr := os.OpenFile(imgPath, os.O_WRONLY, 0); uerr == nil {
					_ = uf.Truncate(orig)
					_ = uf.Close()
				}
				return fmt.Errorf("resize disk image (grow): %s: %w", strings.TrimSpace(string(out)), rerr)
			}
		} else {
			// Shrink: resize the filesystem to its minimum first, then the file.
			if _, rerr := exec.Command("resize2fs", "-M", imgPath).CombinedOutput(); rerr == nil {
				if f, ferr := os.OpenFile(imgPath, os.O_WRONLY, 0); ferr == nil {
					_ = f.Truncate(limitBytes)
					_ = f.Close()
				}
			} else {
				// Not enough free space to shrink safely — keep the current
				// image size rather than risk data loss.
				log.Warn("skip shrinking disk image %s (needs free space): %v", imgPath, rerr)
			}
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
