package network

import (
	"reflect"
	"testing"
)

func TestLocalhostInputGuardOnlyAllowsTrackedReplies(t *testing.T) {
	got := localhostInputGuard(BridgeName)
	want := []string{
		"-i", BridgeName, "-d", "127.0.0.0/8",
		"-m", "conntrack", "!", "--ctstate", "ESTABLISHED,RELATED",
		"-j", "DROP",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("localhostInputGuard(%q) = %v, want %v", BridgeName, got, want)
	}
}
