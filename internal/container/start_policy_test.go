//go:build linux

package container

import "testing"

func TestShouldRestart(t *testing.T) {
	tests := []struct {
		name          string
		policy        string
		exitCode      int
		stoppedByUser bool
		want          bool
	}{
		{name: "no never restarts", policy: "no", exitCode: 1, want: false},
		{name: "always restarts after success", policy: "always", exitCode: 0, want: true},
		{name: "always restarts after manual flag", policy: "always", exitCode: 1, stoppedByUser: true, want: true},
		{name: "on failure restarts on error", policy: "on-failure", exitCode: 1, want: true},
		{name: "on failure does not restart on success", policy: "on-failure", exitCode: 0, want: false},
		{name: "unless stopped restarts after crash", policy: "unless-stopped", exitCode: 1, want: true},
		{name: "unless stopped stays stopped after user stop", policy: "unless-stopped", exitCode: 1, stoppedByUser: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRestart(tt.policy, tt.exitCode, tt.stoppedByUser); got != tt.want {
				t.Fatalf("shouldRestart(%q, %d, %v) = %v, want %v", tt.policy, tt.exitCode, tt.stoppedByUser, got, tt.want)
			}
		})
	}
}
