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
	defer func() { _ = f.Close() }()
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

func makeSpecialTarGz(t *testing.T, headers ...*tar.Header) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "special.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, header := range headers {
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			content := []byte("content")
			if int64(len(content)) != header.Size {
				content = make([]byte, header.Size)
			}
			if _, err := tw.Write(content); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractLayerRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../outside", "/absolute", "dir/../../outside", `dir\\..\\outside`} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			tarFile := makeTarGz(t, map[string]string{name: "blocked"})
			if err := ExtractLayer(tarFile, root); err == nil {
				t.Fatalf("expected traversal path %q to be rejected", name)
			}
		})
	}
}

func TestExtractLayerAllowsRootDirectoryEntry(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0755},
		&tar.Header{Name: "python", Typeflag: tar.TypeReg, Size: int64(len("python")), Mode: 0755},
	)
	if err := ExtractLayer(tarFile, root); err != nil {
		t.Fatalf("expected root directory entry to be accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "python")); err != nil {
		t.Fatalf("root entry file was not extracted: %v", err)
	}
}

func TestExtractLayerAllowsSafeSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "usr/lib64", Typeflag: tar.TypeDir, Mode: 0755},
		&tar.Header{Name: "lib64", Typeflag: tar.TypeSymlink, Linkname: "usr/lib64", Mode: 0777},
		&tar.Header{Name: "lib64/file.txt", Typeflag: tar.TypeReg, Size: int64(len("content")), Mode: 0644},
	)
	if err := ExtractLayer(tarFile, root); err != nil {
		t.Fatalf("expected safe symlink ancestor to be accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "usr", "lib64", "file.txt")); err != nil {
		t.Fatalf("file was not extracted through safe symlink ancestor: %v", err)
	}
}

func TestExtractLayerAllowsTemurinBuildIDSymlink(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "usr/lib64", Typeflag: tar.TypeDir, Mode: 0755},
		&tar.Header{Name: "usr/lib64/libgcc_s-14.so.1", Typeflag: tar.TypeReg, Size: int64(len("libgcc")), Mode: 0755},
		&tar.Header{Name: "lib64", Typeflag: tar.TypeSymlink, Linkname: "usr/lib64", Mode: 0777},
		&tar.Header{Name: "usr/lib/.build-id/96", Typeflag: tar.TypeDir, Mode: 0755},
		&tar.Header{Name: "usr/lib/.build-id/96/db820fc92f7d60085943810403e791b6fd84a9", Typeflag: tar.TypeSymlink, Linkname: "../../../../lib64/libgcc_s-14.so.1", Mode: 0777},
	)
	if err := ExtractLayer(tarFile, root); err != nil {
		t.Fatalf("expected Temurin build-id symlink to be accepted: %v", err)
	}
	link, err := os.Readlink(filepath.Join(root, "usr", "lib", ".build-id", "96", "db820fc92f7d60085943810403e791b6fd84a9"))
	if err != nil {
		t.Fatalf("read build-id symlink: %v", err)
	}
	want := filepath.FromSlash("../../../../lib64/libgcc_s-14.so.1")
	if link != want {
		t.Fatalf("symlink target = %q, want %q", link, want)
	}
}

func TestExtractLayerAllowsAbsoluteSymlinkWithinRoot(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "bin", Typeflag: tar.TypeDir, Mode: 0755},
		&tar.Header{Name: "bin/busybox", Typeflag: tar.TypeReg, Size: int64(len("busybox")), Mode: 0755},
		&tar.Header{Name: "bin/arch", Typeflag: tar.TypeSymlink, Linkname: "/bin/busybox", Mode: 0777},
	)
	if err := ExtractLayer(tarFile, root); err != nil {
		t.Fatalf("expected absolute in-root symlink to be accepted: %v", err)
	}
	link, err := os.Readlink(filepath.Join(root, "bin", "arch"))
	if err != nil {
		t.Fatalf("read extracted symlink: %v", err)
	}
	want := filepath.FromSlash("/bin/busybox")
	if link != want {
		t.Fatalf("symlink target = %q, want %q", link, want)
	}
}

func TestExtractLayerRejectsUnsafeLinks(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "../../outside", Mode: 0777},
	)
	if err := ExtractLayer(tarFile, root); err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}

	root = t.TempDir()
	tarFile = makeSpecialTarGz(t,
		&tar.Header{Name: "escape", Typeflag: tar.TypeLink, Linkname: "../../outside", Mode: 0644},
	)
	if err := ExtractLayer(tarFile, root); err == nil {
		t.Fatal("expected escaping hardlink to be rejected")
	}
}

func TestExtractLayerRejectsSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	tarFile := makeSpecialTarGz(t,
		&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "real", Mode: 0777},
		&tar.Header{Name: "link/blocked", Typeflag: tar.TypeReg, Size: int64(len("content")), Mode: 0644},
	)
	if err := ExtractLayer(tarFile, root); err == nil {
		t.Fatal("expected file below symlink ancestor to be rejected")
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
