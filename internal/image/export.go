package image

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dck/internal/state"
)

// Export saves an image as a .tar.gz file
func Export(ref, outputPath string) error {
	name, tag := parseRef(ref)

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
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	imageDir := state.ImageDir(name, tag)

	err = filepath.Walk(imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(imageDir, path)
		if relPath == "." {
			return nil
		}
		relPath = stringsReplace(relPath, "\\", "/")

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if _, err := tw.Write(data); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk image dir: %w", err)
	}

	fmt.Printf("Exported %s:%s -> %s (%d bytes)\n", name, tag, outputPath, fileSize(outputPath))
	return nil
}

// Import loads an image from a .tar.gz file
func Import(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	// Extract image name/tag from manifest
	var manifestName, manifestTag string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		if header.Name == "image.json" {
			data, readErr := io.ReadAll(tr)
			if readErr != nil {
				return readErr
			}
			var metadata Image
			if json.Unmarshal(data, &metadata) == nil && metadata.Name != "" {
				manifestName, manifestTag = metadata.Name, metadata.Tag
				if manifestTag == "" {
					manifestTag = "latest"
				}
			}
			break
		}
		if header.Name == "manifest.json" {
			data, readErr := io.ReadAll(tr)
			if readErr != nil {
				return readErr
			}
			// Also accept Docker-style exported manifests.
			var cfg struct {
				RepoTags []string `json:"RepoTags"`
			}
			if json.Unmarshal(data, &cfg) == nil && len(cfg.RepoTags) > 0 {
				manifestName, manifestTag = parseRef(cfg.RepoTags[0])
			}
		}
	}

	if manifestName == "" {
		manifestName = "imported"
	}
	if manifestTag == "" {
		manifestTag = "latest"
	}

	// Rewind
	_, _ = f.Seek(0, 0)
	_ = gr.Close()
	_ = f.Close()

	// Re-open and extract
	f2, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f2.Close()

	gr2, err := gzip.NewReader(f2)
	if err != nil {
		return err
	}
	defer gr2.Close()

	tr2 := tar.NewReader(gr2)
	destDir := state.ImageDir(manifestName, manifestTag)
	os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return err
	}

	for {
		header, err := tr2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0755)
		data, err := io.ReadAll(tr2)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
	}

	// Ensure imported archives have dck's internal metadata even when they
	// originated from a Docker-style archive.
	if LoadFromStore(manifestName, manifestTag) == nil {
		if err := SaveToStore(&Image{Name: manifestName, Tag: manifestTag}); err != nil {
			return fmt.Errorf("save imported image metadata: %w", err)
		}
	}

	fmt.Printf("Imported %s:%s from %s\n", manifestName, manifestTag, path)
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
