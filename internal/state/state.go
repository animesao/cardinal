package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func DataDir() string {
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
	// Ensure Home is set for root when running under systemd
	if os.Getuid() == 0 {
		_ = os.Setenv("HOME", "/root")
	}
}

func ImagesDir() string {
	return filepath.Join(DataDir(), "images")
}

func ContainersDir() string {
	return filepath.Join(DataDir(), "containers")
}

func LogsDir() string {
	return filepath.Join(DataDir(), "logs")
}

func OverlayDir() string {
	return filepath.Join(DataDir(), "overlay")
}

func VolumesDir() string {
	return filepath.Join(DataDir(), "volumes")
}

func ResolveVolume(source string) string {
	// Named volumes (no path separators) are stored under VolumesDir
	if !strings.Contains(source, "/") && !strings.Contains(source, "\\") {
		return filepath.Join(VolumesDir(), source)
	}
	return source
}

func ImageDir(name, tag string) string {
	return filepath.Join(ImagesDir(), name, tag)
}

func ImageRootfsDir(name, tag string) string {
	return filepath.Join(ImageDir(name, tag), "rootfs")
}

func ContainerPath(id string) string {
	return filepath.Join(ContainersDir(), id+".json")
}

func LogPath(id string) string {
	return filepath.Join(LogsDir(), id+".log")
}

func OverlayDirs(id string) (upper, work, merged string) {
	base := filepath.Join(OverlayDir(), id)
	return filepath.Join(base, "upper"),
		filepath.Join(base, "work"),
		filepath.Join(base, "merged")
}

func ConsolesDir() string {
	return filepath.Join(DataDir(), "consoles")
}

func CacheDir() string {
	return filepath.Join(DataDir(), "cache")
}

func LayerCacheDir() string {
	return filepath.Join(CacheDir(), "layers")
}

func LayerPath(digest string) string {
	hash := strings.TrimPrefix(digest, "sha256:")
	return filepath.Join(LayerCacheDir(), hash, "layer.tar.gz")
}

func ConsolePath(containerID string) string {
	return filepath.Join(ConsolesDir(), containerID+".sock")
}

func EnsureDirs() error {
	for _, d := range []string{DataDir(), ImagesDir(), ContainersDir(), LogsDir(), OverlayDir(), ConsolesDir(), VolumesDir(), CacheDir(), LayerCacheDir()} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

func WriteJSON(path string, v interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}

func ReadJSON(path string, v interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
