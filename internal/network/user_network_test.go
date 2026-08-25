package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateListAndAllocateUserNetwork(t *testing.T) {
	t.Setenv("CARDINAL_DATA_DIR", t.TempDir())

	n, err := CreateNetwork("testnet", "10.33.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}
	if n.Subnet != "10.33.0.0/24" || n.Gateway != "10.33.0.1" {
		t.Fatalf("unexpected network addressing: %+v", n)
	}
	if n.Bridge == "" || n.ID == "" {
		t.Fatalf("network identity was not generated: %+v", n)
	}

	listed, err := ListNetworks()
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "testnet" {
		t.Fatalf("ListNetworks() = %+v, want one testnet", listed)
	}

	first, err := AllocateNetworkIP("testnet")
	if err != nil {
		t.Fatalf("AllocateNetworkIP() error = %v", err)
	}
	second, err := AllocateNetworkIP("testnet")
	if err != nil {
		t.Fatalf("second AllocateNetworkIP() error = %v", err)
	}
	if first == second || first != "10.33.0.2" || second != "10.33.0.3" {
		t.Fatalf("allocated IPs = %q, %q", first, second)
	}

	if err := RemoveNetwork("testnet"); err == nil {
		t.Fatal("RemoveNetwork() accepted a network with allocated addresses")
	}
	ReleaseNetworkIP("testnet", first)
	ReleaseNetworkIP("testnet", second)
	if err := RemoveNetwork("testnet"); err != nil {
		t.Fatalf("RemoveNetwork() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(t.TempDir(), "unused")); !os.IsNotExist(err) {
		t.Fatalf("unexpected temporary path state: %v", err)
	}
}

func TestCreateNetworkValidation(t *testing.T) {
	t.Setenv("CARDINAL_DATA_DIR", t.TempDir())
	for _, name := range []string{"", "bridge", "bad/name", "bad name"} {
		if _, err := CreateNetwork(name, "10.34.0.0/24"); err == nil {
			t.Errorf("CreateNetwork(%q) unexpectedly succeeded", name)
		}
	}
	if _, err := CreateNetwork("small", "10.35.0.0/30"); err == nil {
		t.Fatal("CreateNetwork() accepted an unusable /30 subnet")
	}
}
