package overlayutil

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	_ = tw.Close()
	_ = gw.Close()
	return path
}

func TestShortDigest(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "sha256:abcdef123456"},
		{"short", "short"},
		{"", ""},
		{"123456789012345678901", "1234567890123456789"},
	}
	for _, tt := range tests {
		got := ShortDigest(tt.input)
		if got != tt.want {
			t.Errorf("ShortDigest(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello world"
	_ = os.WriteFile(path, []byte(content), 0644)

	hash, size := HashFile(path)
	if size != int64(len(content)) {
		t.Errorf("HashFile size = %d, want %d", size, len(content))
	}
	if hash == "" {
		t.Error("HashFile returned empty hash")
	}
	if len(hash) != 64 {
		t.Errorf("HashFile hash length = %d, want 64 (SHA256 hex)", len(hash))
	}

	hash2, size2 := HashFile(filepath.Join(dir, "nonexistent"))
	if hash2 != "" || size2 != 0 {
		t.Errorf("HashFile for non-existent: hash=%q size=%d", hash2, size2)
	}
}

func TestExtractLayer(t *testing.T) {
	extractDir := t.TempDir()
	tarFile := makeTarGz(t, map[string]string{"hello.txt": "world"})

	if err := ExtractLayer(tarFile, extractDir); err != nil {
		t.Fatalf("ExtractLayer failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(extractDir, "hello.txt"))
	if err != nil {
		t.Fatalf("extracted file not found: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("content = %q, want %q", string(data), "world")
	}

	err = ExtractLayer(filepath.Join(t.TempDir(), "nope.tar.gz"), extractDir)
	if err == nil {
		t.Error("ExtractLayer on non-existent file should fail")
	}
}

func TestExtractLayerEmptyTar(t *testing.T) {
	extractDir := t.TempDir()
	tarFile := makeTarGz(t, nil)
	if err := ExtractLayer(tarFile, extractDir); err != nil {
		t.Fatalf("ExtractLayer on empty tar should succeed, got: %v", err)
	}
}

func TestUnmountOverlay(t *testing.T) {
	UnmountOverlay("/nonexistent/path/12345")
}

func TestShortDigestEdgeCases(t *testing.T) {
	if ShortDigest("") != "" {
		t.Error("ShortDigest('') should be empty")
	}
	long := strings.Repeat("a", 100)
	if len(ShortDigest(long)) != 19 {
		t.Errorf("ShortDigest(long) length = %d, want 19", len(ShortDigest(long)))
	}
}
