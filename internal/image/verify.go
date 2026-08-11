package image

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dck/internal/overlayutil"
	"dck/internal/state"
)

// VerifyReference verifies an image config digest and every locally stored
// manifest layer without contacting the registry.
func VerifyReference(ref string) error {
	name, tag := parseRef(ref)
	img := LoadFromStore(name, tag)
	if img == nil {
		return fmt.Errorf("image %s:%s is not present locally", name, tag)
	}

	configPath := filepath.Join(state.ImageDir(name, tag), "config.json")
	configDigest, err := verifyHashFile(configPath)
	if err != nil {
		return fmt.Errorf("verify config: %w", err)
	}
	if img.Digest != "" && img.Digest != configDigest {
		return fmt.Errorf("config digest mismatch: metadata=%s actual=%s", img.Digest, configDigest)
	}

	manifest := ReadManifest(name, tag)
	if manifest == nil {
		return fmt.Errorf("manifest not found for %s:%s", name, tag)
	}
	if manifest.Config.Digest != "" && manifest.Config.Digest != configDigest {
		return fmt.Errorf("manifest config digest mismatch: manifest=%s actual=%s", manifest.Config.Digest, configDigest)
	}
	for i, layer := range manifest.Layers {
		path := ResolveLayerByIndex(name, tag, i, layer.Digest)
		if path == "" {
			return fmt.Errorf("layer %d %s is missing", i, layer.Digest)
		}
		actual, size := overlayutil.HashFile(path)
		if size <= 0 || actual == "" {
			return fmt.Errorf("layer %d %s cannot be hashed", i, layer.Digest)
		}
		actual = "sha256:" + actual
		if actual != layer.Digest {
			return fmt.Errorf("layer %d digest mismatch: expected %s, got %s", i, layer.Digest, actual)
		}
	}
	return nil
}

func verifyHashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
