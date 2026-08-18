//go:build linux

package cmd

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractBackupAllowsRootDataDirectory(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "backup.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	writeEntry := func(name string, mode int64, data []byte) {
		t.Helper()
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if len(data) > 0 {
			if _, err := tarWriter.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}

	writeEntry("container.json", 0600, []byte(`{"id":"test-container"}`))
	writeEntry("data", 0700, nil)
	writeEntry("data/example.txt", 0600, []byte("restored"))
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	metadata, err := extractBackupToStage(stage, archivePath)
	if err != nil {
		t.Fatalf("root data directory should be accepted: %v", err)
	}
	if string(metadata) != `{"id":"test-container"}` {
		t.Fatalf("unexpected metadata: %s", metadata)
	}
	content, err := os.ReadFile(filepath.Join(stage, "data", "example.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "restored" {
		t.Fatalf("unexpected restored content: %q", content)
	}
}
