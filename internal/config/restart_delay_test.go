package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadComposeRestartDelay(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "service field",
			yaml: "services:\n  app:\n    image: alpine:latest\n    restart: unless-stopped\n    restart_delay: 1m\n",
			want: "1m",
		},
		{
			name: "deploy policy delay",
			yaml: "services:\n  app:\n    image: alpine:latest\n    deploy:\n      restart_policy:\n        condition: on-failure\n        delay: 30s\n",
			want: "30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "compose.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadCompose(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Container["app"].RestartDelay; got != tt.want {
				t.Fatalf("RestartDelay = %q, want %q", got, tt.want)
			}
		})
	}
}
