package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

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
// Import + makeImportArchive because the stdlib tar.Writer refuses
// to encode a NUL into a PAX header in the first place; an attacker
// could bypass stdlib and ship a raw tar bytestream with a NUL in
// the path, and we want our defensive layer to catch that case.
//
// safeArchivePath is internal but here we exercise the same checks
// through the public Import path with a hand-crafted tar bytestream.
func TestSafeArchivePathRejectsNullByte(t *testing.T) {
	// Construct a minimal valid tar archive whose header Name has a
	// NUL byte. We bypass stdlib tar.Writer by writing the header
	// bytes directly; the tar format is well-defined enough to do
	// this without depending on the stdlib encoder.
	const headerSize = 512
	type tarHeader struct {
		Name     [100]byte
		Mode     [8]byte
		UID      [8]byte
		GID      [8]byte
		Size     [12]byte
		Mtime    [12]byte
		Chksum   [8]byte
		Typeflag byte
		Linkname [100]byte
		Magic    [6]byte
		Version  [2]byte
		Uname    [32]byte
		Gname    [32]byte
		Devmajor [8]byte
		Devminor [8]byte
		Prefix   [155]byte
		Padding  [12]byte
	}
	var h tarHeader
	copy(h.Name[:], "good\x00hack\x00") // NUL-terminated padded to 100 bytes; second NUL is the str terminator
	h.Typeflag = tar.TypeReg
	// size = 1
	const one = "00000000001\x00"
	copy(h.Size[:], one)
	copy(h.Magic[:], "ustar\x00")
	copy(h.Version[:], "00")

	var raw [headerSize * 2]byte
	headerBytes := (*[headerSize]byte)(unsafe.Pointer(&h))
	// chksum field is treated as all spaces during computation
	for i := range headerBytes {
		if i >= 148 && i < 156 {
			raw[i] = ' '
		} else {
			raw[i] = headerBytes[i]
		}
	}
	// Fill in chksum: octal sum of all 512 bytes
	var sum int64
	for _, b := range raw[:512] {
		sum += int64(b)
	}
	chk := fmt.Sprintf("%06o\x00 ", sum)
	copy(raw[148:156], chk)
	// End-of-archive: 1024 bytes of NULs

	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	if _, err := gw.Write(raw[:]); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	old := os.Getenv("DCK_DATA_DIR")
	_ = os.Setenv("DCK_DATA_DIR", dataDir)
	t.Cleanup(func() { _ = os.Setenv("DCK_DATA_DIR", old) })
	if err := state.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "import.tar.gz")
	if err := os.WriteFile(path, gzbuf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	err := Import(path)
	if err == nil {
		t.Fatal("Import accepted archive whose filename contained a NUL byte")
	}
	if !strings.Contains(err.Error(), "NUL") && !strings.Contains(err.Error(), "unsafe archive path") {
		t.Errorf("Import error did not mention NUL/unsafe path; got: %v", err)
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
