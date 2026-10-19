package image

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cardinal/internal/state"
)

// RegistryAllowlistEntry records a single allowed registry hostname.
//
// The on-disk format is one hostname per line; comments start with '#'. The
// list is consulted on every Pull/Push to refuse pulls from registries
// outside the allowlist when CARDINAL_REGISTRY_STRICT=1 is set in the
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
// run. Строгий режим (allowlist по умолчанию) включается сам; оператор
// расширяет список через CARDINAL_REGISTRY_ALLOWLIST или файл allowlist.
func init() {
	allowlist = append(allowlist, defaultAllowlist...)
}

func allowlistPath() string {
	return filepath.Join(state.DataDir(), "registry-allowlist")
}

// InitRegistryAllowlist loads the on-disk allowlist and applies any
// CARDINAL_REGISTRY_ALLOWLIST (colon-separated) overrides on top.
//
// The function is idempotent and safe to call multiple times. It is wired
// into the pull/push paths so a deploy-time flip of CARDINAL_REGISTRY_STRICT=1
// immediately tightens accepted registries without a restart.
func InitRegistryAllowlist() error {
	allowlistMu.Lock()
	defer allowlistMu.Unlock()

	// Start from defaults so a fresh install still works against Docker Hub.
	allowlist = append(allowlist[:0], defaultAllowlist...)

	if env := strings.TrimSpace(os.Getenv("CARDINAL_REGISTRY_ALLOWLIST")); env != "" {
		for _, h := range strings.Split(env, ":") {
			h = strings.TrimSpace(h)
			if h != "" {
				allowlist = append(allowlist, h)
			}
		}
	}

	if env := strings.TrimSpace(os.Getenv("CARDINAL_REGISTRY_BLOCKLIST")); env != "" {
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

// CheckRegistryAllowed enforces the pull/push gate for an image reference
// (full `name[:tag]` or bare name; the tag is stripped before extracting
// the registry host). By default hosts outside the allowlist are refused
// before any network contact; the strict gate can be turned off with
// CARDINAL_REGISTRY_STRICT=0.
func CheckRegistryAllowed(ref string) error {
	name := ref
	// Strip :tag (but never a :port, and never touch digest refs).
	if !strings.Contains(name, "@") {
		if i := strings.LastIndex(name, ":"); i > 0 && !strings.Contains(name[i:], "/") {
			name = name[:i]
		}
	}
	host, _ := registryHostAndRepo(name)
	if !IsRegistryAllowed(host) {
		return fmt.Errorf("registry %q is not in the allowlist (enable CARDINAL_REGISTRY_STRICT=0 to allow any registry)", host)
	}
	return nil
}

// IsRegistryAllowed reports whether `host` is in the current allowlist.
//
// ПО УМОЛЧАНИЮ allowlist строгий: разрешены только перечисленные реестры.
// Раньше без CARDINAL_REGISTRY_STRICT=1 функция возвращала true для ЛЮБОГО
// хоста, и allowlist служил только документацией — то есть образ можно было
// тянуть с любого сервера, включая внутренний (SSRF через pull) или
// скомпрометированный зеркальный реестр.
//
// Строгий режим можно отключить явно: CARDINAL_REGISTRY_STRICT=0.
func IsRegistryAllowed(host string) bool {
	host = normalizeRegistryHostname(host)
	if !registryStrict() {
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

// registryStrict — включён ли строгий режим allowlist (по умолчанию да).
func registryStrict() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CARDINAL_REGISTRY_STRICT")))
	if v == "" {
		return true
	}
	return !(v == "0" || v == "false" || v == "no")
}

// checkRegistryHostNotInternal — SSRF-барьер для хоста реестра.
//
// Pull собирает URL вида https://<host>/v2/... из пользовательской ссылки
// на образ. Без проверки ссылка вида "169.254.169.254/latest/meta-data"
// заставляла демон обратиться к cloud-metadata или к внутреннему сервису.
// Loopback/private/link-local диапазоны запрещены (self-hosted реестр на
// localhost — намеренно полезный случай, его можно разрешить явно через
// CARDINAL_ALLOW_INSECURE_REGISTRY=1).
func checkRegistryHostNotInternal(host string) error {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return fmt.Errorf("registry host is empty")
	}
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		if IsInsecureRegistryAllowed() {
			return nil
		}
		return fmt.Errorf("registry host %q is internal (set CARDINAL_ALLOW_INSECURE_REGISTRY=1 to allow)", host)
	}
	if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
			if IsInsecureRegistryAllowed() {
				return nil
			}
			return fmt.Errorf("registry host %q is an internal address (set CARDINAL_ALLOW_INSECURE_REGISTRY=1 to allow)", host)
		}
		return nil
	}
	return nil
}

// IsInsecureRegistryAllowed reports whether pulling from a plain-http or
// localhost registry is permitted. The default is to refuse so operators
// have to opt in by setting CARDINAL_ALLOW_INSECURE_REGISTRY=1.
func IsInsecureRegistryAllowed() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CARDINAL_ALLOW_INSECURE_REGISTRY")))
	return v == "1" || v == "true" || v == "yes"
}
