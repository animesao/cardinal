package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cardinal/internal/state"
)

// ConfigDigest returns the digest from the manifest for display.
func configDigest(name, tag string) string {
	m := ReadManifest(name, tag)
	if m == nil {
		return ""
	}
	return shortDigest(m.Config.Digest)
}

func Push(ref, username, password string) error {
	name, tag := parseRef(ref)

	img := LoadFromStore(name, tag)
	if img == nil {
		return fmt.Errorf("image %s:%s not found locally", name, tag)
	}

	fmt.Printf("Pushing %s:%s...\n", name, tag)

	token, err := getPushToken(name, username, password)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	// Read manifest file
	manifestPath := filepath.Join(state.ImageDir(name, tag), "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	var manifest ManifestV2
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Upload config blob
	fmt.Printf("  Config: %s\n", shortDigest(manifest.Config.Digest))
	if err := uploadBlob(name, manifest.Config.Digest, token, func() ([]byte, error) {
		configPath := filepath.Join(state.ImageDir(name, tag), "config.json")
		return os.ReadFile(configPath)
	}); err != nil {
		return fmt.Errorf("upload config: %w", err)
	}

	// Upload each layer blob
	for i, layer := range manifest.Layers {
		fmt.Printf("  Layer %d/%d: %s\n", i+1, len(manifest.Layers), shortDigest(layer.Digest))

		layerPath := ResolveLayerByIndex(name, tag, i, layer.Digest)
		if layerPath == "" {
			return fmt.Errorf("layer %d (%s) not found on disk", i, shortDigest(layer.Digest))
		}

		if err := uploadBlobFromFile(name, layer.Digest, token, layerPath); err != nil {
			return fmt.Errorf("upload layer %d: %w", i, err)
		}
	}

	// Upload manifest
	fmt.Printf("  Manifest: %s:%s\n", name, tag)
	if err := uploadManifest(name, tag, token, manifestData); err != nil {
		return fmt.Errorf("upload manifest: %w", err)
	}

	fmt.Printf("Done: %s:%s\n", name, tag)
	return nil
}

func getPushToken(repo, username, password string) (string, error) {
	scope := fmt.Sprintf("repository:%s:push,pull", repo)
	u := fmt.Sprintf("%s?service=%s&scope=%s", authURL, authService, scope)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}

	if username == "" || password == "" {
		username, password = GetCredentials(repo)
	}
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token HTTP %d: %s", resp.StatusCode, string(body))
	}

	var ar authResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return "", err
	}
	if ar.Token != "" {
		return ar.Token, nil
	}
	return ar.AccessToken, nil
}

func blobExists(repo, digest, token string) (bool, error) {
	u := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, digest)
	req, _ := http.NewRequest("HEAD", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.image.rootfs.diff.tar.gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == 404 {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

func uploadBlob(repo, digest, token string, dataFn func() ([]byte, error)) error {
	exists, err := blobExists(repo, digest, token)
	if err != nil {
		return fmt.Errorf("check blob: %w", err)
	}
	if exists {
		fmt.Printf("    already exists, skipping\n")
		return nil
	}

	data, err := dataFn()
	if err != nil {
		return err
	}

	return monolithicUpload(repo, digest, token, data)
}

func uploadBlobFromFile(repo, digest, token, filePath string) error {
	exists, err := blobExists(repo, digest, token)
	if err != nil {
		return fmt.Errorf("check blob: %w", err)
	}
	if exists {
		fmt.Printf("    already exists, skipping\n")
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return monolithicUpload(repo, digest, token, data)
}

func monolithicUpload(repo, digest, token string, data []byte) error {
	u := fmt.Sprintf("%s/v2/%s/blobs/uploads/?digest=%s", registryURL, repo, digest)
	req, err := http.NewRequest("POST", u, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload blob: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func uploadManifest(repo, tag, token string, manifestData []byte) error {
	u := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repo, tag)
	req, _ := http.NewRequest("PUT", u, strings.NewReader(string(manifestData)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload manifest: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
