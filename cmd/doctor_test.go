//go:build linux

package cmd

import (
	"os"
	"testing"
)

func TestDiagnosticsFailed(t *testing.T) {
	checks := []diagnostic{
		{name: "ok", status: diagnosticOK},
		{name: "warning", status: diagnosticWarn},
	}
	if diagnosticsFailed(checks, false) {
		t.Fatal("warnings should not fail non-strict diagnostics")
	}
	if !diagnosticsFailed(checks, true) {
		t.Fatal("strict diagnostics should fail on warnings")
	}
	checks = append(checks, diagnostic{name: "failure", status: diagnosticFail})
	if !diagnosticsFailed(checks, false) {
		t.Fatal("diagnostics should fail when a check fails")
	}
}

func TestCheckAPIConfiguration(t *testing.T) {
	oldHost, oldToken := getenv("CARDINAL_HOST"), getenv("CARDINAL_TOKEN")
	t.Cleanup(func() {
		setenv("CARDINAL_HOST", oldHost)
		setenv("CARDINAL_TOKEN", oldToken)
	})

	setenv("CARDINAL_HOST", "0.0.0.0:2375")
	setenv("CARDINAL_TOKEN", "")
	checks := checkAPIConfiguration()
	if len(checks) != 1 || checks[0].status != diagnosticFail {
		t.Fatalf("external API without token = %#v, want one failure", checks)
	}

	setenv("CARDINAL_TOKEN", "test-token")
	checks = checkAPIConfiguration()
	if len(checks) != 1 || checks[0].status != diagnosticOK {
		t.Fatalf("external API with token = %#v, want one success", checks)
	}
}

func getenv(name string) string {
	value, _ := os.LookupEnv(name)
	return value
}

func setenv(name, value string) {
	if value == "" {
		_ = os.Unsetenv(name)
		return
	}
	_ = os.Setenv(name, value)
}
