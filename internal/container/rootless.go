//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"cardinal/internal/network"
)

// IsRootless returns true if cardinal is running without root privileges
func IsRootless() bool {
	return os.Geteuid() != 0
}

// CheckRootlessPrereqs checks if rootless mode can work on this system.
// Returns a list of warnings (non-fatal) and errors (fatal).
func CheckRootlessPrereqs() ([]string, []string) {
	var warnings []string
	var errors []string

	if !IsRootless() {
		return warnings, errors
	}

	// Check fuse-overlayfs
	if _, err := exec.LookPath("fuse-overlayfs"); err != nil {
		warnings = append(warnings, "fuse-overlayfs not found: overlayfs will not work in rootless mode")
	}

	// Check slirp4netns
	if _, err := exec.LookPath("slirp4netns"); err != nil {
		warnings = append(warnings, "slirp4netns not found: networking will not work in rootless mode. Install with: apt install slirp4netns")
	}

	// Check newuidmap/newgidmap
	if _, err := exec.LookPath("newuidmap"); err != nil {
		errors = append(errors, "newuidmap not found: user namespace mapping required for rootless. Install with: apt install uidmap")
	}
	if _, err := exec.LookPath("newgidmap"); err != nil {
		errors = append(errors, "newgidmap not found: group namespace mapping required for rootless. Install with: apt install uidmap")
	}

	// Check /etc/subuid for our user
	currentUser, err := user.Current()
	if err == nil {
		uid := currentUser.Uid
		hasSubuid := false
		if data, err := os.ReadFile("/etc/subuid"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 1 && parts[0] == currentUser.Username {
					hasSubuid = true
					break
				}
				if len(parts) >= 1 && parts[0] == uid {
					hasSubuid = true
					break
				}
			}
		}
		if !hasSubuid {
			warnings = append(warnings, fmt.Sprintf("user %s not found in /etc/subuid. Run: sudo usermod --add-subuids 100000-165536 %s", currentUser.Username, currentUser.Username))
		}

		hasSubgid := false
		if data, err := os.ReadFile("/etc/subgid"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 1 && parts[0] == currentUser.Username {
					hasSubgid = true
					break
				}
				if len(parts) >= 1 && parts[0] == uid {
					hasSubgid = true
					break
				}
			}
		}
		if !hasSubgid {
			warnings = append(warnings, fmt.Sprintf("user %s not found in /etc/subgid. Run: sudo usermod --add-subgids 100000-165536 %s", currentUser.Username, currentUser.Username))
		}
	}

	return warnings, errors
}

// RootlessOverlaySupported returns true if rootless overlayfs is available
func RootlessOverlaySupported() bool {
	_, err := exec.LookPath("fuse-overlayfs")
	return err == nil
}

// RootlessNetworkSupported returns true if rootless networking is available
func RootlessNetworkSupported() bool {
	_, err := exec.LookPath("slirp4netns")
	return err == nil
}

// MountRootlessOverlay mounts an overlay filesystem using fuse-overlayfs
func MountRootlessOverlay(lower, upper, work, merged string) error {
	os.MkdirAll(merged, 0755)
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	cmd := exec.Command("fuse-overlayfs", "-o", opts, merged)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fuse-overlayfs: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// SetupRootlessNetwork sets up networking via slirp4netns
func SetupRootlessNetwork(pid int, containerID string) (string, error) {
	if !RootlessNetworkSupported() {
		return "", fmt.Errorf("slirp4netns not available")
	}

	// slirp4netns creates a tap device in the container's netns
	// and provides NAT to the outside world
	cmd := exec.Command("slirp4netns", "--configure",
		"--mtu", "65520",
		"-c", // configure the tap interface inside the namespace
		strconv.Itoa(pid),
		"tap0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("slirp4netns: %s: %w", strings.TrimSpace(string(out)), err)
	}

	ip, err := network.AllocateIP()
	if err != nil {
		return "10.0.2.100", nil
	}
	return ip, nil
}

// RootlessPortForward sets up port forwarding using rootlesskit port driver
// For now, we use socat as a simple forwarding mechanism
// Returns the PIDs of spawned processes for later cleanup.
func RootlessPortForward(hostPort, containerPort int, protocol, containerIP string) ([]int, error) {
	// Try rootlesskit first, fall back to socat
	if _, err := exec.LookPath("rootlesskit"); err == nil {
		cmd := exec.Command("rootlessctl", "add-ports",
			fmt.Sprintf("127.0.0.1:%d/%s", hostPort, strings.ToUpper(protocol)),
			fmt.Sprintf("%s:%d", containerIP, containerPort))
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("rootlesskit port: %s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil, nil
	}

	// Fallback to socat
	if _, err := exec.LookPath("socat"); err == nil {
		proto := strings.ToUpper(protocol)
		cmd := exec.Command("socat",
			fmt.Sprintf("%s-LISTEN:%d,reuseaddr,fork", proto, hostPort),
			fmt.Sprintf("%s:%s:%d", proto, containerIP, containerPort))
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("socat start: %w", err)
		}
		return []int{cmd.Process.Pid}, nil
	}

	return nil, fmt.Errorf("no port forwarding available (install rootlesskit or socat)")
}

// CleanupRootlessPorts kills only forwarding processes that match the
// container's configured endpoints. PIDs are persisted in state and may be
// stale after a reboot or PID reuse, so a generic PID-only kill is unsafe.
func CleanupRootlessPorts(pids []int, ports []PortMap) {
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}
		command := strings.ReplaceAll(string(cmdline), "\x00", " ")
		if !strings.Contains(command, "socat") && !strings.Contains(command, "rootlesskit") {
			continue
		}
		matchesPort := false
		for _, port := range ports {
			if strings.Contains(command, fmt.Sprintf("LISTEN:%d", port.HostPort)) && strings.Contains(command, fmt.Sprintf(":%d", port.ContainerPort)) {
				matchesPort = true
				break
			}
		}
		if !matchesPort {
			continue
		}
		_ = exec.Command("kill", strconv.Itoa(pid)).Run()
	}
}

// PrintRootlessInfo prints information about rootless mode
func PrintRootlessInfo() {
	fmt.Println("Running in rootless mode")
	warnings, errors := CheckRootlessPrereqs()

	if len(errors) > 0 {
		fmt.Println("Required dependencies missing:")
		for _, e := range errors {
			fmt.Printf("  ERROR: %s\n", e)
		}
	}

	if len(warnings) > 0 {
		fmt.Println("Optional dependencies missing:")
		for _, w := range warnings {
			fmt.Printf("  WARN: %s\n", w)
		}
	}
}

// GetRootlessUIDMappings returns the UID/GID mappings for rootless containers
func GetRootlessUIDMappings() (uidMap, gidMap string) {
	currentUser, err := user.Current()
	if err != nil {
		return "", ""
	}
	return fmt.Sprintf("0:%s:1", currentUser.Uid), fmt.Sprintf("0:%s:1", currentUser.Gid)
}
