package image

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/state"
)

// AuthEntry stores credentials for a registry
type AuthEntry struct {
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var authFile = ""

func authPath() string {
	if authFile != "" {
		return authFile
	}
	return filepath.Join(state.DataDir(), "auth.json")
}

// Login saves registry credentials to auth.json
func Login(registry, username, password string) error {
	if !strings.Contains(registry, "://") {
		registry = "https://" + registry
	}

	// Persist with scheme-less hostname so exact-match lookups work
	// regardless of whether the user typed the https:// prefix.
	if strings.HasPrefix(registry, "https://") {
		registry = strings.TrimPrefix(registry, "https://")
	} else if strings.HasPrefix(registry, "http://") {
		registry = strings.TrimPrefix(registry, "http://")
	}

	entries, _ := loadAuth()

	// Remove existing entry for same registry
	updated := make([]AuthEntry, 0, len(entries))
	for _, e := range entries {
		if e.Registry != registry {
			updated = append(updated, e)
		}
	}

	updated = append(updated, AuthEntry{
		Registry: registry,
		Username: username,
		Password: password,
	})

	if err := saveAuth(updated); err != nil {
		return fmt.Errorf("save auth: %w", err)
	}

	fmt.Printf("Logged in to %s as %s\n", registry, username)
	return nil
}

// Logout removes registry credentials from auth.json
func Logout(registry string) error {
	if !strings.Contains(registry, "://") {
		registry = "https://" + registry
	}

	// Persist hostname-only so exact-match comparison sees the same value
	// that was stored by Login.
	if strings.HasPrefix(registry, "https://") {
		registry = strings.TrimPrefix(registry, "https://")
	} else if strings.HasPrefix(registry, "http://") {
		registry = strings.TrimPrefix(registry, "http://")
	}

	entries, err := loadAuth()
	if err != nil {
		return fmt.Errorf("no saved credentials for %s", registry)
	}

	updated := make([]AuthEntry, 0, len(entries))
	found := false
	for _, e := range entries {
		if e.Registry == registry {
			found = true
			continue
		}
		updated = append(updated, e)
	}

	if !found {
		return fmt.Errorf("not logged in to %s", registry)
	}

	if err := saveAuth(updated); err != nil {
		return fmt.Errorf("save auth: %w", err)
	}

	fmt.Printf("Logged out from %s\n", registry)
	return nil
}

// GetCredentials returns cached credentials for a registry.
//
// Lookups are exact-hostname matches against the stored entry; the previous
// substring logic incorrectly leaked credentials to look-alike registries
// (e.g. an entry for `docker.io` would otherwise match `attacker-docker.io`).
// We normalize the requested registry the same way as Login/Logout do.
func GetCredentials(registry string) (string, string) {
	// Env override first
	user := os.Getenv("DOCKER_USERNAME")
	pass := os.Getenv("DOCKER_PASSWORD")
	if user != "" && pass != "" {
		return user, pass
	}

	host := normalizeRegistryHostname(registry)

	entries, err := loadAuth()
	if err != nil {
		return "", ""
	}

	for _, e := range entries {
		if normalizeRegistryHostname(e.Registry) == host {
			return e.Username, e.Password
		}
	}

	return "", ""
}

// normalizeRegistryHostname strips scheme and path components from a registry
// literal so Login, Logout, and GetCredentials compare the same key.
func normalizeRegistryHostname(registry string) string {
	s := registry
	if strings.HasPrefix(s, "https://") {
		s = strings.TrimPrefix(s, "https://")
	} else if strings.HasPrefix(s, "http://") {
		s = strings.TrimPrefix(s, "http://")
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

// AuthHeader returns the Authorization header value for a registry
func AuthHeader(registry string) string {
	user, pass := GetCredentials(registry)
	if user == "" || pass == "" {
		return ""
	}
	auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + auth
}

func loadAuth() ([]AuthEntry, error) {
	data, err := os.ReadFile(authPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in")
		}
		return nil, err
	}

	var entries []AuthEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveAuth(entries []AuthEntry) error {
	dir := filepath.Dir(authPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(authPath(), data, 0600)
}
