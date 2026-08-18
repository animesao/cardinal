//go:build linux

package container

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"dck/internal/state"
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

// TestContainerInitCommandline verifies that the fallback only accepts the
// exact internal dck init command for the requested container.
func TestContainerInitCommandline(t *testing.T) {
	want := []byte("/usr/local/bin/dck\x00init\x00container-123\x00/var/lib/dck/overlay/merged\x00")
	if !isContainerInitCommandline(want, "container-123") {
		t.Fatal("valid dck init command line was not recognized")
	}
	if isContainerInitCommandline(want, "other-container") {
		t.Fatal("command line for another container was accepted")
	}
	if isContainerInitCommandline([]byte("/usr/local/bin/dck\\x00start\\x00container-123\\x00"), "container-123") {
		t.Fatal("non-init command line was accepted")
	}
}

func TestInitNetworkEnvironment(t *testing.T) {
	env := initNetworkEnvironment([]string{
		"PATH=/usr/bin",
		"DCK_INIT_READY_FD=99",
		"DCK_INIT_GO_FD=100",
	})
	if len(env) != 3 || env[0] != "PATH=/usr/bin" || env[1] != "DCK_INIT_READY_FD=3" || env[2] != "DCK_INIT_GO_FD=4" {
		t.Fatalf("unexpected network handshake environment: %#v", env)
	}
}

// TestContainerProcessAlive verifies the supervisor's exit-detection heuristic,
// including the PID-reuse guard for the unshare PID.
func TestContainerProcessAlive(t *testing.T) {
	if ContainerProcessAlive(&Container{ID: "none"}) {
		t.Fatal("container with no recorded processes reported alive")
	}

	// Legacy state: live init PID means alive.
	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	defer func() { _ = cmd.Wait() }()
	c := &Container{ID: "legacy-live", PID: cmd.Process.Pid}
	if !ContainerProcessAlive(c) {
		t.Fatal("live init PID not detected")
	}

	// Dead PID (reaped helper) means not alive.
	gone := exec.Command("sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	dead := &Container{ID: "legacy-dead", PID: gone.Process.Pid}
	if ContainerProcessAlive(dead) {
		t.Fatal("dead PID reported alive")
	}

	// A live unshare PID whose command line does not contain this container ID
	// (e.g. the PID was recycled by an unrelated process) must not count.
	other := exec.Command("sh", "-c", "sleep 30")
	if err := other.Start(); err != nil {
		t.Fatalf("start other helper: %v", err)
	}
	defer func() { _ = other.Process.Kill() }()
	defer func() { _ = other.Wait() }()
	mismatch := &Container{ID: "totally-different-id", UnsharePID: other.Process.Pid}
	if ContainerProcessAlive(mismatch) {
		t.Fatal("unrelated live process reported as container process")
	}
}

// TestPidAliveTreatsZombieAsDead verifies that a zombie (defunct) process is
// not reported alive. Detached containers are reparented to init once the CLI
// exits, so their corpses can linger until systemd reaps them; treating them as
// alive would stall exit detection and leave status stuck on "running".
func TestPidAliveTreatsZombieAsDead(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	defer func() { _ = cmd.Wait() }()

	if !pidAlive(cmd.Process.Pid) {
		t.Fatal("live process reported dead")
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	// The helper is now a zombie until the test reaps it via Wait.
	deadline := time.Now().Add(2 * time.Second)
	for pidAlive(cmd.Process.Pid) {
		if time.Now().After(deadline) {
			t.Fatal("zombie process still reported alive")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRemovalTombstone verifies that the removal marker written by Remove()
// is visible to the supervisor so an automatic restart cannot resurrect a
// container mid-removal.
func TestRemovalTombstone(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)
	os.Setenv("DCK_DATA_DIR", t.TempDir())
	if err := state.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	c := &Container{ID: "tombstone-test"}
	if IsBeingRemoved(c.ID) {
		t.Fatal("marker reported before any removal")
	}

	marker := removalMarkerPath(c.ID)
	if err := os.WriteFile(marker, []byte("removing\n"), 0600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if !IsBeingRemoved(c.ID) {
		t.Fatal("marker not detected")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if IsBeingRemoved(c.ID) {
		t.Fatal("marker still reported after cleanup")
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
