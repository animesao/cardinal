//go:build linux

package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cardinal/internal/container"
)

func TestAutomaticBackupDefaults(t *testing.T) {
	if got := automaticBackupInterval(&container.Container{}); got != defaultBackupInterval {
		t.Fatalf("automaticBackupInterval() = %v, want %v", got, defaultBackupInterval)
	}
	if got := automaticBackupRetention(&container.Container{}); got != defaultBackupRetention {
		t.Fatalf("automaticBackupRetention() = %d, want %d", got, defaultBackupRetention)
	}

	c := &container.Container{BackupInterval: "15m", BackupRetention: 3}
	if got := automaticBackupInterval(c); got != 15*time.Minute {
		t.Fatalf("automaticBackupInterval() = %v, want 15m", got)
	}
	if got := automaticBackupRetention(c); got != 3 {
		t.Fatalf("automaticBackupRetention() = %d, want 3", got)
	}
}

func TestPruneAutomaticBackups(t *testing.T) {
	dir := t.TempDir()
	c := &container.Container{Name: "minecraft", BackupDir: dir, BackupRetention: 2}
	for i, name := range []string{"minecraft-old.tar.gz", "minecraft-mid.tar.gz", "minecraft-new.tar.gz"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Now().Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneAutomaticBackups(c); err != nil {
		t.Fatalf("pruneAutomaticBackups() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "minecraft-old.tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("oldest backup still exists, err = %v", err)
	}
	for _, name := range []string{"minecraft-mid.tar.gz", "minecraft-new.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("backup %s missing: %v", name, err)
		}
	}
}
