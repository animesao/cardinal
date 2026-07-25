package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dck/internal/overlayutil"
	"dck/internal/state"
)

func CommitContainer(rootfsDir, name, tag, author, message string) (*Image, error) {
	if err := os.MkdirAll(state.ImagesDir(), 0755); err != nil {
		return nil, err
	}

	ref, refTag := parseRef(name + ":" + tag)

	imgDir := state.ImageDir(ref, refTag)
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return nil, err
	}

	layerFile := filepath.Join(imgDir, "layer.tar.gz")
	if err := createLayer(rootfsDir, layerFile); err != nil {
		return nil, fmt.Errorf("create layer: %w", err)
	}

	layerHash, layerSize := hashFile(layerFile)
	layerDigest := "sha256:" + layerHash

	config := map[string]interface{}{
		"created": time.Now().UTC().Format(time.RFC3339),
		"author":  author,
		"architecture": "amd64",
		"os":       "linux",
		"config": map[string]interface{}{
			"Cmd":        []string{"/bin/sh"},
			"Entrypoint": nil,
		},
		"history": []map[string]interface{}{
			{
				"created":   time.Now().UTC().Format(time.RFC3339),
				"created_by": "dck commit",
				"author":    author,
				"comment":   message,
			},
		},
		"rootfs": map[string]interface{}{
			"type": "layers",
			"diff_ids": []string{layerDigest},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	configHash := sha256.Sum256(configData)
	configDigest := "sha256:" + hex.EncodeToString(configHash[:])

	if err := os.WriteFile(filepath.Join(imgDir, "config.json"), configData, 0644); err != nil {
		return nil, err
	}

	manifest := ManifestV2{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		}{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Size:      len(configData),
			Digest:    configDigest,
		},
		Layers: []struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		}{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      layerSize,
				Digest:    layerDigest,
			},
		},
	}

	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), manifestData, 0644); err != nil {
		return nil, err
	}

	img := &Image{Name: ref, Tag: refTag, Digest: configDigest}
	if err := SaveToStore(img); err != nil {
		return nil, err
	}

	rootfsDir2 := state.ImageRootfsDir(ref, refTag)
	if err := extractLayer(layerFile, rootfsDir2); err != nil {
		return nil, fmt.Errorf("extract rootfs: %w", err)
	}

	return img, nil
}

func createLayer(rootfsDir, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(rootfsDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, rootfsDir)
		relPath = strings.TrimPrefix(relPath, string(os.PathSeparator))
		if relPath == "" {
			return nil
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath

		if fi.IsDir() && len(relPath) > 0 {
			hdr.Name = relPath + "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		f.Close()
		return err
	})
}

func hashFile(path string) (string, int) {
	h, size := overlayutil.HashFile(path)
	return h, int(size)
}
