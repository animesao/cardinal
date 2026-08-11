//go:build linux

package container

import (
	"testing"
	"time"
)

func TestRestartDelay(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "default delay", want: time.Second},
		{name: "seconds", value: "10s", want: 10 * time.Second},
		{name: "minute", value: "1m", want: time.Minute},
		{name: "invalid falls back", value: "not-a-duration", want: time.Second},
		{name: "zero falls back", value: "0s", want: time.Second},
		{name: "negative falls back", value: "-1s", want: time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{RestartDelay: tt.value}
			if got := c.restartDelay(); got != tt.want {
				t.Fatalf("restartDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}
