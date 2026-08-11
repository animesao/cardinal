//go:build linux

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsExternalHost(t *testing.T) {
	tests := []struct {
		host     string
		external bool
	}{
		{host: "127.0.0.1", external: false},
		{host: "localhost", external: false},
		{host: "::1", external: false},
		{host: "[::1]", external: false},
		{host: "0.0.0.0", external: true},
		{host: "192.0.2.10", external: true},
		{host: "example.invalid", external: true},
	}

	for _, tt := range tests {
		if got := isExternalHost(tt.host); got != tt.external {
			t.Errorf("isExternalHost(%q) = %v, want %v", tt.host, got, tt.external)
		}
	}
}

func TestAllowedCORSOrigin(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "https://127.0.0.1:5173", "http://[::1]:8080"} {
		if !isAllowedCORSOrigin(origin) {
			t.Errorf("isAllowedCORSOrigin(%q) = false, want true", origin)
		}
	}
	for _, origin := range []string{"https://example.com", "http://localhost.evil", "http://user@localhost:3000", "http://localhost/path"} {
		if isAllowedCORSOrigin(origin) {
			t.Errorf("isAllowedCORSOrigin(%q) = true, want false", origin)
		}
	}
}

func TestStartServerWithTLSRequiresBothFiles(t *testing.T) {
	oldToken := authToken
	t.Cleanup(func() { authToken = oldToken })
	SetAuthToken("")
	if err := StartServerWithTLS(2375, "127.0.0.1", "cert.pem", ""); err == nil {
		t.Fatal("StartServerWithTLS unexpectedly accepted only a certificate")
	}
	if err := StartServerWithTLS(2375, "127.0.0.1", "", "key.pem"); err == nil {
		t.Fatal("StartServerWithTLS unexpectedly accepted only a key")
	}
}

func TestAuthMiddlewareRequiresBearerHeader(t *testing.T) {
	oldToken := authToken
	t.Cleanup(func() { authToken = oldToken })
	SetAuthToken("test-token")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(next)

	tests := []struct {
		name   string
		header string
		query  string
		status int
	}{
		{name: "missing", status: http.StatusForbidden},
		{name: "query token rejected", query: "?token=test-token", status: http.StatusForbidden},
		{name: "raw token rejected", header: "test-token", status: http.StatusForbidden},
		{name: "bearer accepted", header: "Bearer test-token", status: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/version"+tt.query, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.status {
				t.Errorf("status = %d, want %d", resp.Code, tt.status)
			}
		})
	}
}
