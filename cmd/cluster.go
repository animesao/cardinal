//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"dck/internal/api"
	"dck/internal/orchestrator"
)

func Cluster(args []string) {
	if len(args) < 1 {
		printClusterUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "init":
		clusterInit(subargs)
	case "join":
		clusterJoin(subargs)
	case "leave":
		clusterLeave(subargs)
	case "join-token":
		clusterJoinToken(subargs)
	case "info":
		clusterInfo(subargs)
	case "node":
		clusterNode(subargs)
	case "serve":
		clusterServe(subargs)
	case "ls", "list":
		clusterList(subargs)
	default:
		fmt.Printf("unknown cluster command: %s\n", subcommand)
		printClusterUsage()
		os.Exit(1)
	}
}

func printClusterUsage() {
	fmt.Println(`Usage: dck cluster COMMAND

Manage clusters

Commands:
  init           Initialize a new cluster
  join <peer>    Join an existing cluster
  join-token     Show the connection address for other nodes
  leave          Leave the cluster
  info           Show cluster overview
  ls             List cluster nodes
  node           Manage cluster nodes (ls, inspect)
  serve          Start cluster API server (accepts remote replica requests)`)
}

func clusterInit(args []string) {
	fs := flag.NewFlagSet("cluster init", flag.ExitOnError)
	name := fs.String("name", "default", "Cluster name")
	bind := fs.String("bind", "127.0.0.1", "Cluster bind address")
	port := fs.Int("port", 7946, "Cluster port")
	apiPort := fs.Int("api-port", 2375, "API server port (for remote replica requests)")
	startAPI := fs.Bool("serve", false, "Start API server after init")
	token := fs.String("token", "", "API authentication token (or DCK_TOKEN env)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cluster options: %v\n", err)
		os.Exit(1)
	}
	if *token == "" {
		*token = os.Getenv("DCK_TOKEN")
	}
	api.SetAuthToken(*token)
	api.SetServerVersion(version)
	orchestrator.SetAPIToken(*token)

	if err := orchestrator.InitCluster(*name, *bind, *port); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cluster %q initialized\n", *name)

	node, err := orchestrator.GetNode()
	if err == nil {
		fmt.Printf("  Node ID:   %s\n", shortID(node.ID))
		fmt.Printf("  Address:   %s:%d\n", node.Address, node.APIPort)
		fmt.Printf("  API Port:  %d (for remote replica requests)\n", *apiPort)
	}

	if *startAPI {
		fmt.Printf("Starting API server on %s:%d...\n", *bind, *apiPort)
		go func() {
			if err := api.StartServer(*apiPort, *bind); err != nil {
				fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
			}
		}()
	}
}

func clusterJoin(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck cluster join <peer_addr>")
		os.Exit(1)
	}

	peerAddr := args[0]
	bind := "127.0.0.1"
	port := 2375
	startAPI := false
	token := ""

	fs := flag.NewFlagSet("cluster join", flag.ExitOnError)
	fs.StringVar(&bind, "bind", "127.0.0.1", "Bind address")
	fs.IntVar(&port, "port", 2375, "API port")
	fs.BoolVar(&startAPI, "serve", false, "Start API server after join")
	fs.StringVar(&token, "token", "", "API authentication token (or DCK_TOKEN env)")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cluster join options: %v\n", err)
		os.Exit(1)
	}
	if token == "" {
		token = os.Getenv("DCK_TOKEN")
	}
	api.SetAuthToken(token)
	api.SetServerVersion(version)
	orchestrator.SetAPIToken(token)

	if err := orchestrator.JoinCluster(peerAddr, bind, port); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Joined cluster via %s\n", peerAddr)

	if startAPI {
		fmt.Printf("Starting API server on %s:%d...\n", bind, port)
		go func() {
			if err := api.StartServer(port, bind); err != nil {
				fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
			}
		}()
	}
}

func clusterLeave(args []string) {
	if err := orchestrator.LeaveCluster(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Left the cluster")
}

func clusterJoinToken(args []string) {
	node, err := orchestrator.GetNode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: not part of a cluster: %v\n", err)
		os.Exit(1)
	}

	info := orchestrator.GetClusterInfo()
	if info == nil {
		fmt.Fprintf(os.Stderr, "Error: cluster not initialized\n")
		os.Exit(1)
	}

	fmt.Printf("Cluster: %s (%s)\n", info.ClusterName, shortID(info.ClusterID))
	fmt.Printf("Join token:\n")
	fmt.Printf("  dck cluster join %s:%d\n", node.Address, node.APIPort)
}

func clusterInfo(args []string) {
	info := orchestrator.GetClusterInfo()
	if info == nil {
		fmt.Fprintln(os.Stderr, "Not part of a cluster")
		os.Exit(1)
	}

	nodes, err := orchestrator.ListNodes()
	if err != nil {
		nodes = nil
	}

	svcs, err2 := orchestrator.ListServices()
	if err2 != nil {
		svcs = nil
	}

	fmt.Printf("Cluster:\n")
	fmt.Printf("  Name:       %s\n", info.ClusterName)
	fmt.Printf("  ID:         %s\n", info.ClusterID)
	fmt.Printf("  Created:    %s\n", info.CreatedAt.Format(time.RFC1123))
	fmt.Printf("  Nodes:      %d\n", len(nodes))
	fmt.Printf("  Services:   %d\n", len(svcs))

	leaderCount := 0
	workerCount := 0
	activeCount := 0
	for _, n := range nodes {
		if n.Role == orchestrator.NodeRoleLeader {
			leaderCount++
		} else {
			workerCount++
		}
		if n.State == orchestrator.NodeStateActive {
			activeCount++
		}
	}
	fmt.Printf("  Leaders:    %d\n", leaderCount)
	fmt.Printf("  Workers:    %d\n", workerCount)
	fmt.Printf("  Active:     %d\n", activeCount)

	node, _ := orchestrator.GetNode()
	if node != nil {
		fmt.Printf("\n  Local node: %s (%s)\n", node.Name, shortID(node.ID))
	}
}

func clusterNode(args []string) {
	if len(args) < 1 {
		fmt.Println(`Usage: dck cluster node COMMAND

Manage cluster nodes

Commands:
  ls               List all nodes
  inspect <id>     Show detailed node info`)
		os.Exit(1)
	}

	switch args[0] {
	case "ls", "list":
		clusterNodeList(args[1:])
	case "inspect":
		clusterNodeInspect(args[1:])
	default:
		fmt.Printf("unknown node command: %s\n", args[0])
	}
}

func clusterNodeList(args []string) {
	nodes, err := orchestrator.ListNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes in cluster")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tADDRESS\tROLE\tSTATE\tCPU\tMEM\tLABELS"); err != nil {
		return
	}
	for _, n := range nodes {
		labels := ""
		if len(n.Labels) > 0 {
			first := true
			for k, v := range n.Labels {
				if !first {
					labels += ","
				}
				labels += k + "=" + v
				first = false
			}
		}
		mem := fmt.Sprintf("%.1fG", float64(n.MemTotal)/1e9)
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%s\t%d\t%s\t%s\n",
			shortID(n.ID),
			n.Name,
			n.Address, n.APIPort,
			n.Role,
			n.State,
			n.CPUCores,
			mem,
			labels,
		); err != nil {
			return
		}
	}
	_ = w.Flush()
}

func clusterNodeInspect(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck cluster node inspect <id>")
		os.Exit(1)
	}

	query := args[0]
	nodes, err := orchestrator.ListNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var found *orchestrator.Node
	for _, n := range nodes {
		if n.ID == query || len(n.ID) >= len(query) && n.ID[:len(query)] == query {
			found = n
			break
		}
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "Node %q not found\n", query)
		os.Exit(1)
	}

	fmt.Printf("  ID:        %s\n", found.ID)
	fmt.Printf("  Name:      %s\n", found.Name)
	fmt.Printf("  Address:   %s:%d\n", found.Address, found.APIPort)
	fmt.Printf("  Role:      %s\n", found.Role)
	fmt.Printf("  State:     %s\n", found.State)
	fmt.Printf("  CPU Cores: %d\n", found.CPUCores)
	fmt.Printf("  Memory:    %.1fG total, %.1fG available\n",
		float64(found.MemTotal)/1e9, float64(found.MemAvail)/1e9)
	fmt.Printf("  Joined:    %s\n", found.JoinedAt.Format(time.RFC1123))
	fmt.Printf("  Last Seen: %s\n", found.LastSeen.Format(time.RFC1123))
	if len(found.Labels) > 0 {
		fmt.Println("  Labels:")
		for k, v := range found.Labels {
			fmt.Printf("    %s=%s\n", k, v)
		}
	}
}

func clusterList(args []string) {
	nodes, err := orchestrator.ListNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes in cluster")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tADDRESS\tROLE\tSTATE\tLAST SEEN"); err != nil {
		return
	}
	for _, n := range nodes {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%s\t%s\n",
			shortID(n.ID),
			n.Name,
			n.Address, n.APIPort,
			n.Role,
			n.State,
			n.LastSeen.Format("15:04:05"),
		); err != nil {
			return
		}
	}
	_ = w.Flush()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func clusterServe(args []string) {
	fs := flag.NewFlagSet("cluster serve", flag.ExitOnError)
	port := fs.Int("p", 2375, "API port")
	host := fs.String("H", "127.0.0.1", "API host (external addresses require --token or DCK_TOKEN)")
	token := fs.String("token", "", "API authentication token (or DCK_TOKEN env)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cluster options: %v\n", err)
		os.Exit(1)
	}
	if *token == "" {
		*token = os.Getenv("DCK_TOKEN")
	}
	api.SetAuthToken(*token)
	api.SetServerVersion(version)
	orchestrator.SetAPIToken(*token)

	fmt.Printf("Starting cluster API server on %s:%d...\n", *host, *port)
	fmt.Println("  Accepting replica requests from cluster peers")

	if err := api.StartServer(*port, *host); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var _ = time.Now
