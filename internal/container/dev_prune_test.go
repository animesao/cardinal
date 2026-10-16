//go:build linux

package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequestedCapabilitiesDropMknodByDefault(t *testing.T) {
	keep, err := requestedCapabilities(&Container{})
	if err != nil {
		t.Fatalf("requestedCapabilities: %v", err)
	}
	if keep["MKNOD"] {
		t.Fatal("MKNOD must not be in the default set (device nodes need explicit --cap-add MKNOD)")
	}
}

func TestRequestedCapabilitiesMknodExplicitGrant(t *testing.T) {
	keep, err := requestedCapabilities(&Container{CapAdd: []string{"MKNOD"}})
	if err != nil {
		t.Fatalf("requestedCapabilities: %v", err)
	}
	if !keep["MKNOD"] {
		t.Fatal("explicit MKNOD grant was not retained")
	}
}

func TestPruneDevtmpfsKeepsAllowlist(t *testing.T) {
	dev := t.TempDir()
	for _, name := range []string{"null", "zero", "sda", "sda1", "kvm", "fuse", "net"} {
		if err := os.MkdirAll(filepath.Join(dev, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneDevtmpfs(dev); err != nil {
		t.Fatalf("PruneDevtmpfs: %v", err)
	}
	for _, name := range []string{"null", "zero"} {
		if _, err := os.Stat(filepath.Join(dev, name)); err != nil {
			t.Errorf("allowlisted %s was pruned: %v", name, err)
		}
	}
	for _, name := range []string{"sda", "sda1", "kvm", "fuse", "net"} {
		if _, err := os.Stat(filepath.Join(dev, name)); !os.IsNotExist(err) {
			t.Errorf("host device %s survived pruning", name)
		}
	}
}
