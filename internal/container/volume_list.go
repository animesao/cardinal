//go:build linux

package container

import "strings"

// SplitVolumeSpecs splits a CLI volume list while keeping comma-delimited mount
// options (for example, ro,rshared) attached to the preceding mount.
func SplitVolumeSpecs(value string) []string {
	var specs []string
	var current string
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if current != "" && isVolumeOption(token) {
			current += "," + token
			continue
		}
		if current != "" {
			specs = append(specs, current)
		}
		current = token
	}
	if current != "" {
		specs = append(specs, current)
	}
	return specs
}

func isVolumeOption(value string) bool {
	switch {
	case value == "ro", value == "rw", value == "shared", value == "rshared":
		return true
	case value == "slave", value == "rslave", value == "private", value == "rprivate":
		return true
	case value == "Z", value == "z", value == "nocopy":
		return true
	case strings.HasPrefix(value, "size="), strings.HasPrefix(value, "mode="):
		return true
	case strings.HasPrefix(value, "nfsopts="):
		return true
	case value == "hard", value == "soft", value == "intr", value == "nointr":
		return true
	case strings.HasPrefix(value, "rsize="), strings.HasPrefix(value, "wsize="):
		return true
	default:
		return false
	}
}
