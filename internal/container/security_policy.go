//go:build linux

package container

import (
	"fmt"
	"strings"
)

// DangerousCaps is the canonical blocklist of Linux capabilities that
// effectively give a container the same privileges as the host. They are
// refused unless explicitly acknowledged (CLI --allow-dangerous-caps,
// API AllowDangerousCaps). The check lives here — in the runtime layer —
// so the CLI and the API enforce the same policy; a CLI-only check would
// leave the API as a silent bypass.
var DangerousCaps = map[string]struct{}{
	"SYS_ADMIN":  {}, // mount cgroups, mount arbitrary filesystems, swapon
	"SYS_MODULE": {}, // load kernel modules
	"SYS_RAWIO":  {}, // access physical memory and I/O ports
	"SYS_PTRACE": {}, // trace arbitrary processes
	"SYS_BOOT":   {}, // reboot the host
	"NET_ADMIN":  {}, // reconfigure networking (iptables, routing, interfaces)
	"NET_RAW":    {}, // raw socket manipulation
	"BPF":        {}, // extended BPF program loading
	"PERFMON":    {}, // perf_event_open GPU/CPU side channels
	"WAKE_ALARM": {}, // configure RTC wake alarms (DoS)
}

// DangerousCapsRequested returns the subset of caps that is blocklisted.
// "ALL" counts as dangerous: the runtime expands it to every capability,
// including the blocklisted ones.
func DangerousCapsRequested(caps []string) []string {
	var out []string
	for _, c := range caps {
		c = strings.ToUpper(strings.TrimSpace(c))
		c = strings.TrimPrefix(c, "CAP_")
		if c == "ALL" {
			out = append(out, c)
			continue
		}
		if _, ok := DangerousCaps[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// IsRootUser reports whether a --user value means UID 0.
func IsRootUser(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" || u == "root" || u == "0" || u == "0:0" {
		return u != "" // empty is the default; not root
	}
	if parts := strings.SplitN(u, ":", 2); len(parts) >= 1 {
		switch parts[0] {
		case "0", "root":
			return true
		}
	}
	return false
}

// ValidateRuntimePolicy enforces the runtime hardening policy shared by the
// CLI and the API:
//
//   - capAdd containing a blocklisted capability is rejected unless
//     allowDangerousCaps is set;
//   - an explicit root user (--user 0/root) is rejected unless allowRoot
//     is set. NOTE: the unset default also runs as uid 0 (no user
//     namespace); the gate only catches explicit requests, so automation
//     does not escalate silently.
func ValidateRuntimePolicy(capAdd []string, user string, allowDangerousCaps, allowRoot bool) error {
	if dangerous := DangerousCapsRequested(capAdd); len(dangerous) > 0 && !allowDangerousCaps {
		return fmt.Errorf("refusing dangerous capabilities without explicit acknowledgement: %s", strings.Join(dangerous, ", "))
	}
	if IsRootUser(user) && !allowRoot {
		return fmt.Errorf("refusing --user %q without explicit acknowledgement", user)
	}
	return nil
}
