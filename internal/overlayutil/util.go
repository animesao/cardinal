package overlayutil

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func MountOverlay(lower, upper, work, merged string) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	_ = os.RemoveAll(work)
	os.MkdirAll(work, 0755)

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := tryMount(merged, opts); err != nil {
		if err2 := tryMount(merged, opts+",redirect_dir=off,userxattr"); err2 != nil {
			return err
		}
	}
	return nil
}

func tryMount(merged, opts string) error {
	var stderr strings.Builder
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, merged)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mount overlay: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func UnmountOverlay(merged string) {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := os.Stat(merged); err != nil {
		return
	}
	if err := exec.Command("umount", merged).Run(); err != nil {
		_ = exec.Command("umount", "-l", merged).Run()
	}
}

func ExtractLayer(cachePath, rootfsDir string) error {
	f, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(rootfsDir, hdr.Name)
		if !strings.HasPrefix(path, filepath.Clean(rootfsDir)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, os.FileMode(hdr.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(path), 0755)
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			_ = os.Remove(path)
			_ = os.Symlink(hdr.Linkname, path)
		case tar.TypeLink:
			_ = os.Remove(path)
			_ = os.Link(filepath.Join(rootfsDir, hdr.Linkname), path)
		}
	}
	return nil
}

func ShortDigest(d string) string {
	if len(d) > 19 {
		return d[:19]
	}
	return d
}

func HashFile(path string) (string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0
	}
	return fmt.Sprintf("%x", h.Sum(nil)), size
}
