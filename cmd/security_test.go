//go:build linux

package cmd

import (
	"strings"
	"testing"
)

func TestValidateDangerousRuntimeOptions_BlocksSYS_ADMIN(t *testing.T) {
	// Without --allow-dangerous-caps: must reject.
	good := validateDangerousRuntimeOptions([]string{"SYS_ADMIN"}, "", false, false)
	if good {
		t.Errorf("expected SYS_ADMIN to be rejected without --allow-dangerous-caps")
	}
	// With --allow-dangerous-caps: must accept.
	good = validateDangerousRuntimeOptions([]string{"SYS_ADMIN"}, "", true, false)
	if !good {
		t.Errorf("expected SYS_ADMIN to be accepted with --allow-dangerous-caps")
	}
}

func TestValidateDangerousRuntimeOptions_NormalCapsAllowed(t *testing.T) {
	if !validateDangerousRuntimeOptions([]string{"CHOWN", "DAC_OVERRIDE"}, "", false, false) {
		t.Errorf("non-dangerous caps must not trigger the gate")
	}
}

func TestValidateDangerousRuntimeOptions_CapPrefix(t *testing.T) {
	if !validateDangerousRuntimeOptions([]string{"cap_sys_admin"}, "", true, false) {
		t.Errorf("CAP_ prefix must be normalized and detected as dangerous")
	}
}

func TestIsRootUser(t *testing.T) {
	yes := []string{"root", "0", "0:0", "0:1000", "root:0"}
	no := []string{"", "1000", "1000:1000", "nobody", "100:200"}
	for _, v := range yes {
		if !isRootUser(v) {
			t.Errorf("isRootUser(%q) = false; want true", v)
		}
	}
	for _, v := range no {
		if isRootUser(v) {
			t.Errorf("isRootUser(%q) = true; want false", v)
		}
	}
}

func TestDangerousCapsRequested(t *testing.T) {
	got := dangerousCapsRequested([]string{"chown", "SYS_ADMIN", "Net_Admin", "DAC_OVERRIDE", "cap_sys_module"})
	want := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_MODULE"}
	if !equalSet(got, want) {
		t.Errorf("dangerousCapsRequested = %v; want %v", got, want)
	}
}

func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := 0
	for _, x := range a {
		for _, y := range b {
			if strings.EqualFold(x, y) {
				count++
				break
			}
		}
	}
	return count == len(a)
}
