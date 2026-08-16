package image

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dck/internal/state"
)

func makeImportArchive(t *testing.T, headers ...*tar.Header) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "image.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			body := make([]byte, h.Size)
			if _, err := tw.Write(body); err != nil {
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

func imageMetadata(t *testing.T, name, tag string) *tar.Header {
	t.Helper()
	data, err := json.Marshal(Image{Name: name, Tag: tag})
	if err != nil {
		t.Fatal(err)
	}
	return &tar.Header{Name: "image.json", Typeflag: tar.TypeReg, Size: int64(len(data)), Mode: 0600, PAXRecords: map[string]string{"dck-test": string(data)}}
}

func TestImportRejectsTraversal(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	if err := os.Setenv("DCK_DATA_DIR", dataDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("DCK_DATA_DIR", old) }()
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	archive := makeImportArchive(t, &tar.Header{Name: "../../escape", Typeflag: tar.TypeReg, Size: 1, Mode: 0600})
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted traversal entry")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "escape")); !os.IsNotExist(err) {
		t.Fatalf("traversal created outside file: %v", err)
	}
}

func TestImportRejectsDuplicateMetadata(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	defer func() { _ = os.Setenv("DCK_DATA_DIR", old) }()
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := makeImportArchive(t, imageMetadata(t, "safe", "latest"), imageMetadata(t, "safe", "latest"))
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted duplicate metadata")
	}
}

func TestImportRejectsSpecialEntries(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	defer func() { _ = os.Setenv("DCK_DATA_DIR", old) }()
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := makeImportArchive(t, &tar.Header{Name: "device", Typeflag: tar.TypeChar, Mode: 0600})
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted a device entry")
	}
}

func TestImportRejectsUnsafeMetadataReference(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	defer func() { _ = os.Setenv("DCK_DATA_DIR", old) }()
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "metadata.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	body, _ := json.Marshal(Image{Name: "../../outside", Tag: "latest"})
	h := &tar.Header{Name: "image.json", Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0600}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted unsafe image metadata")
	}
}

// TestImportRejectsHardlinkOfflineArchive documents that hardlink entries
// (TypeLink in tar) cannot be created in this codebase at all — we only
// accept regular files, directories, and symlinks so a malicious layer can
// not point a hardlink at /etc/passwd.
func TestImportRejectsHardlinkOfflineArchive(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	t.Cleanup(func() { _ = os.Setenv("DCK_DATA_DIR", old) })
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := makeImportArchive(t,
		imageMetadata(t, "normal", "latest"),
		&tar.Header{Name: "passwd", Typeflag: tar.TypeLink, Linkname: "../../../../etc/passwd"},
	)
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted archive with TypeLink")
	}
}

// TestSafeArchivePathRejectsNullByte verifies that the path-sanity
// layer rejects filenames containing NUL bytes before they reach the
// filesystem. We test the helper directly instead of going through
// Import + a raw tar bytestream because Go's archive/tar.Reader uses
// C-string semantics: a Name byte sequence with an interior NUL is
// silently truncated at the NUL, so the parsed header.Name never
// carries the trailing portion of the malicious path through to our
// safeArchivePath. The defensive check we want is the one directly
// against safeArchivePath.
func TestSafeArchivePathRejectsNullByte(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name string
	}{
		{"good\x00hack"},
		{"\x00leading-nul"},
		{"trailing-nul\x00"},
		{"multi\x00ple\x00nuls"},
	}
	for _, c := range cases {
		if _, err := safeArchivePath(root, c.name); err == nil {
			t.Errorf("safeArchivePath(%q) accepted NUL-containing path", c.name)
		} else if !strings.Contains(err.Error(), "NUL") {
			t.Errorf("safeArchivePath(%q) error %q did not mention NUL", c.name, err)
		}
	}
}

// TestImportRejectsSymlinkLoop covers the secondary attack class where a
// layer ships a symlink that points to itself or to an ancestor. The
// symlink resolver caches paths and a permanent loop could enable
// unbounded recursion if naive traversal code follows it.
func TestImportRejectsSymlinkLoop(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	t.Cleanup(func() { _ = os.Setenv("DCK_DATA_DIR", old) })
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := makeImportArchive(t,
		imageMetadata(t, "normal", "latest"),
		&tar.Header{Name: "loop", Typeflag: tar.TypeSymlink, Linkname: "loop"},
	)
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted a self-referencing symlink")
	}
}

// TestImportRejectsDeepTraversal attempts to smuggle a file out of the
// extraction root using `..` segments in the middle of a longer path; the
// destination validator must still notice the parent-directory escape.
func TestImportRejectsDeepTraversal(t *testing.T) {
	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	t.Cleanup(func() { _ = os.Setenv("DCK_DATA_DIR", old) })
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	archive := makeImportArchive(t,
		imageMetadata(t, "normal", "latest"),
		&tar.Header{Name: "good/../../escape", Typeflag: tar.TypeReg, Size: 1, Mode: 0600},
	)
	if err := Import(archive); err == nil {
		t.Fatal("Import accepted nested-traversal path")
	}
}
