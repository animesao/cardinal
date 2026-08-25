//go:build linux

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"cardinal/internal/container"
)

func Port(args []string) {
	if len(args) < 1 {
		printPortUsage()
		os.Exit(1)
	}

	sub := args[0]

	switch sub {
	case "add":
		portAdd(args[1:])
	case "remove", "rm":
		portRemove(args[1:])
	default:
		portShow(args)
	}
}

func printPortUsage() {
	fmt.Println(`Usage:
  cardinal port <container>                  Show port mappings
  cardinal port add <c> <host>:<cont>[/p]   Add port mapping
  cardinal port remove <c> <host>[/p]        Remove port mapping
  cardinal port rm <c> <host>[/p]            Remove port mapping (alias)`)
}

func portShow(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal port <container>")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(c.Ports) == 0 {
		fmt.Printf("Container %s has no port mappings\n", c.Name)
		return
	}

	for _, p := range c.Ports {
		fmt.Printf("%s -> %d:%d/%s\n", c.Name, p.HostPort, p.ContainerPort, p.Protocol)
	}
}

func parsePortSpec(s string) (hostPort, containerPort int, protocol string, err error) {
	proto := "tcp"
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		proto = parts[1]
		s = parts[0]
	}
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, "", fmt.Errorf("invalid port spec %q (expected host:container or host:container/protocol)", s)
	}
	host, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid host port %q", parts[0])
	}
	cont, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid container port %q", parts[1])
	}
	return host, cont, proto, nil
}

func parsePortRef(s string) (hostPort int, protocol string, err error) {
	proto := "tcp"
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		proto = parts[1]
		s = parts[0]
	}
	host, err := strconv.Atoi(s)
	if err != nil {
		return 0, "", fmt.Errorf("invalid port %q", s)
	}
	return host, proto, nil
}

func portAdd(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cardinal port add <container> <host>:<container>[/proto]")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	hostPort, containerPort, protocol, err := parsePortSpec(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if c.FindPort(hostPort, protocol) != nil {
		fmt.Fprintf(os.Stderr, "Error: port %d/%s already mapped for container %s\n", hostPort, protocol, c.Name)
		os.Exit(1)
	}

	if err := c.AddPort(hostPort, containerPort, protocol); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added port mapping: %s -> %d:%d/%s\n", c.Name, hostPort, containerPort, protocol)
}

func portRemove(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cardinal port remove <container> <host>[/proto]")
		os.Exit(1)
	}

	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	hostPort, protocol, err := parsePortRef(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.RemovePort(hostPort, protocol); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed port mapping: %s -> %d/%s\n", c.Name, hostPort, protocol)
}
