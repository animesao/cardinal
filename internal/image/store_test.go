package image

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"dck/internal/state"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantTag  string
	}{
		{"nginx", "library/nginx", "latest"},
		{"nginx:1.21", "library/nginx", "1.21"},
		{"myrepo/myapp:v2", "myrepo/myapp", "v2"},
		{"alpine:3.18", "library/alpine", "3.18"},
		{"registry.example.com/app:1.0", "registry.example.com/app", "1.0"},
	}
	for _, tt := range tests {
		name, tag := parseRef(tt.input)
		if name != tt.wantName {
			t.Errorf("parseRef(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
		if tag != tt.wantTag {
			t.Errorf("parseRef(%q) tag = %q, want %q", tt.input, tag, tt.wantTag)
		}
	}
}

func TestSaveAndLoadFromStore(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("DCK_DATA_DIR")
	os.Setenv("DCK_DATA_DIR", tmpDir)
	defer os.Setenv("DCK_DATA_DIR", origDir)

	img := &Image{Name: "nginx", Tag: "latest", Digest: "sha256:abc123"}

	if err := SaveToStore(img); err != nil {
		t.Fatalf("SaveToStore() error: %v", err)
	}

	loaded := LoadFromStore("nginx", "latest")
	if loaded == nil {
		t.Fatal("LoadFromStore returned nil")
	}
	if loaded.Name != "nginx" || loaded.Tag != "latest" || loaded.Digest != "sha256:abc123" {
		t.Errorf("Loaded = %+v, want {nginx latest sha256:abc123}", loaded)
	}

	notFound := LoadFromStore("nonexistent", "latest")
	if notFound != nil {
		t.Error("LoadFromStore for nonexistent should return nil")
	}
}

func TestListImages(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("DCK_DATA_DIR")
	os.Setenv("DCK_DATA_DIR", tmpDir)
	defer os.Setenv("DCK_DATA_DIR", origDir)

	images, err := ListImages()
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("ListImages() should be empty initially, got %d", len(images))
	}

	if err := SaveToStore(&Image{Name: "library/alpine", Tag: "latest"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveToStore(&Image{Name: "library/nginx", Tag: "1.21"}); err != nil {
		t.Fatal(err)
	}

	images, err = ListImages()
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("ListImages() count = %d, want 2", len(images))
	}
}

func TestRemoveImage(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("DCK_DATA_DIR")
	os.Setenv("DCK_DATA_DIR", tmpDir)
	defer os.Setenv("DCK_DATA_DIR", origDir)

	img := &Image{Name: "test-img", Tag: "latest"}
	if err := SaveToStore(img); err != nil {
		t.Fatal(err)
	}

	if err := RemoveImage("test-img", "latest"); err != nil {
		t.Fatalf("RemoveImage() error: %v", err)
	}

	loaded := LoadFromStore("test-img", "latest")
	if loaded != nil {
		t.Error("Image should be removed")
	}
}

func TestVerifyDigest(t *testing.T) {
	data := []byte("layer bytes")
	hash := sha256.Sum256(data)
	good := "sha256:" + hex.EncodeToString(hash[:])
	if err := verifyDigest(data, good); err != nil {
		t.Fatalf("verifyDigest(valid): %v", err)
	}
	if err := verifyDigest(data, "sha256:"+strings.Repeat("0", 64)); err == nil {
		t.Fatal("verifyDigest(invalid) unexpectedly succeeded")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("DCK_DATA_DIR")
	os.Setenv("DCK_DATA_DIR", tmpDir)
	defer os.Setenv("DCK_DATA_DIR", origDir)

	imgDir := state.ImageDir("myimg", "latest")
	os.MkdirAll(imgDir, 0755)

	configData := []byte(`{"config":{"Cmd":["nginx"],"WorkingDir":"/etc/nginx"}}`)
	if err := saveConfig(imgDir, configData); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	cfg, err := ReadConfig("myimg", "latest")
	if err != nil {
		t.Fatalf("ReadConfig() error: %v", err)
	}
	if len(cfg.Config.Cmd) != 1 || cfg.Config.Cmd[0] != "nginx" {
		t.Errorf("Config Cmd = %v, want [nginx]", cfg.Config.Cmd)
	}
}
