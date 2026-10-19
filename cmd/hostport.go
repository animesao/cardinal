//go:build linux

package cmd

import (
	"fmt"
	"os"

	"cardinal/internal/container"
	"cardinal/internal/portmap"
)

// rejectCrossContainerPortConflicts exits when any requested host port is
// already mapped by another cardinal container.
//
// Общая логика живёт в internal/container — там же её использует HTTP API,
// поэтому CLI и API не могут разойтись.
func rejectCrossContainerPortConflicts(ports []container.PortMap, selfID, selfName string) {
	if conflict := container.CheckPortConflicts(ports, selfID, selfName); conflict != nil {
		fmt.Fprintf(os.Stderr, "Error: host port %d/%s is already mapped by container %q (%d -> %d/%s)\n",
			conflict.HostPort, conflict.Protocol, conflict.OwnerName,
			conflict.ContainerPort, conflict.ContainerPort, conflict.Protocol)
		fmt.Fprintln(os.Stderr, "Hint: one host port serves one container; pick another host port (e.g. -p 8081:80) or remove the existing mapping first")
		exitFunc(1)
	}
}

// warnHostLocalPortConflicts probes the host for listeners on the requested
// ports and warns when something (e.g. cardinal-wings on 8080) already holds
// one. It only warns: a host-local listener shadows localhost clients, but
// external traffic still follows the PREROUTING DNAT rule to the container,
// so the mapping may be intentional.
func warnHostLocalPortConflicts(ports []container.PortMap) {
	for _, p := range ports {
		if p.HostPort == 0 {
			continue
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		if !portmap.HostPortInUse(p.HostPort, proto) {
			continue
		}
		if svc := portmap.KnownService(p.HostPort); svc != "" {
			fmt.Fprintf(os.Stderr, "Warning: host port %d/%s is already in use on this host — likely by %s\n",
				p.HostPort, proto, svc)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: host port %d/%s is already in use on this host (check: ss -tlnp | grep %d)\n",
				p.HostPort, proto, p.HostPort)
		}
		fmt.Fprintf(os.Stderr, "Hint: localhost clients (curl http://localhost:%d) will reach that service instead of this container; use another host port (e.g. -p 8081:%d)\n",
			p.HostPort, p.ContainerPort)
	}
}
