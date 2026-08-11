//go:build linux

package container

import (
	"os"
	"os/exec"
	"testing"

	"dck/internal/image"
)

func TestContainerLifecycleE2E(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount and namespace setup")
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("requires unshare")
	}
	if _, err := exec.LookPath("mount"); err != nil {
		t.Skip("requires mount")
	}

	ref := os.Getenv("DCK_E2E_IMAGE")
	if ref == "" {
		t.Skip("set DCK_E2E_IMAGE to an already pulled image to run lifecycle e2e")
	}
	name, tag := parseE2EReference(ref)
	img := image.LoadFromStore(name, tag)
	if img == nil {
		t.Skipf("image %s:%s is not present locally", name, tag)
	}

	// Use the configured dck data directory so the already-pulled image and its
	// rootfs remain available to the runtime. The generated container is removed
	// during cleanup.
	c := New(img, CreateOpts{
		Name:        "lifecycle-e2e",
		Cmd:         []string{"/bin/sh", "-c", "printf lifecycle-ok"},
		NetworkMode: "none",
	})
	t.Cleanup(func() {
		_ = c.Remove(true)
	})
	if err := c.Save(); err != nil {
		t.Fatalf("save container: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("start container: %v", err)
	}
	if c.Status != Stopped {
		t.Fatalf("status after foreground command = %q, want %q", c.Status, Stopped)
	}
	if output, err := os.ReadFile(c.LogFile()); err == nil && !containsBytes(output, []byte("lifecycle-ok")) {
		t.Fatalf("lifecycle output does not contain lifecycle-ok: %q", output)
	}
}

func parseE2EReference(ref string) (string, string) {
	name, tag := ref, "latest"
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == ':' {
			name, tag = ref[:i], ref[i+1:]
			break
		}
		if ref[i] == '/' {
			break
		}
	}
	if len(name) > 0 && !containsSlash(name) {
		name = "library/" + name
	}
	return name, tag
}

func containsBytes(value, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(value); i++ {
		match := true
		for j := range needle {
			if value[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func containsSlash(value string) bool {
	for _, r := range value {
		if r == '/' {
			return true
		}
	}
	return false
}
