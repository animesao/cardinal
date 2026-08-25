//go:build linux

package container

import (
	"os"
	"path/filepath"
	"testing"
)

// withTestAllowlist swaps the runtime allowlist for a deterministic slice
// during a test, then restores the production value on cleanup.
func withTestAllowlist(t *testing.T, paths []string) {
	t.Helper()
	protectedMu.Lock()
	prev := allowedHostPaths
	allowedHostPaths = append([]string(nil), paths...)
	hostPolicyInited = true
	protectedMu.Unlock()
	t.Cleanup(func() {
		protectedMu.Lock()
		allowedHostPaths = prev
		protectedMu.Unlock()
	})
}

func TestIsProtectedHostPath_SystemDirsAreBlocked(t *testing.T) {
	// Touch a known-good real dir to drive filepath.Abs through a stable path.
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "fake-etc"), 0o755)

	cases := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/etc", true},
		{"/etc/passwd", true},
		{"/var/run/cardinal", true},
		{"/var/run/docker.sock", true},
		{"/root/.ssh", true},
		{"/home", true},
		{"/opt", true},
		// Anything outside the blocklist is not protected by default.
		{"/data", false},
		{"/srv/cardinal", false},
	}
	for _, c := range cases {
		if got := IsProtectedHostHostPathForTest(c.path); got != c.want {
			t.Errorf("IsProtectedHostPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// IsProtectedHostHostPathForTest is a thin wrapper that pinpoints the
// production symbol under test — protects against accidental shadowing
// during refactors.
func IsProtectedHostHostPathForTest(p string) bool { return IsProtectedHostPath(p) }

func TestIsProtectedHostPath_AllowlistOverridesBlocklist(t *testing.T) {
	// /etc/by-allowlist should be permitted via the env-driven allowlist.
	t.Setenv("CARDINAL_ALLOWED_HOST_PATHS", "/etc:/var/run/cardinal")
	// Reset both the global allowlist (which InitHostPathPolicy will
	// append to) and the hostPolicyInited flag (so InitHostPathPolicy
	// will re-read env). t.Cleanup restores the previous state so
	// subsequent tests (TestBindSourceValidation, etc.) run against
	// the production default blocklist.
	t.Cleanup(func() {
		protectedMu.Lock()
		allowedHostPaths = nil
		hostPolicyInited = false
		protectedMu.Unlock()
	})
	InitHostPathPolicy()

	if IsProtectedHostPath("/etc/foo") {
		t.Errorf("expected /etc/foo to be allowed; got protected")
	}
	if !IsProtectedHostPath("/var/foo") {
		t.Errorf("expected /var/foo to remain protected (only /var/run/cardinal allowed)")
	}
}

func TestIsProtectedHostPath_RejectsRelative(t *testing.T) {
	if !IsProtectedHostPath("etc/passwd") {
		t.Errorf("relative paths must be flagged as protected (never mount)")
	}
	if !IsProtectedHostPath("") {
		t.Errorf("empty path must be flagged as protected")
	}
}
