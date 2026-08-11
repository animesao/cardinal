//go:build linux

package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"dck/internal/orchestrator"
)

func Service(args []string) {
	if len(args) < 1 {
		printServiceUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "create":
		serviceCreate(subargs)
	case "ls", "list":
		serviceList(subargs)
	case "rm", "remove":
		serviceRemove(subargs)
	case "scale":
		serviceScale(subargs)
	case "update":
		serviceUpdate(subargs)
	default:
		fmt.Printf("unknown service command: %s\n", subcommand)
		printServiceUsage()
		os.Exit(1)
	}
}

func printServiceUsage() {
	fmt.Println(`Usage: dck service COMMAND

Manage services

Commands:
  create         Create a new service
  ls             List services
  rm             Remove a service
  scale          Scale a service
  update         Update a service`)
}

func serviceCreate(args []string) {
	name := ""
	image := ""
	replicas := 1
	var ports []orchestrator.ServicePort
	env := make(map[string]string)

	// Handle --name and --replicas
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--replicas":
			if i+1 < len(args) {
				replicas, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-p", "--port":
			if i+1 < len(args) {
				parts := strings.Split(args[i+1], ":")
				sp := orchestrator.ServicePort{
					Protocol: "tcp",
				}
				switch len(parts) {
				case 1:
					sp.Port, _ = strconv.Atoi(parts[0])
					sp.TargetPort = sp.Port
				case 2:
					sp.Port, _ = strconv.Atoi(parts[0])
					sp.TargetPort, _ = strconv.Atoi(parts[1])
				}
				ports = append(ports, sp)
				i++
			}
		case "-e", "--env":
			if i+1 < len(args) {
				kv := strings.SplitN(args[i+1], "=", 2)
				if len(kv) == 2 {
					env[kv[0]] = kv[1]
				}
				i++
			}
		default:
			if image == "" {
				image = args[i]
			}
		}
	}

	if name == "" || image == "" {
		fmt.Println("Usage: dck service create --name <name> [--replicas N] [--port P:T] <image>")
		os.Exit(1)
	}

	opts := orchestrator.ServiceOpts{
		Ports: ports,
		Env:   env,
	}

	svc, err := orchestrator.CreateService(name, image, replicas, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created service %s\n", svc.Name)
	fmt.Printf("  Image: %s\n", svc.Image)
	fmt.Printf("  Replicas: %d\n", svc.Replicas)
}

func serviceList(args []string) {
	services, err := orchestrator.ListServices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(services) == 0 {
		fmt.Println("No services")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tIMAGE\tREPLICAS\tPORTS\tCREATED"); err != nil {
		return
	}
	for _, s := range services {
		portStr := ""
		for i, p := range s.Ports {
			if i > 0 {
				portStr += ", "
			}
			portStr += fmt.Sprintf("%d->%d/%s", p.Port, p.TargetPort, p.Protocol)
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			s.Name, s.Image, s.Replicas, portStr,
			s.CreatedAt.Format("2006-01-02 15:04:05")); err != nil {
			return
		}
	}
	_ = w.Flush()
}

func serviceRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck service rm <name>")
		os.Exit(1)
	}

	for _, name := range args {
		if err := orchestrator.RemoveService(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing service %q: %v\n", name, err)
		} else {
			fmt.Printf("Removed service %s\n", name)
		}
	}
}

func serviceScale(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: dck service scale <name> <replicas>")
		os.Exit(1)
	}

	name := args[0]
	replicas, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid replica count %q\n", args[1])
		os.Exit(1)
	}

	svc, err := orchestrator.ScaleService(name, replicas)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service %s scaled to %d replicas\n", svc.Name, svc.Replicas)
}

func serviceUpdate(args []string) {
	name := ""
	image := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--image":
			if i+1 < len(args) {
				image = args[i+1]
				i++
			}
		default:
			if name == "" {
				name = args[i]
			} else if image == "" {
				image = args[i]
			}
		}
	}

	if name == "" || image == "" {
		fmt.Println("Usage: dck service update <name> --image <new_image>")
		os.Exit(1)
	}

	svc, err := orchestrator.UpdateService(context.Background(), name, image, orchestrator.ServiceOpts{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service %s updated\n  Image: %s\n  Replicas: %d\n", svc.Name, svc.Image, svc.Replicas)
}
