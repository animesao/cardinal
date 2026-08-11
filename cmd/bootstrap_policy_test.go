//go:build linux

package cmd

import "testing"

func TestShouldBootstrap(t *testing.T) {
	tests := []struct {
		name          string
		policy        string
		stoppedByUser bool
		want          bool
	}{
		{name: "always starts after reboot", policy: "always", stoppedByUser: true, want: true},
		{name: "unless stopped starts when not manually stopped", policy: "unless-stopped", want: true},
		{name: "unless stopped stays stopped after manual stop", policy: "unless-stopped", stoppedByUser: true, want: false},
		{name: "on failure is not a boot policy", policy: "on-failure", want: false},
		{name: "no is not a boot policy", policy: "no", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBootstrap(tt.policy, tt.stoppedByUser); got != tt.want {
				t.Fatalf("shouldBootstrap(%q, %v) = %v, want %v", tt.policy, tt.stoppedByUser, got, tt.want)
			}
		})
	}
}
