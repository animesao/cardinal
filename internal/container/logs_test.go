//go:build linux

package container

import (
	"os"
	"testing"
)

func TestOpenFreshLogFileTruncatesPreviousRun(t *testing.T) {
	path := t.TempDir() + "/container.log"
	if err := os.WriteFile(path, []byte("previous run\n"), 0600); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	f, err := OpenFreshLogFile(path, 0600)
	if err != nil {
		t.Fatalf("open fresh log: %v", err)
	}
	if _, err := f.WriteString("current run\n"); err != nil {
		_ = f.Close()
		t.Fatalf("write current log: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if got, want := string(data), "current run\n"; got != want {
		t.Fatalf("log contents = %q, want %q", got, want)
	}
}
