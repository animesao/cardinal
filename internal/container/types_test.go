//go:build linux

package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseMemoryString(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"512m", 512 * 1024 * 1024},
		{"1g", 1 * 1024 * 1024 * 1024},
		{"256M", 256 * 1024 * 1024},
		{"", 0},
		{"100", 100},
	}
	for _, tt := range tests {
		got, err := ParseMemoryString(tt.input)
		if err != nil {
			t.Errorf("ParseMemoryString(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseMemoryString(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}

	_, err := ParseMemoryString("invalid")
	if err == nil {
		t.Error("ParseMemoryString('invalid') should error")
	}
}

func TestLogFile(t *testing.T) {
	c := &Container{ID: "test123"}
	path := c.LogFile()
	if path == "" {
		t.Error("LogFile() returned empty")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("LogFile() = %q, want absolute path", path)
	}
}

func TestParseVolumeString(t *testing.T) {
	tests := []struct {
		input string
		src   string
		dst   string
	}{
		{"/host/path:/container/path", "/host/path", "/container/path"},
		{"named:/path", "named", "/path"},
	}
	for _, tt := range tests {
		got := ParseVolumeString(tt.input)
		if got.Source != tt.src {
			t.Errorf("ParseVolumeString(%q).Source = %q, want %q", tt.input, got.Source, tt.src)
		}
		if got.Target != tt.dst {
			t.Errorf("ParseVolumeString(%q).Target = %q, want %q", tt.input, got.Target, tt.dst)
		}
	}
}

func TestVolumeNameValidation(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", "nested/name"} {
		if err := validateVolumeName(name); err == nil {
			t.Errorf("validateVolumeName(%q) unexpectedly succeeded", name)
		}
	}
	if err := validateVolumeName("safe-volume"); err != nil {
		t.Fatalf("validateVolumeName(safe-volume): %v", err)
	}
}

func TestContainerTargetValidation(t *testing.T) {
	for _, target := range []string{"", "relative", "/", "/../escape", `/\\escape`} {
		if err := validateContainerTarget(target); err == nil {
			t.Errorf("validateContainerTarget(%q) unexpectedly succeeded", target)
		}
	}
	if err := validateContainerTarget("/var/lib/app"); err != nil {
		t.Fatalf("validateContainerTarget(/var/lib/app): %v", err)
	}
}

func TestBindSourceValidation(t *testing.T) {
	if _, err := validateBindSource("/etc"); err == nil {
		t.Fatal("validateBindSource(/etc) unexpectedly succeeded")
	}
	root := t.TempDir()
	if resolved, err := validateBindSource(root); err != nil || resolved != root {
		t.Fatalf("validateBindSource(%q) = %q, %v", root, resolved, err)
	}
}

func TestRemoveKeepsNamedVolume(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	tmpDir := t.TempDir()
	if err := os.Setenv("DCK_DATA_DIR", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("DCK_DATA_DIR", origDataDir)

	vol, err := CreateVolume("persistent", "local", nil, nil)
	if err != nil {
		t.Fatalf("CreateVolume: %v", err)
	}
	c := &Container{
		ID:      "remove-keeps-volume",
		Status:  Stopped,
		Volumes: []VolumeMount{{Source: vol.Name, Target: "/data"}},
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save container: %v", err)
	}
	if err := c.Remove(false); err != nil {
		t.Fatalf("remove container: %v", err)
	}
	if _, err := os.Stat(vol.Mountpoint); err != nil {
		t.Fatalf("named volume was removed with container: %v", err)
	}
}

func TestNeedsNetwork(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"", true},
		{"bridge", true},
		{"host", false},
		{"none", false},
	}
	for _, tt := range tests {
		c := &Container{NetworkMode: tt.mode}
		got := c.NeedsNetwork()
		if got != tt.want {
			t.Errorf("NeedsNetwork() with mode %q = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Save the original data dir
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)

	tmpDir := t.TempDir()
	os.Setenv("DCK_DATA_DIR", tmpDir)

	c := &Container{
		ID:        "test-save-load",
		Name:      "test-container",
		Status:    Created,
		ImageName: "nginx",
		ImageTag:  "latest",
		Ports: []PortMap{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
	}

	if err := c.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load("test-save-load")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Name != c.Name {
		t.Errorf("Loaded Name = %q, want %q", loaded.Name, c.Name)
	}
	if loaded.ImageName != c.ImageName {
		t.Errorf("Loaded ImageName = %q, want %q", loaded.ImageName, c.ImageName)
	}
	if len(loaded.Ports) != len(c.Ports) {
		t.Errorf("Loaded Ports length = %d, want %d", len(loaded.Ports), len(c.Ports))
	}
}

func TestNormalizeLoadedStateRejectsLegacyRunningState(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)
	os.Setenv("DCK_DATA_DIR", t.TempDir())

	// A legacy state whose init process is gone (and no unshare PID is
	// recorded) is truly dead and must be downgraded to stopped.
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	c := &Container{Status: Running, PID: cmd.Process.Pid}
	normalizeLoadedState(c)
	if c.Status != Stopped {
		t.Fatalf("legacy running state status = %q, want %q", c.Status, Stopped)
	}
	if c.PID != 0 || c.MountNamespace != 0 {
		t.Fatalf("legacy running state retained runtime identity: pid=%d namespace=%d", c.PID, c.MountNamespace)
	}
}

func TestContainerState(t *testing.T) {
	c := &Container{ID: "test-state", Status: Created}
	if c.Status != Created {
		t.Errorf("Status should be Created initially")
	}

	c.Status = Running
	if c.Status != Running {
		t.Errorf("Status should be Running after change")
	}

	c.Status = Stopped
	if c.Status != Stopped {
		t.Errorf("Status should be Stopped after change")
	}
}

func TestPortMap(t *testing.T) {
	p := PortMap{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
	}
	if p.HostPort != 8080 {
		t.Errorf("HostPort = %d", p.HostPort)
	}
	if p.ContainerPort != 80 {
		t.Errorf("ContainerPort = %d", p.ContainerPort)
	}
	if p.Protocol != "tcp" {
		t.Errorf("Protocol = %s", p.Protocol)
	}
}

func TestFindByName(t *testing.T) {
	origDataDir := os.Getenv("DCK_DATA_DIR")
	defer os.Setenv("DCK_DATA_DIR", origDataDir)

	tmpDir := t.TempDir()
	os.Setenv("DCK_DATA_DIR", tmpDir)

	c := &Container{
		ID:   "findme",
		Name: "find-me-by-name",
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	found := FindByName("find-me-by-name")
	if found == nil {
		t.Fatal("FindByName returned nil")
	}
	if found.ID != "findme" {
		t.Errorf("Found ID = %s, want findme", found.ID)
	}

	notFound := FindByName("nonexistent")
	if notFound != nil {
		t.Error("FindByName for nonexistent should return nil")
	}
}
