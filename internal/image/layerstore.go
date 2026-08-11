package image

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/state"
)

// EnsureLayer stores a layer file in the shared content-addressable cache.
// Returns the sha256 digest and size. If the layer already exists, it's a no-op.
func EnsureLayer(layerPath string) (digest string, size int, err error) {
	// Compute hash
	f, err := os.Open(layerPath)
	if err != nil {
		return "", 0, fmt.Errorf("open layer: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash layer: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", 0, fmt.Errorf("close layer: %w", err)
	}

	hash := hex.EncodeToString(h.Sum(nil))
	digest = "sha256:" + hash
	size = int(n)

	// Check if already cached
	cachePath := state.LayerPath(digest)
	if state.FileExists(cachePath) {
		return digest, size, nil
	}

	// Copy to cache
	if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
		return "", 0, err
	}
	if err := os.Chmod(filepath.Dir(cachePath), 0700); err != nil {
		return "", 0, err
	}
	src, err := os.Open(layerPath)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(cachePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return "", 0, err
	}

	return digest, size, nil
}

// ResolveLayer returns the path to the cached layer file for the given digest.
// Returns empty string if the layer is not found in cache.
func ResolveLayer(digest string) string {
	cachePath := state.LayerPath(digest)
	if state.FileExists(cachePath) {
		return cachePath
	}
	return ""
}

// ResolveLayerByIndex resolves a layer from an image directory.
// Checks cache first, then falls back to local image dir.
func ResolveLayerByIndex(name, tag string, index int, digest string) string {
	// First check shared cache
	if p := ResolveLayer(digest); p != "" {
		return p
	}

	imgDir := state.ImageDir(name, tag)

	// Check layer_N.tar.gz (from build.go)
	layerPath := filepath.Join(imgDir, fmt.Sprintf("layer_%d.tar.gz", index))
	if state.FileExists(layerPath) {
		return layerPath
	}

	// Check layer.tar.gz (from commit.go)
	layerPath2 := filepath.Join(imgDir, "layer.tar.gz")
	if state.FileExists(layerPath2) {
		return layerPath2
	}

	// Check layers/<digest_with_underscore> (from pull.go)
	cacheKey := strings.ReplaceAll(digest, ":", "_")
	layerPath3 := filepath.Join(imgDir, "layers", cacheKey)
	if state.FileExists(layerPath3) {
		return layerPath3
	}

	return ""
}

// EnsureAllLayers reads the manifest and imports all layers into the shared cache.
func EnsureAllLayers(name, tag string) error {
	manifest := ReadManifest(name, tag)
	if manifest == nil {
		return fmt.Errorf("manifest not found for %s:%s", name, tag)
	}

	for i, layer := range manifest.Layers {
		layerPath := ResolveLayerByIndex(name, tag, i, layer.Digest)
		if layerPath == "" {
			return fmt.Errorf("layer %d (%s) not found", i, shortDigest(layer.Digest))
		}

		// Ensure in shared cache
		digest, _, err := EnsureLayer(layerPath)
		if err != nil {
			return fmt.Errorf("cache layer %d: %w", i, err)
		}

		// Verify digest matches
		if digest != layer.Digest {
			return fmt.Errorf("layer %d digest mismatch: expected %s, got %s", i, layer.Digest, digest)
		}
	}

	return nil
}

// ReadManifest reads the OCI manifest from the image directory.
// Falls back to reconstructing from config.json and layers/ directory.
func ReadManifest(name, tag string) *ManifestV2 {
	imgDir := state.ImageDir(name, tag)

	// The OCI/Docker manifest is kept separate from dck's internal image.json metadata.
	// Keep a read-only fallback for images written by older dck versions.
	for _, f := range []string{"manifest.json", "oci-manifest.json"} {
		p := filepath.Join(imgDir, f)
		if !state.FileExists(p) {
			continue
		}
		var m ManifestV2
		if err := state.ReadJSON(p, &m); err == nil && len(m.Layers) > 0 {
			return &m
		}
	}

	// Fallback: reconstruct from layers/ directory
	layersDir := filepath.Join(imgDir, "layers")
	entries, err := os.ReadDir(layersDir)
	if err != nil {
		return nil
	}
	var m ManifestV2
	m.SchemaVersion = 2
	m.MediaType = "application/vnd.docker.distribution.manifest.v2+json"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		d := strings.ReplaceAll(e.Name(), "_", ":")
		fi, _ := e.Info()
		m.Layers = append(m.Layers, struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		}{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Size:      int(fi.Size()),
			Digest:    d,
		})
	}
	if len(m.Layers) == 0 {
		return nil
	}
	return &m
}
