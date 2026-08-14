//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"dck/internal/log"
)

// RestrictDevShm mounts /dev/shm with restrictive options inside the container.
func RestrictDevShm(merged string) error {
	devShm := merged + "/dev/shm"

	// Ensure the directory exists
	if err := os.MkdirAll(devShm, 01777); err != nil {
		return fmt.Errorf("create /dev/shm: %w", err)
	}

	// Mount tmpfs on /dev/shm with size limit and noexec
	opts := "size=64M,mode=1777,noexec,nosuid,nodev"
	if err := exec.Command("mount", "-t", "tmpfs", "-o", opts, "tmpfs", devShm).Run(); err != nil {
		// Try mounting without noexec if it fails (some kernels restrict tmpfs options)
		opts = "size=64M,mode=1777,nosuid,nodev"
		if err := exec.Command("mount", "-t", "tmpfs", "-o", opts, "tmpfs", devShm).Run(); err != nil {
			return fmt.Errorf("mount /dev/shm: %w", err)
		}
	}

	return nil
}

// RestrictDevMqueue mounts /dev/mqueue with restrictive options.
func RestrictDevMqueue(merged string) error {
	devMqueue := merged + "/dev/mqueue"

	// Ensure the directory exists
	if err := os.MkdirAll(devMqueue, 01777); err != nil {
		return fmt.Errorf("create /dev/mqueue: %w", err)
	}

	// Mount mqueue filesystem with restrictive options
	opts := "mode=1777,noexec,nosuid,nodev"
	if err := exec.Command("mount", "-t", "mqueue", "-o", opts, "mqueue", devMqueue).Run(); err != nil {
		// mqueue might not be supported, try without options
		if err := exec.Command("mount", "-t", "mqueue", "mqueue", devMqueue).Run(); err != nil {
			return fmt.Errorf("mount /dev/mqueue: %w", err)
		}
	}

	return nil
}

// RestrictProcSys mounts /proc/sys as read-only.
func RestrictProcSys(merged string) error {
	procSys := merged + "/proc/sys"

	// Check if /proc/sys exists
	if _, err := os.Stat(procSys); os.IsNotExist(err) {
		return nil
	}

	// Bind mount /proc/sys to itself
	if err := exec.Command("mount", "--bind", procSys, procSys).Run(); err != nil {
		return fmt.Errorf("bind mount /proc/sys: %w", err)
	}

	// Remount as read-only
	if err := exec.Command("mount", "--bind", "-o", "remount,ro", procSys, procSys).Run(); err != nil {
		return fmt.Errorf("remount /proc/sys read-only: %w", err)
	}

	return nil
}

// RestrictSys mounts /sys as read-only.
func RestrictSys(merged string) error {
	sysDir := merged + "/sys"

	// Check if /sys exists
	if _, err := os.Stat(sysDir); os.IsNotExist(err) {
		return nil
	}

	// Bind mount /sys to itself
	if err := exec.Command("mount", "--bind", sysDir, sysDir).Run(); err != nil {
		return fmt.Errorf("bind mount /sys: %w", err)
	}

	// Remount as read-only
	if err := exec.Command("mount", "--bind", "-o", "remount,ro", sysDir, sysDir).Run(); err != nil {
		return fmt.Errorf("remount /sys read-only: %w", err)
	}

	return nil
}

// ApplyDeviceRestrictions applies all device restrictions to the container.
func ApplyDeviceRestrictions(merged string) error {
	restrictions := []struct {
		name string
		fn   func(string) error
	}{
		{"restrict /dev/shm", RestrictDevShm},
		{"restrict /dev/mqueue", RestrictDevMqueue},
		{"restrict /proc/sys", RestrictProcSys},
		{"restrict /sys", RestrictSys},
	}

	for _, r := range restrictions {
		if err := r.fn(merged); err != nil {
			log.Warn("%s: %v", r.name, err)
			// Continue with other restrictions even if one fails
		}
	}

	return nil
}

// RestrictSensitiveDevices restricts access to sensitive device files.
func RestrictSensitiveDevices(merged string) error {
	// Create a minimal /dev with only safe devices
	devDir := merged + "/dev"
	if err := os.MkdirAll(devDir, 0755); err != nil {
		return fmt.Errorf("create /dev: %w", err)
	}

	// Mount devtmpfs if not already mounted
	if err := exec.Command("mount", "-t", "devtmpfs", "devtmpfs", devDir).Run(); err != nil {
		log.Warn("mount devtmpfs: %v", err)
	}

	// Remove or restrict access to sensitive devices
	sensitiveDevices := []string{
		"/dev/mem",
		"/dev/kmem",
		"/dev/sda",
		"/dev/sda1",
		"/dev/nvme0n1",
		"/dev/vda",
	}

	for _, device := range sensitiveDevices {
		// Create a node that denies access
		devicePath := merged + device
		if _, err := os.Stat(devicePath); err == nil {
			// Remove the device node
			if err := os.Remove(devicePath); err != nil {
				log.Warn("remove %s: %v", device, err)
			}
		}
	}

	return nil
}

// RestrictMountOperations restricts mount operations inside the container.
func RestrictMountOperations() error {
	// This is typically handled by seccomp or AppArmor profiles
	// For additional safety, we can set the no_new_privs flag
	// which prevents gaining new privileges through setuid binaries

	// Set PR_SET_NO_NEW_PRIVS
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_PRCTL,
		0x38, // PR_SET_NO_NEW_PRIVS
		1,
		0, 0, 0, 0,
	); errno != 0 {
		return fmt.Errorf("set no_new_privs: %v", errno)
	}

	return nil
}
