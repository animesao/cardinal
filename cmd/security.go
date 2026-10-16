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
	ExitCodeOK          = 0
	ExitCodeUserError   = 2  // EX_USAGE
	ExitCodeDataError   = 65 // EX_DATAERR
	ExitCodeNoInput     = 66 // EX_NOINPUT
	ExitCodeSoftware    = 70 // EX_SOFTWARE
	ExitCodeIOError     = 74 // EX_IOERR
	ExitCodeConfigError = 78 // EX_CONFIG
	ExitCodePermission  = 77 // EX_NOPERM
)

// dangerousCaps mirrors container.DangerousCaps for CLI messages; the
// enforcement itself lives in internal/container (ValidateRuntimePolicy)
// so the API enforces the same policy.
var dangerousCaps = container.DangerousCaps

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
// caller can exitFunc(ExitCodeUserError).
func validateDangerousRuntimeOptions(capAdd []string, user string, allowDangerousCaps, allowRoot bool) bool {
	if err := container.ValidateRuntimePolicy(capAdd, user, allowDangerousCaps, allowRoot); err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "refusing --user") {
			fmt.Fprintf(os.Stderr,
				"Error: refusing to start a container with --user %q without explicit acknowledgement.\n"+
					"Re-run with --allow-root if this is intentional.\n",
				user)
			return false
		}
		fmt.Fprintf(os.Stderr,
			"Error: %s.\n"+
				"Re-run with --allow-dangerous-caps if this is intentional (and record why in your change ticket).\n",
			msg)
		return false
	}
	if dangerous := container.DangerousCapsRequested(capAdd); len(dangerous) > 0 {
		fmt.Fprintf(os.Stderr,
			"WARNING: container will run with dangerous capabilities: %s\n",
			strings.Join(dangerous, ", "))
	}
	return true
}

func dangerousCapsRequested(caps []string) []string {
	return container.DangerousCapsRequested(caps)
}

func isRootUser(u string) bool {
	return container.IsRootUser(u)
}
