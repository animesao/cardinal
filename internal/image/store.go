package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/state"
)

func SaveToStore(img *Image) error {
	imgDir := state.ImageDir(img.Name, img.Tag)
	if err := os.MkdirAll(imgDir, 0700); err != nil {
		return fmt.Errorf("mkdir image dir: %w", err)
	}
	if err := os.Chmod(imgDir, 0700); err != nil {
		return fmt.Errorf("chmod image dir: %w", err)
	}
	return state.WriteJSON(filepath.Join(imgDir, "image.json"), img)
}

func LoadFromStore(name, tag string) *Image {
	path := filepath.Join(state.ImageDir(name, tag), "image.json")
	var img Image
	if state.FileExists(path) {
		if err := state.ReadJSON(path, &img); err == nil && img.Name != "" {
			return &img
		}
	}

	// Read legacy internal metadata written to manifest.json by older releases.
	// OCI manifests are rejected by requiring the internal name and tag fields.
	legacyPath := filepath.Join(state.ImageDir(name, tag), "manifest.json")
	if state.FileExists(legacyPath) {
		var legacy Image
		if err := state.ReadJSON(legacyPath, &legacy); err == nil && legacy.Name != "" && legacy.Tag != "" {
			return &legacy
		}
	}
	return nil
}

func ListImages() ([]Image, error) {
	imagesDir := state.ImagesDir()
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var images []Image
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		namespace := entry.Name()
		imgNames, err := os.ReadDir(filepath.Join(imagesDir, namespace))
		if err != nil {
			continue
		}
		for _, imgName := range imgNames {
			if !imgName.IsDir() {
				continue
			}
			fullName := namespace + "/" + imgName.Name()
			tags, err := os.ReadDir(filepath.Join(imagesDir, namespace, imgName.Name()))
			if err != nil {
				continue
			}
			for _, tag := range tags {
				if tag.IsDir() {
					img := LoadFromStore(fullName, tag.Name())
					if img != nil {
						images = append(images, *img)
					}
				}
			}
		}
	}
	return images, nil
}

func RemoveImage(name, tag string) error {
	return os.RemoveAll(state.ImageDir(name, tag))

}

func HasImage(name, tag string) bool {
	return state.FileExists(filepath.Join(state.ImageDir(name, tag), "image.json"))
}

func parseRef(ref string) (name, tag string) {
	tag = "latest"
	if i := strings.LastIndex(ref, ":"); i > 0 && strings.LastIndex(ref, "/") < i {
		tag = ref[i+1:]
		ref = ref[:i]
	}
	if !strings.Contains(ref, "/") {
		ref = "library/" + ref
	}
	return ref, tag
}
