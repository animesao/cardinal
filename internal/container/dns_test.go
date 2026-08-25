//go:build linux

package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDNSRegistry(t *testing.T) {
	// Override DNS registry path to temp dir
	tmpDir := t.TempDir()
	orig := dnsRegistry.path
	dnsRegistry.path = filepath.Join(tmpDir, "dns-registry.json")
	defer func() { dnsRegistry.path = orig }()

	// Register
	RegisterDNSName("web", "10.0.2.100")
	RegisterDNSName("db", "10.0.2.101")

	// Resolve
	if ip := ResolveDNSName("web"); ip != "10.0.2.100" {
		t.Errorf("ResolveDNSName('web') = %q, want '10.0.2.100'", ip)
	}
	if ip := ResolveDNSName("db"); ip != "10.0.2.101" {
		t.Errorf("ResolveDNSName('db') = %q, want '10.0.2.101'", ip)
	}
	if ip := ResolveDNSName("unknown"); ip != "" {
		t.Errorf("ResolveDNSName('unknown') = %q, want ''", ip)
	}

	// List
	entries := ListDNSNames()
	if len(entries) != 2 {
		t.Errorf("ListDNSNames count = %d, want 2", len(entries))
	}

	// Unregister
	UnregisterDNSName("web")
	if ip := ResolveDNSName("web"); ip != "" {
		t.Errorf("After unregister, ResolveDNSName('web') = %q, want ''", ip)
	}
	if ip := ResolveDNSName("db"); ip != "10.0.2.101" {
		t.Errorf("After unregister db, ResolveDNSName('db') = %q, want '10.0.2.101'", ip)
	}
}

func TestDNSRegistryPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	orig := dnsRegistry.path
	dnsRegistry.path = filepath.Join(tmpDir, "dns-registry.json")
	defer func() { dnsRegistry.path = orig }()

	RegisterDNSName("cache", "10.0.2.200")

	// Create a new registry instance to test persistence
	newRegistry := &DNSRegistry{path: dnsRegistry.path}
	entries := newRegistry.read()
	if len(entries) != 1 {
		t.Errorf("Persisted count = %d, want 1", len(entries))
	}
	if entries[0].Name != "cache" || entries[0].IP != "10.0.2.200" {
		t.Errorf("Persisted entry = %+v, want {cache 10.0.2.200}", entries[0])
	}
}

func TestEnsureContainerHosts(t *testing.T) {
	tmpDir := t.TempDir()
	orig := dnsRegistry.path
	dnsRegistry.path = filepath.Join(tmpDir, "dns-registry.json")
	defer func() { dnsRegistry.path = orig }()

	mergedDir := filepath.Join(tmpDir, "merged")
	os.MkdirAll(filepath.Join(mergedDir, "etc"), 0755)
	if err := os.WriteFile(filepath.Join(mergedDir, "etc", "hosts"), []byte("127.0.0.1 localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}

	RegisterDNSName("app1", "10.0.2.10")
	RegisterDNSName("app2", "10.0.2.11")

	EnsureContainerHosts(mergedDir, "app1", "10.0.2.10", []string{"8.8.8.8"})

	// Check that /etc/hosts was updated
	data, _ := os.ReadFile(filepath.Join(mergedDir, "etc", "hosts"))
	content := string(data)
	if !strContains(content, "10.0.2.11") {
		t.Error("/etc/hosts should contain app2 IP")
	}
	if !strContains(content, "app2") {
		t.Error("/etc/hosts should contain app2 name")
	}
	if strContains(content, "app1") {
		t.Error("/etc/hosts should NOT contain own name")
	}
	if !strContains(content, "# cardinal-managed") {
		t.Error("/etc/hosts should contain cardinal-managed marker")
	}

	// Check resolv.conf
	resolvData, _ := os.ReadFile(filepath.Join(mergedDir, "etc", "resolv.conf"))
	if !strContains(string(resolvData), "8.8.8.8") {
		t.Error("resolv.conf should contain DNS server")
	}
}

func strContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
