//go:build linux

package container

import (
	"fmt"
	"os"
	"syscall"
)

func namespaceInode(pid int, namespace string) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid container PID")
	}
	info, err := os.Stat(fmt.Sprintf("/proc/%d/ns/%s", pid, namespace))
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("read %s namespace metadata", namespace)
	}
	return stat.Ino, nil
}

func mountNamespaceInode(pid int) (uint64, error) {
	return namespaceInode(pid, "mnt")
}

func containerNamespaceIdentities(pid int) (mount, pidNS, network, ipc, uts uint64, err error) {
	mount, err = namespaceInode(pid, "mnt")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	pidNS, err = namespaceInode(pid, "pid")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	network, err = namespaceInode(pid, "net")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	ipc, err = namespaceInode(pid, "ipc")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	uts, err = namespaceInode(pid, "uts")
	return mount, pidNS, network, ipc, uts, err
}

func (c *Container) validateNamespaceTarget() error {
	if c == nil || c.Status != Running || c.PID <= 0 {
		return fmt.Errorf("container is not running")
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d/root", c.PID)); err != nil {
		return fmt.Errorf("container process is unavailable: %w", err)
	}
	if c.MountNamespace == 0 || c.PIDNamespace == 0 || c.NetworkNamespace == 0 || c.IPCNamespace == 0 || c.UTSNamespace == 0 {
		return fmt.Errorf("container namespace identity is unavailable; restart the container")
	}
	mount, pidNS, network, ipc, uts, err := containerNamespaceIdentities(c.PID)
	if err != nil {
		return fmt.Errorf("read container namespaces: %w", err)
	}
	if mount != c.MountNamespace || pidNS != c.PIDNamespace || network != c.NetworkNamespace || ipc != c.IPCNamespace || uts != c.UTSNamespace {
		return fmt.Errorf("container PID no longer belongs to this container")
	}
	return nil
}
