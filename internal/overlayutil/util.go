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
	if err := os.MkdirAll(work, 0755); err != nil {
		return err
	}

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
	defer func() { _ = f.Close() }()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	root, err := filepath.Abs(rootfsDir)
	if err != nil {
		return fmt.Errorf("resolve rootfs: %w", err)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("create rootfs: %w", err)
	}

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path, err := safeTarPath(root, hdr.Name)
		if err != nil {
			return fmt.Errorf("unsafe tar path %q: %w", hdr.Name, err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureNoSymlinkAncestors(root, path, true); err != nil {
				return fmt.Errorf("directory %q: %w", hdr.Name, err)
			}
			mode := os.FileMode(hdr.Mode) & 07777
			if err := os.MkdirAll(path, mode); err != nil {
				return err
			}
			if err := os.Chmod(path, mode); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := ensureNoSymlinkAncestors(root, path, true); err != nil {
				return fmt.Errorf("file %q: %w", hdr.Name, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&07777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Chmod(os.FileMode(hdr.Mode) & 07777); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}

		case tar.TypeSymlink:
			if err := ensureNoSymlinkAncestors(root, path, false); err != nil {
				return fmt.Errorf("symlink %q: %w", hdr.Name, err)
			}
			if err := validateSymlinkTarget(root, path, hdr.Linkname); err != nil {
				return fmt.Errorf("symlink %q -> %q: %w", hdr.Name, hdr.Linkname, err)
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}

		case tar.TypeLink:
			if err := ensureNoSymlinkAncestors(root, path, false); err != nil {
				return fmt.Errorf("hardlink %q: %w", hdr.Name, err)
			}
			source, err := safeTarPath(root, hdr.Linkname)
			if err != nil {
				return fmt.Errorf("hardlink target %q: %w", hdr.Linkname, err)
			}
			if err := ensureNoSymlinkAncestors(root, source, true); err != nil {
				return fmt.Errorf("hardlink target %q: %w", hdr.Linkname, err)
			}
			info, err := os.Lstat(source)
			if err != nil {
				return fmt.Errorf("hardlink target %q: %w", hdr.Linkname, err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("hardlink target %q is a symlink", hdr.Linkname)
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Link(source, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func safeTarPath(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("NUL byte in path")
	}
	converted := filepath.FromSlash(name)
	if filepath.IsAbs(converted) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("absolute path")
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("parent traversal")
		}
	}
	clean := filepath.Clean(converted)
	if clean == "." {
		return root, nil
	}
	if clean == string(filepath.Separator) {
		return "", fmt.Errorf("invalid root entry")
	}
	path := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes rootfs")
	}
	return path, nil
}

func ensureNoSymlinkAncestors(root, path string, includeTarget bool) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes rootfs")
	}
	parts := []string{}
	if rel != "." {
		parts = strings.Split(rel, string(filepath.Separator))
	}
	limit := len(parts)
	if !includeTarget && limit > 0 {
		limit--
	}
	current := root
	for i, part := range parts {
		current = filepath.Join(current, part)
		if i >= limit {
			break
		}
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			evaluated, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return fmt.Errorf("evaluate symlink %q: %w", current, evalErr)
			}
			evaluatedRel, relErr := filepath.Rel(root, evaluated)
			if relErr != nil || evaluatedRel == ".." || strings.HasPrefix(evaluatedRel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("component %q escapes rootfs", current)
			}
		}
	}
	return nil
}

func validateSymlinkTarget(root, linkPath, target string) error {
	if target == "" || strings.IndexByte(target, 0) >= 0 {
		return fmt.Errorf("invalid target")
	}
	if strings.HasPrefix(target, "\\") {
		return fmt.Errorf("absolute target")
	}
	converted := filepath.FromSlash(target)
	absolute := strings.HasPrefix(target, "/")
	if !absolute && filepath.IsAbs(converted) {
		return fmt.Errorf("absolute target")
	}
	if absolute {
		// OCI image symlinks are resolved inside the container rootfs. Keep
		// the leading slash in the extracted link, but validate its target
		// against rootfs rather than the host filesystem.
		converted = filepath.FromSlash(strings.TrimLeft(target, "/"))
	}
	base := filepath.Dir(linkPath)
	if absolute {
		base = root
	}
	resolved := filepath.Join(base, converted)
	if err := ensureNoSymlinkAncestors(root, resolved, false); err != nil {
		return fmt.Errorf("target ancestor: %w", err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target escapes rootfs")
	}
	if evaluated, err := filepath.EvalSymlinks(resolved); err == nil {
		evaluatedRel, relErr := filepath.Rel(root, evaluated)
		if relErr != nil || evaluatedRel == ".." || strings.HasPrefix(evaluatedRel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("target resolves outside rootfs")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolve target: %w", err)
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
	defer func() { _ = f.Close() }()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0
	}
	return fmt.Sprintf("%x", h.Sum(nil)), size
}
