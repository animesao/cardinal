//go:build linux

package container

import "testing"

func TestRequestedCapabilitiesUsesSafeDefaults(t *testing.T) {
	keep, err := requestedCapabilities(&Container{})
	if err != nil {
		t.Fatalf("requestedCapabilities: %v", err)
	}
	for _, name := range []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETUID", "SETGID", "NET_BIND_SERVICE"} {
		if !keep[name] {
			t.Errorf("default capabilities do not include %s", name)
		}
	}
	if keep["SYS_ADMIN"] || keep["SYS_MODULE"] {
		t.Fatal("dangerous capabilities were enabled by default")
	}
}

func TestRequestedCapabilitiesCanDropAll(t *testing.T) {
	keep, err := requestedCapabilities(&Container{CapDrop: []string{"ALL"}})
	if err != nil {
		t.Fatalf("requestedCapabilities: %v", err)
	}
	if len(keep) != 0 {
		t.Fatalf("cap-drop ALL retained capabilities: %#v", keep)
	}
}

func TestRequestedCapabilitiesAddsExplicitCapability(t *testing.T) {
	keep, err := requestedCapabilities(&Container{CapAdd: []string{"SYS_PTRACE"}})
	if err != nil {
		t.Fatalf("requestedCapabilities: %v", err)
	}
	if !keep["SYS_PTRACE"] {
		t.Fatal("explicit SYS_PTRACE was not retained")
	}
}
