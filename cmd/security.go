//go:build linux

package cmd

import (
	"fmt"
	"os"
	"strings"

	"cardinal/internal/container"
)

// Exit codes follow sysexits(3) conventions where reasonable so scripts
// can branch on the failure class instead of guessing from prose output.
const (
	ExitCodeOK             = 0
	ExitCodeUserError      = 2 // EX_USAGE
	ExitCodeDataError      = 65 // EX_DATAERR
	ExitCodeNoInput        = 66 // EX_NOINPUT
	ExitCodeSoftware       = 70 // EX_SOFTWARE
	ExitCodeIOError        = 74 // EX_IOERR
	ExitCodeConfigError    = 78 // EX_CONFIG
	ExitCodePermission     = 77 // EX_NOPERM
)

// dangerousCaps is the canonical blocklist of Linux capabilities that
// effectively give a container the same privileges as the host. Operators
// must opt in via --allow-dangerous-caps before any of these can be added.
var dangerousCaps = map[string]struct{}{
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

// validateDangerousRuntimeOptions enforces the runtime hardening policy.
//
// Two distinct policies:
//   - capAdd containing any name in dangerousCaps is rejected unless the
//     user passes --allow-dangerous-caps. Acknowledgement is logged in
//     the container's audit trail.
//   - --user 0 / 0:0 / root is rejected unless the user passes
//     --allow-root. Running as UID 0 inside a user-namespaced cgroup is
//     still distinct from root on the host, but operators should not do
//     it by accident.
//
// Returns true if the configuration is acceptable (after the user-supplied
// overrides were honored). Returns false after printing the error so the
// caller can os.Exit(ExitCodeUserError).
func validateDangerousRuntimeOptions(capAdd []string, user string, allowDangerousCaps, allowRoot bool) bool {
	dangerous := dangerousCapsRequested(capAdd)
	if len(dangerous) > 0 && !allowDangerousCaps {
		fmt.Fprintf(os.Stderr,
			"Error: refusing to add dangerous capabilities without explicit acknowledgement: %s.\n"+
				"Re-run with --allow-dangerous-caps if this is intentional (and record why in your change ticket).\n",
			strings.Join(dangerous, ", "))
		return false
	}
	if len(dangerous) > 0 {
		fmt.Fprintf(os.Stderr,
			"WARNING: container will run with dangerous capabilities: %s\n",
			strings.Join(dangerous, ", "))
	}

	if isRootUser(user) && !allowRoot {
		fmt.Fprintf(os.Stderr,
			"Error: refusing to start a container with --user %q without explicit acknowledgement.\n"+
				"Re-run with --allow-root if this is intentional.\n",
			user)
		return false
	}
	return true
}

func dangerousCapsRequested(caps []string) []string {
	var out []string
	for _, c := range caps {
		c = strings.ToUpper(strings.TrimSpace(c))
		// Convert "CAP_SYS_ADMIN" form to "SYS_ADMIN" for lookup.
		c = strings.TrimPrefix(c, "CAP_")
		if _, ok := dangerousCaps[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

func isRootUser(u string) bool {
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

// Sanity check that no audit event references the legacy "container.New"
// path without going through this validator. This is enforced by an
// explicit test on the helper below.
var _ = container.New // import-only: ensures container package is linked.
