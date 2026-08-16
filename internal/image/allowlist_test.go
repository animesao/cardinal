//go:build linux

package image

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowlist_DefaultIncludesDockerHub(t *testing.T) {
	got := AllowlistSnapshot()
	want := "registry-1.docker.io"
	found := false
	for _, h := range got {
		if h == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default allowlist to include %s; got %v", want, got)
	}
}

func TestAllowlist_StrictModeBlocksOffList(t *testing.T) {
	t.Setenv("DCK_REGISTRY_STRICT", "1")
	t.Setenv("DCK_DATA_DIR", t.TempDir())
	if err := InitRegistryAllowlist(); err != nil {
		t.Fatalf("InitRegistryAllowlist: %v", err)
	}
	if !IsRegistryAllowed("https://registry-1.docker.io/v2/") {
		t.Errorf("registry-1.docker.io must be allowed by default")
	}
	if IsRegistryAllowed("https://attacker.example.com/v2/") {
		t.Errorf("attacker.example.com must be rejected in strict mode")
	}
}

func TestAllowlist_PermissiveModeLetsEverything(t *testing.T) {
	t.Setenv("DCK_REGISTRY_STRICT", "0")
	if err := InitRegistryAllowlist(); err != nil {
		t.Fatalf("InitRegistryAllowlist: %v", err)
	}
	if !IsRegistryAllowed("https://attacker.example.com/v2/") {
		t.Errorf("with DCK_REGISTRY_STRICT=0 the gate should be permissive")
	}
}

func TestAllowlist_AddRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCK_DATA_DIR", dir)
	t.Setenv("DCK_REGISTRY_STRICT", "1")
	if err := InitRegistryAllowlist(); err != nil {
		t.Fatalf("InitRegistryAllowlist: %v", err)
	}
	if err := AddToAllowlist("trusted.example.com"); err != nil {
		t.Fatalf("AddToAllowlist: %v", err)
	}
	if !IsRegistryAllowed("trusted.example.com") {
		t.Errorf("expected trusted.example.com to be allowed after AddToAllowlist")
	}
	if err := RemoveFromAllowlist("trusted.example.com"); err != nil {
		t.Fatalf("RemoveFromAllowlist: %v", err)
	}
	if IsRegistryAllowed("trusted.example.com") {
		t.Errorf("expected trusted.example.com to be denied after RemoveFromAllowlist")
	}
	// File should exist so operator can grep.
	if _, err := os.Stat(filepath.Join(dir, "registry-allowlist")); err != nil {
		t.Errorf("persistent allowlist file missing after RemoveFromAllowlist: %v", err)
	}
}

func TestAllowlist_InsecureRegistryDefaultDenied(t *testing.T) {
	t.Setenv("DCK_ALLOW_INSECURE_REGISTRY", "")
	if IsInsecureRegistryAllowed() {
		t.Errorf("insecure registry must be denied by default")
	}
	t.Setenv("DCK_ALLOW_INSECURE_REGISTRY", "yes")
	if !IsInsecureRegistryAllowed() {
		t.Errorf("DCK_ALLOW_INSECURE_REGISTRY=yes should opt in")
	}
}

func TestNormalizeRegistryHostname(t *testing.T) {
	cases := map[string]string{
		"https://docker.io":            "docker.io",
		"http://localhost:5000":        "localhost:5000",
		"ghcr.io/v2/":                  "ghcr.io",
		"docker.io":                    "docker.io",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeRegistryHostname(in); got != want {
			t.Errorf("normalizeRegistryHostname(%q) = %q, want %q", in, got, want)
		}
	}
}
