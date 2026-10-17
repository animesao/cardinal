//go:build linux

package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirTreeSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), make([]byte, 1000), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.bin"), make([]byte, 500), 0644); err != nil {
		t.Fatal(err)
	}
	if got := dirTreeSize(dir); got != 1500 {
		t.Fatalf("dirTreeSize = %d, want 1500", got)
	}
	if got := dirTreeSize(filepath.Join(dir, "missing")); got != 0 {
		t.Fatalf("dirTreeSize(missing) = %d, want 0", got)
	}
}

func TestIsNamedVolume(t *testing.T) {
	cases := []struct {
		vol  VolumeMount
		want bool
	}{
		{VolumeMount{Type: VolumeTypeVolume, Source: "data"}, true},
		{VolumeMount{Source: "data"}, true},
		{VolumeMount{Type: VolumeTypeBind, Source: "/host/path"}, false},
		{VolumeMount{Type: VolumeTypeTmpfs, Target: "/tmp"}, false},
		{VolumeMount{Type: VolumeTypeNFS, Source: "host:/share"}, false},
		{VolumeMount{}, false},
	}
	for i, tc := range cases {
		if got := isNamedVolume(tc.vol); got != tc.want {
			t.Fatalf("case %d: isNamedVolume = %v, want %v", i, got, tc.want)
		}
	}
}

func TestCachedDirSizeTTL(t *testing.T) {
	dir := t.TempDir()
	id := "test-cache-ttl"
	defer func() {
		duCache.Lock()
		delete(duCache.entries, id)
		duCache.Unlock()
	}()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), make([]byte, 10), 0644); err != nil {
		t.Fatal(err)
	}
	if got := cachedDirSize(id, []string{dir}); got != 10 {
		t.Fatalf("first = %d, want 10", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), make([]byte, 1000), 0644); err != nil {
		t.Fatal(err)
	}
	if got := cachedDirSize(id, []string{dir}); got != 10 {
		t.Fatalf("cached = %d, want stale 10", got)
	}
	duCache.Lock()
	e := duCache.entries[id]
	e.at = time.Now().Add(-2 * duCacheTTL)
	duCache.entries[id] = e
	duCache.Unlock()
	if got := cachedDirSize(id, []string{dir}); got != 1010 {
		t.Fatalf("expired = %d, want 1010", got)
	}
}
