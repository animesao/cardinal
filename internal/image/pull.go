package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dck/internal/overlayutil"
	"dck/internal/state"
)

const (
	registryURL = "https://registry-1.docker.io"
	authURL     = "https://auth.docker.io/token"
	authService = "registry.docker.io"
)

var httpClient = &http.Client{Timeout: 300 * time.Second}

type authResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

func Pull(ref string) (*Image, error) {
	return PullWithPlatform(ref, "", "")
}

func PullWithPlatform(ref, platformOS, platformArch string) (*Image, error) {
	if err := os.MkdirAll(state.ImagesDir(), 0700); err != nil {
		return nil, err
	}

	name, tag := parseRef(ref)
	if img := LoadFromStore(name, tag); img != nil {
		fmt.Printf("Image %s:%s already exists\n", name, tag)
		return img, nil
	}

	fmt.Printf("Pulling %s:%s...\n", name, tag)

	token, err := getToken(name)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	manifest, err := getResolvedManifest(name, tag, token, platformOS, platformArch)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	configData, err := downloadBlob(name, manifest.Config.Digest, token)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var cfg ContainerConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	rootfsDir := state.ImageRootfsDir(name, tag)
	layersDir := filepath.Join(state.ImageDir(name, tag), "layers")
	if err := os.MkdirAll(layersDir, 0700); err != nil {
		return nil, fmt.Errorf("create layers directory: %w", err)
	}

	isTerminal := isTerminalOutput()

	for i, layer := range manifest.Layers {
		label := fmt.Sprintf(" %s", shortDigest(layer.Digest))
		if isTerminal {
			fmt.Printf("  %s\r", label)
		} else {
			fmt.Printf("  Layer %d/%d: %s\n", i+1, len(manifest.Layers), shortDigest(layer.Digest))
		}
		cachePath := filepath.Join(layersDir, strings.ReplaceAll(layer.Digest, ":", "_"))

		if err := verifyFileDigest(cachePath, layer.Digest); err != nil {
			percentFn := func(pct int) {
				if isTerminal {
					fmt.Printf("  %s [%s%s] %d%%\r", label, bar(pct, 30), bar(100-pct, 30), pct)
				}
			}
			if err := downloadBlobToFile(name, layer.Digest, token, cachePath, percentFn); err != nil {
				if isTerminal {
					fmt.Println()
				}
				return nil, fmt.Errorf("layer %d: %w", i, err)
			}
			if isTerminal {
				fmt.Printf("  %s [%s] 100%%\n", label, bar(100, 30))
			}
		}

		if isTerminal {
			fmt.Printf("  %s extracting...\r", label)
		}
		if err := extractLayer(cachePath, rootfsDir); err != nil {
			if isTerminal {
				fmt.Println()
			}
			return nil, fmt.Errorf("extract layer %d: %w", i, err)
		}
	}
	if isTerminal {
		fmt.Print(strings.Repeat(" ", 60) + "\r")
	}

	if err := saveConfig(state.ImageDir(name, tag), configData); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	// Save the OCI/Docker manifest separately from dck's internal image.json metadata.
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := state.WriteFileAtomic(filepath.Join(state.ImageDir(name, tag), "manifest.json"), manifestData, 0600); err != nil {
		return nil, fmt.Errorf("save manifest: %w", err)
	}

	img := &Image{Name: name, Tag: tag, Digest: manifest.Config.Digest}
	if err := SaveToStore(img); err != nil {
		return nil, err
	}

	fmt.Printf("Done: %s:%s\n", name, tag)
	return img, nil
}

func getResolvedManifest(repo, ref, token, platformOS, platformArch string) (*ManifestV2, error) {
	m, raw, err := fetchRawManifest(repo, ref, token)
	if err != nil {
		return nil, err
	}

	if m.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		m.MediaType == "application/vnd.oci.image.index.v1+json" {
		var list ManifestList
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("parse manifest list: %w", err)
		}

		targetArch := platformArch
		targetOS := platformOS
		if targetArch == "" {
			targetArch = "amd64"
		}
		if targetOS == "" {
			targetOS = "linux"
		}

		var targetDigest string
		for _, entry := range list.Manifests {
			if entry.Platform.Architecture == targetArch && entry.Platform.OS == targetOS {
				targetDigest = entry.Digest
				break
			}
		}
		if targetDigest == "" && len(list.Manifests) > 0 {
			targetDigest = list.Manifests[0].Digest
		}
		if targetDigest == "" {
			return nil, fmt.Errorf("no suitable manifest found in list")
		}
		fmt.Printf("  Resolved multi-arch to %s\n", shortDigest(targetDigest))
		return getResolvedManifest(repo, targetDigest, token, platformOS, platformArch)
	}

	if m.SchemaVersion == 0 || len(m.Layers) == 0 {
		var v2 ManifestV2
		if err := json.Unmarshal(raw, &v2); err != nil {
			return nil, fmt.Errorf("parse manifest v2: %w", err)
		}
		if v2.SchemaVersion == 0 || len(v2.Layers) == 0 {
			return nil, fmt.Errorf("unrecognized manifest format (mediaType: %s)", m.MediaType)
		}
		return &v2, nil
	}

	return m, nil
}

func fetchRawManifest(repo, ref, token string) (*ManifestV2, []byte, error) {
	u := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repo, ref)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept",
		"application/vnd.docker.distribution.manifest.v2+json,"+
			"application/vnd.oci.image.manifest.v1+json,"+
			"application/vnd.docker.distribution.manifest.list.v2+json,"+
			"application/vnd.oci.image.index.v1+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var m ManifestV2
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, raw, nil
}

func saveConfig(dir string, data []byte) error {
	return state.WriteFileAtomic(filepath.Join(dir, "config.json"), data, 0600)
}

func ReadConfig(name, tag string) (*ContainerConfig, error) {
	path := filepath.Join(state.ImageDir(name, tag), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ContainerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func getToken(repo string) (string, error) {
	u := fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", authURL, authService, repo)
	resp, err := httpClient.Get(u)
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

func getManifest(repo, ref, token string) (*ManifestV2, error) {
	return getResolvedManifest(repo, ref, token, "", "")
}

func downloadBlob(repo, digest, token string) ([]byte, error) {
	u := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, digest)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := verifyDigest(data, digest); err != nil {
		return nil, err
	}
	return data, nil
}

func downloadBlobToFile(repo, digest, token, dest string, onProgress progressFn) error {
	u := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, digest)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.image.rootfs.diff.tar.gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	contentLength := resp.ContentLength

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".dck-layer-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmp, hasher)
	if contentLength > 0 && onProgress != nil {
		written := int64(0)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
					return writeErr
				}
				written += int64(n)
				onProgress(int(written * 100 / contentLength))
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return err
		}
	} else {
		if _, err := io.Copy(writer, resp.Body); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	actual := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if actual != digest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest, actual)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	return nil
}

func verifyFileDigest(path, expected string) error {
	actual, size := overlayutil.HashFile(path)
	if actual == "" {
		return fmt.Errorf("layer cache unavailable: %s", path)
	}
	if expected != "sha256:"+actual {
		return fmt.Errorf("cached layer digest mismatch: expected %s, got sha256:%s", expected, actual)
	}
	if size < 0 {
		return fmt.Errorf("invalid cached layer size")
	}
	return nil
}

func verifyDigest(data []byte, expected string) error {
	h := sha256.Sum256(data)
	actual := "sha256:" + hex.EncodeToString(h[:])
	if actual != expected {
		return fmt.Errorf("digest mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func extractLayer(cachePath, rootfsDir string) error {
	return overlayutil.ExtractLayer(cachePath, rootfsDir)
}

func shortDigest(d string) string {
	return overlayutil.ShortDigest(d)
}

type progressFn func(pct int)

func isTerminalOutput() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func bar(pct, width int) string {
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	b := make([]byte, width)
	for i := 0; i < width; i++ {
		if i < filled {
			b[i] = '='
		} else {
			b[i] = ' '
		}
	}
	return string(b)
}
