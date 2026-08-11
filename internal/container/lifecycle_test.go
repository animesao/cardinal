//go:build linux

package container

import (
	"os"
	"os/exec"
	"testing"
)

// TestNormalizeLoadedStateKeepsLiveContainer verifies that a container whose
// process is still alive is never downgraded to "stopped", even when namespace
// identities are missing from the persisted state. This was the root cause of
// containers being marked stopped (and later rm'd) while the real process tree
// kept running.
func TestNormalizeLoadedStateKeepsLiveContainer(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)
	os.Setenv("DCK_DATA_DIR", t.TempDir())

	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	defer func() { _ = cmd.Wait() }()

	c := &Container{Status: Running, PID: cmd.Process.Pid}
	normalizeLoadedState(c)
	if c.Status != Running {
		t.Fatalf("live container was downgraded to %q", c.Status)
	}
	if c.PID == 0 {
		t.Fatal("live container PID was zeroed")
	}
}

// TestNormalizeLoadedStateFlipsWhenAllProcessesDead verifies the stopped flip
// for states where every recorded process is gone.
func TestNormalizeLoadedStateFlipsWhenAllProcessesDead(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)
	os.Setenv("DCK_DATA_DIR", t.TempDir())

	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	c := &Container{Status: Running, PID: cmd.Process.Pid}
	normalizeLoadedState(c)
	if c.Status != Stopped {
		t.Fatalf("dead container status = %q, want %q", c.Status, Stopped)
	}
}

// TestFindChildPIDResolvesDirectChild verifies findChildPID returns the real
// direct child of a process without guessing or falling back to parent+1.
func TestFindChildPIDResolvesDirectChild(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	defer func() { _ = cmd.Wait() }()

	child := findChildPID(cmd.Process.Pid)
	if child <= 0 {
		t.Fatalf("findChildPID(%d) = %d, want > 0", cmd.Process.Pid, child)
	}
	if child == cmd.Process.Pid {
		t.Fatal("findChildPID returned the parent PID")
	}
}

// TestHasLiveProcesses verifies the live-process detection used by Remove() so
// stale "stopped" state can never leak a running process tree.
func TestHasLiveProcesses(t *testing.T) {
	none := &Container{ID: "none"}
	if none.hasLiveProcesses() {
		t.Fatal("container with no recorded processes reported live")
	}

	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	defer func() { _ = cmd.Wait() }()

	c := &Container{ID: "live", PID: cmd.Process.Pid}
	if !c.hasLiveProcesses() {
		t.Fatal("live PID not detected")
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if c.hasLiveProcesses() {
		t.Fatal("dead PID still reported live")
	}
}
