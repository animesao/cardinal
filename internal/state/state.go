package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func DataDir() string {
	if dir := strings.TrimSpace(os.Getenv("DCK_DATA_DIR")); dir != "" {
		return filepath.Clean(dir)
	}
	if os.Getuid() == 0 {
		return "/root/.dck"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	return filepath.Join(home, ".dck")
}

func init() {
	if os.Getuid() == 0 {
		_ = os.Setenv("HOME", "/root")
	}
}

func ImagesDir() string     { return filepath.Join(DataDir(), "images") }
func ContainersDir() string { return filepath.Join(DataDir(), "containers") }
func LogsDir() string       { return filepath.Join(DataDir(), "logs") }
func OverlayDir() string    { return filepath.Join(DataDir(), "overlay") }
func VolumesDir() string    { return filepath.Join(DataDir(), "volumes") }

func ResolveVolume(source string) string {
	if !strings.Contains(source, "/") && !strings.Contains(source, "\\") {
		return filepath.Join(VolumesDir(), safePart(source))
	}
	return source
}

func ImageDir(name, tag string) string {
	// Keep repository namespaces (for example library/alpine), but never allow
	// an invalid reference to control a filesystem path. Invalid references are
	// mapped to an unreachable-looking private key under images instead.
	if !validImageName(name) || !validTag(tag) {
		return filepath.Join(ImagesDir(), ".invalid", referenceKey(name, tag))
	}
	return filepath.Join(ImagesDir(), filepath.FromSlash(name), tag)
}

func ImageRootfsDir(name, tag string) string { return filepath.Join(ImageDir(name, tag), "rootfs") }
func ContainerPath(id string) string         { return filepath.Join(ContainersDir(), safePart(id)+".json") }
func LogPath(id string) string               { return filepath.Join(LogsDir(), safePart(id)+".log") }

func OverlayDirs(id string) (upper, work, merged string) {
	base := filepath.Join(OverlayDir(), safePart(id))
	return filepath.Join(base, "upper"), filepath.Join(base, "work"), filepath.Join(base, "merged")
}

func ConsolesDir() string   { return filepath.Join(DataDir(), "consoles") }
func CacheDir() string      { return filepath.Join(DataDir(), "cache") }
func LayerCacheDir() string { return filepath.Join(CacheDir(), "layers") }
func LayerPath(digest string) string {
	return filepath.Join(LayerCacheDir(), safePart(strings.TrimPrefix(digest, "sha256:")), "layer.tar.gz")
}
func ConsolePath(containerID string) string {
	return filepath.Join(ConsolesDir(), safePart(containerID)+".sock")
}

func safePart(value string) string {
	if value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`) && filepath.Base(value) == value {
		return value
	}
	return referenceKey(value)
}

func referenceKey(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func validImageName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validTag(tag string) bool {
	return tag != "" && tag != "." && tag != ".." && !strings.ContainsAny(tag, `/\\`)
}

func EnsureDirs() error {
	for _, d := range []string{DataDir(), ImagesDir(), ContainersDir(), LogsDir(), OverlayDir(), ConsolesDir(), VolumesDir(), CacheDir(), LayerCacheDir()} {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
		if err := os.Chmod(d, 0700); err != nil {
			return err
		}
	}
	return nil
}

// WriteFileAtomic writes data to a temporary file in the destination directory,
// syncs it, and atomically replaces the destination.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dck-atomic-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func WriteJSON(path string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, 0600)
}

func ReadJSON(path string, v interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(v)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
