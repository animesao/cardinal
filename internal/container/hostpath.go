//go:build linux

package container

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	protectedHostPaths = []string{
		"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64",
		"/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin",
		"/sys", "/usr", "/var", "/var/lib/dck", "/var/run/dck",
		// State directories for sibling container runtimes; mounting them would
		// give containers parity with the host's existing containers.
		"/var/run/docker.sock", "/var/run/podman", "/var/run/containerd",
		// User credentials directories; bind-mounting ~/.ssh into an
		// untrusted container is one of the most common exfiltration routes.
		"/.ssh", "/.aws", "/.kube", "/.docker", "/.gnupg", "/.netrc",
	}

	protectedMu       sync.RWMutex
	allowedHostPaths  []string // overrides added by the user at runtime.
	hostPolicyInited  bool
)

// IsProtectedHostPath reports whether the absolute, symlink-resolved path
// lives under a directory that should never be bind-mounted into a
// container without an explicit operator action.
//
// The default blocklist covers system directories, runtime state dirs,
// and the common credential locations. Operators can opt in to additional
// paths via the DCK_ALLOWED_HOST_PATHS environment variable (colon-separated
// list). The use-case for the allowlist is homogeneous deployments where
// the data directory is co-located with the working directory.
//
// The check intentionally requires an absolute, symlink-resolved path:
// callers should pass `filepath.EvalSymlinks` output to avoid trivial
// symlink-escape bypasses like /etc -> /private/etc or /tmp -> /private/tmp.
func IsProtectedHostPath(path string) bool {
	if path == "" {
		return true
	}
	if !filepath.IsAbs(path) {
		return true
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	if info, err := os.Lstat(resolved); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if r, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = r
		}
	}

	protectedMu.RLock()
	defer protectedMu.RUnlock()

	for _, allowed := range allowedHostPaths {
		if resolved == allowed || strings.HasPrefix(resolved, filepath.Clean(allowed)+string(filepath.Separator)) {
			return false
		}
	}
	for _, blocked := range protectedHostPaths {
		if resolved == blocked || strings.HasPrefix(resolved, filepath.Clean(blocked)+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// InitHostPathPolicy loads the optional DCK_ALLOWED_HOST_PATHS env var
// exactly once. It is called from server start paths and is safe to call
// multiple times.
func InitHostPathPolicy() {
	protectedMu.Lock()
	defer protectedMu.Unlock()
	if hostPolicyInited {
		return
	}
	hostPolicyInited = true
	if env := strings.TrimSpace(os.Getenv("DCK_ALLOWED_HOST_PATHS")); env != "" {
		for _, p := range strings.Split(env, ":") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			allowedHostPaths = append(allowedHostPaths, abs)
		}
	}
}

// LoadHostPathAllowlistForTests is exposed for unit tests so the production
// list can be inspected deterministically.
func LoadHostPathAllowlistForTests() []string {
	protectedMu.RLock()
	defer protectedMu.RUnlock()
	out := make([]string, len(allowedHostPaths))
	copy(out, allowedHostPaths)
	return out
}
