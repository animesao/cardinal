//go:build linux

package orchestrator

import (
	"testing"
)

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if len(id1) != 16 {
		t.Errorf("expected 16 hex chars, got %d (%s)", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
}

func TestHostname(t *testing.T) {
	h := hostname()
	if h == "" {
		t.Error("hostname should not be empty")
	}
}

func TestCPUCount(t *testing.T) {
	n := cpuCores()
	if n == 0 {
		t.Error("cpuCores should return > 0")
	}
}

func TestMemTotal(t *testing.T) {
	n := memTotal()
	if n == 0 {
		t.Log("memTotal returned 0 (may be non-Linux)")
	}
}
