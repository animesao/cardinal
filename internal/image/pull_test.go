package image

import (
	"context"
	"testing"
)

func TestAllowedTokenHost(t *testing.T) {
	cases := []struct {
		registry, realm string
		want            bool
	}{
		{"ghcr.io", "ghcr.io", true},
		{"registry-1.docker.io", "auth.docker.io", true},
		{"registry-1.docker.io", "registry-1.docker.io", true},
		{"docker.io", "auth.docker.io", true},
		{"ghcr.io", "evil.example.com", false},
		{"registry-1.docker.io", "evil.example.com", false},
		{"registry-1.docker.io", "auth.docker.io.evil.example.com", false},
		{"my-reg.local", "my-reg.local", true},
		{"my-reg.local", "other.local", false},
	}
	for _, tc := range cases {
		if got := allowedTokenHost(tc.registry, tc.realm); got != tc.want {
			t.Errorf("allowedTokenHost(%q, %q) = %v, want %v", tc.registry, tc.realm, got, tc.want)
		}
	}
}

func TestParseBearerChallenge(t *testing.T) {
	realm, service, scope := parseBearerChallenge(
		`Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`)
	if realm != "https://ghcr.io/token" || service != "ghcr.io" || scope != "repository:user/image:pull" {
		t.Fatalf("unexpected parse: realm=%q service=%q scope=%q", realm, service, scope)
	}

	// Docker Hub returns a challenge without a scope.
	realm, service, scope = parseBearerChallenge(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`)
	if realm != "https://auth.docker.io/token" || service != "registry.docker.io" || scope != "" {
		t.Fatalf("unexpected parse: realm=%q service=%q scope=%q", realm, service, scope)
	}
}

// TestPullScope verifies the scope-selection rule: ghcr.io answers the root
// /v2/ probe with a placeholder scope (repository:user/image:pull) that must
// not be used — the token request for it fails with 403 DENIED. The scope must
// be rebuilt for the actual repository.
func TestPullScope(t *testing.T) {
	cases := []struct {
		name      string
		challenge string
		repo      string
		want      string
	}{
		{
			name:      "ghcr placeholder scope is replaced",
			challenge: "repository:user/image:pull",
			repo:      "pterodactyl/yolks",
			want:      "repository:pterodactyl/yolks:pull",
		},
		{
			name:      "no scope in challenge",
			challenge: "",
			repo:      "library/ubuntu",
			want:      "repository:library/ubuntu:pull",
		},
		{
			name:      "real scope for the repo is kept",
			challenge: "repository:pterodactyl/yolks:pull",
			repo:      "pterodactyl/yolks",
			want:      "repository:pterodactyl/yolks:pull",
		},
		{
			name:      "case-insensitive match keeps the challenge scope",
			challenge: "repository:PTERODACTYL/YOLKS:pull",
			repo:      "pterodactyl/yolks",
			want:      "repository:PTERODACTYL/YOLKS:pull",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pullScope(tc.challenge, tc.repo)
			if got != tc.want {
				t.Fatalf("pullScope(%q, %q) = %q, want %q", tc.challenge, tc.repo, got, tc.want)
			}
		})
	}
}

func TestGetTokenWithRetryCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := getTokenWithRetry(ctx, "library/nginx"); err == nil {
		t.Fatal("expected error for canceled context, got token")
	}
}
