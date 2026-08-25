//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"cardinal/internal/network"
)

func Network(args []string) {
	if len(args) == 0 {
		printNetworkUsage()
		return
	}
	switch args[0] {
	case "create":
		networkCreate(args[1:])
	case "ls", "list":
		networkList()
	case "inspect":
		networkInspect(args[1:])
	case "rm", "remove":
		networkRemove(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown network command: %s\n", args[0])
		printNetworkUsage()
		os.Exit(1)
	}
}

func networkCreate(args []string) {
	fs := flag.NewFlagSet("network create", flag.ExitOnError)
	subnet := fs.String("subnet", "", "IPv4 subnet (default: automatically selected)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing network create options: %v\n", err)
		os.Exit(1)
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardinal network create [--subnet 10.10.0.0/24] <name>")
		os.Exit(1)
	}
	n, err := network.CreateNetwork(fs.Args()[0], *subnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating network: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created network: %s\n  Driver: %s\n  Subnet: %s\n  Gateway: %s\n  Bridge: %s\n", n.Name, n.Driver, n.Subnet, n.Gateway, n.Bridge)
}

func networkList() {
	networks, err := network.ListNetworks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing networks: %v\n", err)
		os.Exit(1)
	}
	if len(networks) == 0 {
		fmt.Println("No user-defined networks")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDRIVER\tSUBNET\tGATEWAY\tBRIDGE")
	for _, n := range networks {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n.Name, n.Driver, n.Subnet, n.Gateway, n.Bridge)
	}
	_ = w.Flush()
}

func networkInspect(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardinal network inspect <name>")
		os.Exit(1)
	}
	n, err := network.LoadNetwork(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inspecting network: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Network: %s\n  ID: %s\n  Driver: %s\n  Subnet: %s\n  Gateway: %s\n  Bridge: %s\n  Allocated IPs: %d\n  Created: %s\n", n.Name, n.ID, n.Driver, n.Subnet, n.Gateway, n.Bridge, len(n.Allocated), n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
}

func networkRemove(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardinal network rm <name>")
		os.Exit(1)
	}
	if err := network.RemoveNetwork(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing network: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(args[0])
}

func printNetworkUsage() {
	fmt.Println(`Usage:
  cardinal network create [--subnet 10.10.0.0/24] <name>
  cardinal network ls
  cardinal network inspect <name>
  cardinal network rm <name>

User-defined networks use a Linux bridge and require root/CAP_NET_ADMIN when a
container using the network is started. The built-in bridge network remains
cardinal0 and does not appear in this list.`)
}
