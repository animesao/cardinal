package image

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cardinal/internal/state"
)

const (
	maxImportEntries    = 100000
	maxImportEntryBytes = int64(4 << 30)
	maxImportTotalBytes = int64(16 << 30)
	maxMetadataBytes    = int64(1 << 20)
)

// Export saves an image as a .tar.gz file.
func Export(ref, outputPath string) (retErr error) {
	name, tag := parseRef(ref)
	if err := validateImageReference(name, tag); err != nil {
		return err
	}

	img := LoadFromStore(name, tag)
	if img == nil {
		return fmt.Errorf("image %s:%s not found", name, tag)
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("%s_%s.tar.gz", name, tag)
		outputPath = stringsReplace(outputPath, "/", "_")
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); retErr == nil {
			retErr = fmt.Errorf("close tar archive: %w", err)
		}
		if err := gw.Close(); retErr == nil {
			retErr = fmt.Errorf("close gzip archive: %w", err)
		}
		if err := f.Close(); retErr == nil {
			retErr = fmt.Errorf("close export: %w", err)
		}
	}()

	imageDir := state.ImageDir(name, tag)
	err = filepath.Walk(imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(imageDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = stringsReplace(relPath, "\\", "/")

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.Mode()&os.ModeSymlink != 0 {
			link, readErr := os.Readlink(path)
			if readErr != nil {
				return readErr
			}
			header.Linkname = filepath.ToSlash(link)
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			in, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			_, copyErr := io.Copy(tw, in)
			closeErr := in.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk image dir: %w", err)
	}
	fmt.Printf("Exported %s:%s -> %s (%d bytes)\n", name, tag, outputPath, fileSize(outputPath))
	return nil
}

// Import loads an image from a .tar.gz file. The archive is first extracted
// into a private staging directory; the destination is changed only after all
// paths, links, metadata, and size limits have been validated.
func Import(path string) error {
	stageRoot, err := os.MkdirTemp(state.ImagesDir(), ".import-")
	if err != nil {
		if ensureErr := state.EnsureDirs(); ensureErr != nil {
			return fmt.Errorf("create import staging directory: %w", ensureErr)
		}
		stageRoot, err = os.MkdirTemp(state.ImagesDir(), ".import-")
	}
	if err != nil {
		return fmt.Errorf("create import staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageRoot) }()
	if err := os.Chmod(stageRoot, 0700); err != nil {
		return fmt.Errorf("secure import staging directory: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	var manifestName, manifestTag string
	var totalBytes int64
	entries := 0
	metadataSeen := map[string]bool{}

	for {
		header, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("tar: %w", nextErr)
		}
		entries++
		if entries > maxImportEntries {
			return fmt.Errorf("archive contains too many entries (limit %d)", maxImportEntries)
		}
		if header.Size < 0 || header.Size > maxImportEntryBytes {
			return fmt.Errorf("archive entry %q is too large", header.Name)
		}

		target, pathErr := safeArchivePath(stageRoot, header.Name)
		if pathErr != nil {
			return fmt.Errorf("unsafe archive path %q: %w", header.Name, pathErr)
		}

		if header.Name == "image.json" || header.Name == "manifest.json" {
			if header.Typeflag != tar.TypeReg {
				return fmt.Errorf("metadata entry %q must be a regular file", header.Name)
			}
			if metadataSeen[header.Name] {
				return fmt.Errorf("duplicate metadata entry %q", header.Name)
			}
			metadataSeen[header.Name] = true
			data, readErr := readLimitedEntry(tr, maxMetadataBytes)
			if readErr != nil {
				return fmt.Errorf("read metadata %q: %w", header.Name, readErr)
			}
			if int64(len(data)) > maxImportTotalBytes-totalBytes {
				return fmt.Errorf("archive exceeds total size limit")
			}
			totalBytes += int64(len(data))
			if header.Name == "image.json" {
				var metadata Image
				if err := json.Unmarshal(data, &metadata); err == nil && metadata.Name != "" {
					manifestName, manifestTag = metadata.Name, metadata.Tag
				}
			} else {
				var cfg struct {
					RepoTags []string `json:"RepoTags"`
				}
				if err := json.Unmarshal(data, &cfg); err == nil && len(cfg.RepoTags) > 0 {
					manifestName, manifestTag = parseRef(cfg.RepoTags[0])
				}
			}
			if err := writeArchiveFile(stageRoot, target, data, header.Mode); err != nil {
				return fmt.Errorf("write metadata %q: %w", header.Name, err)
			}
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := ensureArchiveAncestors(stageRoot, target, true); err != nil {
				return fmt.Errorf("directory %q: %w", header.Name, err)
			}
			if err := os.MkdirAll(target, os.FileMode(header.Mode)&07777); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := ensureArchiveAncestors(stageRoot, target, true); err != nil {
				return fmt.Errorf("file %q: %w", header.Name, err)
			}
			if header.Size > maxImportTotalBytes-totalBytes {
				return fmt.Errorf("archive exceeds total size limit")
			}
			if err := writeArchiveStream(target, tr, header.Size, header.Mode); err != nil {
				return fmt.Errorf("write %q: %w", header.Name, err)
			}
			totalBytes += header.Size
		case tar.TypeSymlink:
			if err := ensureArchiveAncestors(stageRoot, target, false); err != nil {
				return fmt.Errorf("symlink %q: %w", header.Name, err)
			}
			if err := validateArchiveLink(stageRoot, target, header.Linkname); err != nil {
				return fmt.Errorf("symlink %q -> %q: %w", header.Name, header.Linkname, err)
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q of type %d", header.Name, header.Typeflag)
		}
	}

	if manifestName == "" {
		manifestName = "imported"
	}
	if manifestTag == "" {
		manifestTag = "latest"
	}
	manifestName, manifestTag = parseRef(manifestName + ":" + manifestTag)
	if err := validateImageReference(manifestName, manifestTag); err != nil {
		return fmt.Errorf("unsafe image metadata: %w", err)
	}

	if err := os.Chmod(stageRoot, 0700); err != nil {
		return err
	}
	imagesRoot := state.ImagesDir()
	if err := ensureNoSymlinkPath(imagesRoot); err != nil {
		return fmt.Errorf("unsafe image storage path: %w", err)
	}
	destDir := state.ImageDir(manifestName, manifestTag)
	if err := ensureNoSymlinkPath(filepath.Dir(destDir)); err != nil {
		return fmt.Errorf("unsafe image destination: %w", err)
	}
	if info, statErr := os.Lstat(destDir); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("image destination is a symlink")
	}
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("remove existing image: %w", err)
	}
	if err := os.Rename(stageRoot, destDir); err != nil {
		return fmt.Errorf("install imported image: %w", err)
	}
	stageRoot = ""

	if LoadFromStore(manifestName, manifestTag) == nil {
		if err := SaveToStore(&Image{Name: manifestName, Tag: manifestTag}); err != nil {
			return fmt.Errorf("save imported image metadata: %w", err)
		}
	}
	fmt.Printf("Imported %s:%s from %s\n", manifestName, manifestTag, path)
	return nil
}

func readLimitedEntry(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return data, nil
}

func writeArchiveStream(path string, r io.Reader, size int64, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("target is a symlink")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode)&07777)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyN(f, r, size)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeArchiveFile(root, path string, data []byte, mode int64) error {
	if err := ensureArchiveAncestors(root, path, true); err != nil {
		return err
	}
	return writeFileBytes(path, data, mode)
}

func writeFileBytes(path string, data []byte, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("target is a symlink")
	}
	return os.WriteFile(path, data, os.FileMode(mode)&07777)
}

func safeArchivePath(root, name string) (string, error) {
	if name == "" || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("empty or NUL-containing path")
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("absolute path")
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("parent traversal")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." {
		return root, nil
	}
	path := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes staging directory")
	}
	return path, nil
}

func ensureArchiveAncestors(root, path string, includeTarget bool) error {
	check := path
	if !includeTarget {
		check = filepath.Dir(path)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	check, err = filepath.Abs(check)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, check)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes staging directory")
	}
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink ancestor %q", current)
		}
	}
	return nil
}

func ensureNoSymlinkPath(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	remainder := strings.TrimPrefix(path, current)
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component %q", current)
		}
	}
	return nil
}

func validateArchiveLink(root, linkPath, target string) error {
	if target == "" || strings.IndexByte(target, 0) >= 0 || strings.HasPrefix(target, "\\") {
		return fmt.Errorf("invalid link target")
	}
	var resolved string
	if strings.HasPrefix(target, "/") {
		resolved = filepath.Join(root, filepath.FromSlash(strings.TrimLeft(target, "/")))
	} else {
		resolved = filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(target))
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return err
	}
	_, err = safeArchivePath(root, filepath.ToSlash(rel))
	return err
}

func validateImageReference(name, tag string) error {
	if name == "" || tag == "" || name == "." || name == ".." || tag == "." || tag == ".." {
		return fmt.Errorf("empty or dot image reference")
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid image name %q", name)
	}
	if strings.ContainsAny(tag, "/\\") {
		return fmt.Errorf("invalid image tag %q", tag)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid image name %q", name)
		}
	}
	return nil
}

func stringsReplace(s, old, new string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if string(s[i]) == old {
			result = append(result, []byte(new)...)
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
