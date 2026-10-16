//go:build linux

package container

import "testing"

func TestValidateRuntimePolicyRejectsDangerousCaps(t *testing.T) {
	for _, cap := range []string{"SYS_ADMIN", "CAP_SYS_ADMIN", "sys_admin", "NET_ADMIN", "ALL"} {
		if err := ValidateRuntimePolicy([]string{cap}, "", false, false); err == nil {
			t.Errorf("dangerous cap %q accepted without acknowledgement", cap)
		}
	}
}

func TestValidateRuntimePolicyAckAllowsDangerousCaps(t *testing.T) {
	if err := ValidateRuntimePolicy([]string{"SYS_ADMIN"}, "", true, false); err != nil {
		t.Errorf("acknowledged dangerous cap rejected: %v", err)
	}
}

func TestValidateRuntimePolicySafeCapsPass(t *testing.T) {
	if err := ValidateRuntimePolicy([]string{"CHOWN", "NET_BIND_SERVICE"}, "", false, false); err != nil {
		t.Errorf("safe caps rejected: %v", err)
	}
}

func TestValidateRuntimePolicyRootUser(t *testing.T) {
	for _, u := range []string{"0", "root", "0:0", "0:1000", "root:1000"} {
		if err := ValidateRuntimePolicy(nil, u, false, false); err == nil {
			t.Errorf("root user %q accepted without acknowledgement", u)
		}
		if err := ValidateRuntimePolicy(nil, u, false, true); err != nil {
			t.Errorf("acknowledged root user %q rejected: %v", u, err)
		}
	}
	if err := ValidateRuntimePolicy(nil, "", false, false); err != nil {
		t.Errorf("default (empty) user rejected: %v", err)
	}
	if err := ValidateRuntimePolicy(nil, "1000", false, false); err != nil {
		t.Errorf("non-root user rejected: %v", err)
	}
}
