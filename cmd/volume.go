//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"dck/internal/container"
)

func Volume(args []string) {
	if len(args) < 1 {
		printVolumeUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "create":
		volumeCreate(subargs)
	case "ls", "list":
		volumeList(subargs)
	case "rm", "remove":
		volumeRemove(subargs)
	case "inspect":
		volumeInspect(subargs)
	case "prune":
		volumePrune(subargs)
	default:
		fmt.Printf("unknown volume command: %s\n", subcommand)
		printVolumeUsage()
		os.Exit(1)
	}
}

func printVolumeUsage() {
	fmt.Println(`Usage: dck volume COMMAND

Manage volumes

Commands:
  create     Create a volume
  ls         List all volumes
  rm         Remove one or more volumes
  inspect    Display detailed information on one or more volumes
  prune      Remove all unused local volumes`)
}

func volumeCreate(args []string) {
	fs := flag.NewFlagSet("volume create", flag.ExitOnError)
	driver := fs.String("d", "local", "Volume driver")
	var labels stringSlice
	fs.Var(&labels, "l", "Set volume labels")
	fs.Var(&labels, "label", "Set volume labels")

	fs.Parse(args)

	freeArgs := fs.Args()
	var name string
	if len(freeArgs) > 0 {
		name = freeArgs[0]
	} else {
		// Generate a name
		name = fmt.Sprintf("vol_%d", time.Now().Unix())
	}

	labelMap := make(map[string]string)
	for _, l := range labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labelMap[parts[0]] = parts[1]
		}
	}

	vol, err := container.CreateVolume(name, *driver, labelMap, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created volume: %s\n", vol.Name)
	fmt.Printf("  Driver: %s\n", vol.Driver)
	fmt.Printf("  Mountpoint: %s\n", vol.Mountpoint)
}

func volumeList(args []string) {
	volumes, err := container.ListVolumes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(volumes) == 0 {
		fmt.Println("No volumes found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VOLUME NAME\tDRIVER\tMOUNTPOINT\tCREATED")
	for _, v := range volumes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			v.Name, v.Driver, v.Mountpoint,
			v.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
}

func volumeRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck volume rm <name> [<name>...]")
		os.Exit(1)
	}

	for _, name := range args {
		if err := container.RemoveVolume(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing volume %q: %v\n", name, err)
		} else {
			fmt.Println(name)
		}
	}
}

func volumeInspect(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck volume inspect <name>")
		os.Exit(1)
	}

	for _, name := range args {
		vol, err := container.InspectVolume(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Printf("Volume: %s\n", vol.Name)
		fmt.Printf("  Driver: %s\n", vol.Driver)
		fmt.Printf("  Mountpoint: %s\n", vol.Mountpoint)
		fmt.Printf("  Created: %s\n", vol.CreatedAt.Format(time.RFC3339))
		if len(vol.Labels) > 0 {
			fmt.Printf("  Labels:\n")
			for k, v := range vol.Labels {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
	}
}

func volumePrune(args []string) {
	volumes, err := container.ListVolumes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check which volumes are in use
	inUse := make(map[string]bool)
	containers, _ := container.List(true)
	for _, c := range containers {
		for _, v := range c.Volumes {
			inUse[v.Source] = true
		}
	}

	pruned := 0
	for _, v := range volumes {
		if inUse[v.Name] {
			continue
		}
		fmt.Printf("Removing volume %s...\n", v.Name)
		if err := container.RemoveVolume(v.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			pruned++
		}
	}

	fmt.Printf("Pruned %d unused volumes\n", pruned)
}
