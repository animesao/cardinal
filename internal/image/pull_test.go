package image

import "testing"

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
		name     string
		challenge string
		repo     string
		want     string
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
