//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dck/internal/log"
)

// DefaultAppArmorProfile returns the default AppArmor profile for containers.
// This profile restricts container access to the host system.
func DefaultAppArmorProfile() string {
	return `#include <tunables/global>

profile dck-container flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  # Deny access to sensitive host paths
  deny /proc/sys/** w,
  deny /proc/sysrq-trigger rw,
  deny /proc/acpi/** rw,
  deny /proc/kcore rw,
  deny /proc/latency_stats rw,
  deny /proc/timer_list rw,
  deny /sys/firmware/** rw,
  deny /sys/devices/virtual/net/** w,

  # Allow basic file operations in the container
  / r,
  /** r,
  /tmp/** rw,
  /var/tmp/** rw,
  /home/** rw,
  /root/** rw,

  # Allow network access
  network inet stream,
  network inet dgram,
  network inet6 stream,
  network inet6 dgram,
  network netlink raw,

  # Allow IPC
  ipc,
  dbus,

  # Allow signals to processes within the container
  signal (send,receive) peer=dck-container,

  # Deny access to /proc except for the container's own namespace
  /proc/* r,
  /proc/*/ns/** r,
  /proc/*/fd/** r,
  /proc/*/cmdline r,
  /proc/*/status r,
  /proc/*/limits r,
  /proc/*/mountinfo r,

  # Allow ptrace for debugging within the container
  ptrace (read,trace) peer=dck-container,

  # Deny mount operations (handled by the container runtime)
  deny mount,

  # Deny access to AppArmor itself
  deny /sys/kernel/security/** rw,

  # Allow /dev access but deny sensitive devices
  /dev/** rw,
  deny /dev/mem rw,
  deny /dev/kmem rw,
  deny /dev/sda* rw,
  deny /dev/nvme* rw,
}`
}

// LoadAppArmorProfile loads an AppArmor profile from a file.
func LoadAppArmorProfile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read AppArmor profile: %w", err)
	}

	return string(data), nil
}

// SaveAppArmorProfile saves an AppArmor profile to a file.
func SaveAppArmorProfile(profile, path string) error {
	return os.WriteFile(path, []byte(profile), 0644)
}

// WriteDefaultAppArmorProfile writes the default AppArmor profile to a file.
func WriteDefaultAppArmorProfile(path string) error {
	return SaveAppArmorProfile(DefaultAppArmorProfile(), path)
}

// ApplyAppArmorProfile applies an AppArmor profile to the current process.
// This must be called before exec into the container.
func ApplyAppArmorProfile(profileName string) error {
	if !AppArmorSupported() {
		log.Warn("AppArmor not supported on this system, skipping profile application")
		return nil
	}

	// Use aa-exec to apply the profile
	cmd := exec.Command("aa-exec", "-p", ":"+profileName, "--", "echo", "ok")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply AppArmor profile %q: %s: %w", profileName, strings.TrimSpace(string(output)), err)
	}

	return nil
}

// AppArmorSupported returns true if AppArmor is available on the system.
func AppArmorSupported() bool {
	// Check if AppArmor is enabled
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err != nil {
		return false
	}

	// Check if aa-exec is available
	if _, err := exec.LookPath("aa-exec"); err != nil {
		return false
	}

	// Check if we're in a confined profile
	data, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		return false
	}

	profile := strings.TrimSpace(string(data))
	// If already confined, we can use AppArmor
	if profile != "unconfined" && profile != "" {
		return true
	}

	// If unconfined, check if we can load profiles
	// by checking if apparmor_parser is available
	if _, err := exec.LookPath("apparmor_parser"); err != nil {
		return false
	}

	return true
}

// LoadProfileIntoKernel loads an AppArmor profile into the kernel.
func LoadProfileIntoKernel(profilePath string) error {
	cmd := exec.Command("apparmor_parser", "-r", profilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("load AppArmor profile: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// SetProfileOnProcess sets an AppArmor profile on a running process.
func SetProfileOnProcess(pid int, profileName string) error {
	// Write to /proc/<pid>/attr/current
	attrPath := fmt.Sprintf("/proc/%d/attr/current", pid)
	return os.WriteFile(attrPath, []byte(":"+profileName), 0644)
}

// IsAppArmorEnabled checks if AppArmor is enabled in the kernel.
func IsAppArmorEnabled() bool {
	// Check if /sys/kernel/security/apparmor exists
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err != nil {
		return false
	}

	// Check if profiles are loaded
	data, err := os.ReadFile("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return false
	}

	return len(strings.TrimSpace(string(data))) > 0
}

// GetActiveProfile returns the currently active AppArmor profile.
func GetActiveProfile() string {
	data, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// EnsureAppArmorProfile ensures the default profile is loaded into the kernel.
func EnsureAppArmorProfile() error {
	if !AppArmorSupported() {
		return nil
	}

	// Check if our profile is already loaded
	profileName := "dck-container"
	data, err := os.ReadFile("/sys/kernel/security/apparmor/profiles")
	if err == nil {
		if strings.Contains(string(data), profileName) {
			return nil // Profile already loaded
		}
	}

	// Write profile to temp file and load it
	tmpFile, err := os.CreateTemp("", "dck-apparmor-*.prof")
	if err != nil {
		return fmt.Errorf("create temp profile: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(DefaultAppArmorProfile()); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write profile: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp profile: %w", err)
	}

	return LoadProfileIntoKernel(tmpFile.Name())
}
