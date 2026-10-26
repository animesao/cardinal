package network

import (
	"reflect"
	"testing"
)

func TestLocalhostMasqueradeRuleStaysScoped(t *testing.T) {
	for _, subnet := range []string{BridgeCIDR, "10.33.0.0/24"} {
		got := localhostMasqueradeRule(subnet)
		want := []string{"-s", "127.0.0.0/8", "-d", subnet, "-j", "MASQUERADE"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("localhostMasqueradeRule(%q) = %v, want %v", subnet, got, want)
		}
	}
}

func TestLocalhostInputGuardsBlockSpoofingAndNewConnections(t *testing.T) {
	got := localhostInputGuards(BridgeName)
	want := [][]string{
		{"-i", BridgeName, "-s", "127.0.0.0/8",
			"-m", "conntrack", "!", "--ctstate", "ESTABLISHED,RELATED",
			"-j", "DROP"},
		{"-i", BridgeName, "-d", "127.0.0.0/8",
			"-m", "conntrack", "!", "--ctstate", "ESTABLISHED,RELATED",
			"-j", "DROP"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("localhostInputGuards(%q) = %v, want %v", BridgeName, got, want)
	}
}
