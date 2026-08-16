//go:build linux

package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dck/internal/state"
)

// RegistryAllowlistEntry records a single allowed registry hostname.
//
// The on-disk format is one hostname per line; comments start with '#'. The
// list is consulted on every Pull/Push to refuse pulls from registries
// outside the allowlist when DCK_REGISTRY_STRICT=1 is set in the
// environment. Env-driven allowlists are layered on top to make ad-hoc
// automation easy without mutating the persistent store.
type RegistryAllowlistEntry struct {
	Host   string
	Source string // "config" | "env" | "default"
}

var (
	allowlistMu sync.RWMutex
	allowlist   []string

	defaultAllowlist = []string{
		"registry-1.docker.io",
		"registry.docker.io",
		"docker.io",
		"ghcr.io",
		"quay.io",
		"gcr.io",
	}
)

// init seeds the in-memory list with the built-in default so the runtime
// never accidentally blocks the official Docker Hub before policy init has
// run. Operators who want a closed environment must enable
// DCK_REGISTRY_STRICT=1 AND pre-populate the allowlist.
func init() {
	allowlist = append(allowlist, defaultAllowlist...)
}

func allowlistPath() string {
	return filepath.Join(state.DataDir(), "registry-allowlist")
}

// InitRegistryAllowlist loads the on-disk allowlist and applies any
// DCK_REGISTRY_ALLOWLIST (colon-separated) overrides on top.
//
// The function is idempotent and safe to call multiple times. It is wired
// into the pull/push paths so a deploy-time flip of DCK_REGISTRY_STRICT=1
// immediately tightens accepted registries without a restart.
func InitRegistryAllowlist() error {
	allowlistMu.Lock()
	defer allowlistMu.Unlock()

	// Start from defaults so a fresh install still works against Docker Hub.
	allowlist = append(allowlist[:0], defaultAllowlist...)

	if env := strings.TrimSpace(os.Getenv("DCK_REGISTRY_ALLOWLIST")); env != "" {
		for _, h := range strings.Split(env, ":") {
			h = strings.TrimSpace(h)
			if h != "" {
				allowlist = append(allowlist, h)
			}
		}
	}

	if env := strings.TrimSpace(os.Getenv("DCK_REGISTRY_BLOCKLIST")); env != "" {
		// The block-list is layered on top: remove any default that the
		// operator wants to forbid (e.g. quay.io for a strict environment).
		block := make(map[string]bool, 4)
		for _, h := range strings.Split(env, ":") {
			block[strings.TrimSpace(h)] = true
		}
		filtered := allowlist[:0]
		for _, h := range allowlist {
			if !block[h] {
				filtered = append(filtered, h)
			}
		}
		allowlist = filtered
	}

	if data, err := os.ReadFile(allowlistPath()); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			allowlist = append(allowlist, line)
		}
	}

	return nil
}

// AddToAllowlist appends a hostname to the persistent allowlist file.
func AddToAllowlist(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("empty hostname")
	}
	if strings.ContainsAny(host, " /") {
		return fmt.Errorf("invalid hostname %q", host)
	}
	if err := os.MkdirAll(filepath.Dir(allowlistPath()), 0700); err != nil {
		return err
	}
	allowlistMu.Lock()
	for _, h := range allowlist {
		if h == host {
			allowlistMu.Unlock()
			return nil // already present, idempotent
		}
	}
	allowlist = append(allowlist, host)
	current := append([]string(nil), allowlist...)
	allowlistMu.Unlock()

	f, err := os.OpenFile(allowlistPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(host + "\n"); err != nil {
		return err
	}
	_ = current // suppress unused-warning; kept for future extensions
	return nil
}

// RemoveFromAllowlist drops a hostname from the persistent store. The
// in-memory default list is never pruned because removing `docker.io` from
// a hardened deployment should be done through the block-list, not by
// breaking the default list.
func RemoveFromAllowlist(host string) error {
	host = strings.TrimSpace(host)
	allowlistMu.Lock()
	filtered := allowlist[:0]
	for _, h := range allowlist {
		if h == host {
			continue
		}
		filtered = append(filtered, h)
	}
	allowlist = filtered
	allowlistMu.Unlock()

	data := strings.Join(allowlist, "\n")
	if data != "" {
		data += "\n"
	}
	return os.WriteFile(allowlistPath(), []byte(data), 0600)
}

// AllowlistSnapshot returns a copy of the current in-memory allowlist,
// safe for iteration from outside this package.
func AllowlistSnapshot() []string {
	allowlistMu.RLock()
	defer allowlistMu.RUnlock()
	out := make([]string, len(allowlist))
	copy(out, allowlist)
	return out
}

// IsRegistryAllowed reports whether `host` is in the current allowlist.
// When DCK_REGISTRY_STRICT=1 is unset, the function returns true for any
// host (the allowlist then serves purely as documentation).
func IsRegistryAllowed(host string) bool {
	host = normalizeRegistryHostname(host)
	if os.Getenv("DCK_REGISTRY_STRICT") != "1" {
		return true
	}
	allowlistMu.RLock()
	defer allowlistMu.RUnlock()
	for _, h := range allowlist {
		if h == host {
			return true
		}
	}
	return false
}

// IsInsecureRegistryAllowed reports whether pulling from a plain-http or
// localhost registry is permitted. The default is to refuse so operators
// have to opt in by setting DCK_ALLOW_INSECURE_REGISTRY=1.
func IsInsecureRegistryAllowed() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DCK_ALLOW_INSECURE_REGISTRY")))
	return v == "1" || v == "true" || v == "yes"
}
