//go:build linux

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupChecksumRoundTrip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "container.tar.gz")
	if err := os.WriteFile(archive, []byte("backup contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupChecksum(archive); err != nil {
		t.Fatalf("writeBackupChecksum: %v", err)
	}
	if err := verifyBackupChecksum(archive); err != nil {
		t.Fatalf("verifyBackupChecksum: %v", err)
	}
}

func TestBackupChecksumDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "container.tar.gz")
	if err := os.WriteFile(archive, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupChecksum(archive); err != nil {
		t.Fatalf("writeBackupChecksum: %v", err)
	}
	if err := os.WriteFile(archive, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupChecksum(archive); err == nil {
		t.Fatal("verifyBackupChecksum unexpectedly accepted modified archive")
	}
}
