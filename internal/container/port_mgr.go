//go:build linux

package container

import (
	"fmt"
	"runtime"

	"dck/internal/network"
)

func (c *Container) AddPort(hostPort, containerPort int, protocol string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if protocol == "" {
		protocol = "tcp"
	}

	if c.Status == Running && c.IP != "" {
		if runtime.GOOS == "linux" {
			if IsRootless() {
				ip := c.IP
				if ip == "" {
					ip = "10.0.2.100"
				}
				pids, err := RootlessPortForward(hostPort, containerPort, protocol, ip)
				if err != nil {
					return fmt.Errorf("rootless port forward: %w", err)
				}
				c.PortForwardPIDs = append(c.PortForwardPIDs, pids...)
			} else {
				if err := network.AddPortForwarding(c.IP, hostPort, containerPort, protocol); err != nil {
					return fmt.Errorf("add port forwarding: %w", err)
				}
			}
		}
	}

	c.Ports = append(c.Ports, PortMap{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      protocol,
	})

	return c.Save()
}

func (c *Container) RemovePort(hostPort int, protocol string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if protocol == "" {
		protocol = "tcp"
	}

	removed := false
	var remaining []PortMap
	for _, p := range c.Ports {
		if p.HostPort == hostPort && p.Protocol == protocol {
			removed = true
			if c.Status == Running && c.IP != "" {
				if runtime.GOOS == "linux" {
					network.RemovePortForwarding(c.IP, p.HostPort, p.ContainerPort, p.Protocol)
				}
			}
			continue
		}
		remaining = append(remaining, p)
	}

	if !removed {
		return fmt.Errorf("port mapping %d/%s not found", hostPort, protocol)
	}

	c.Ports = remaining
	return c.Save()
}

func (c *Container) FindPort(hostPort int, protocol string) *PortMap {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.Ports {
		if p.HostPort == hostPort && p.Protocol == protocol {
			return &p
		}
	}
	return nil
}
