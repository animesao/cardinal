//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"cardinal/internal/container"
)

func Stats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	noStream := fs.Bool("no-stream", false, "Show one-time stats and exit")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing stats options: %v\n", err)
		os.Exit(1)
	}

	remainder := fs.Args()
	if len(remainder) == 0 {
		// Show all running containers
		containers, err := container.List(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(containers) == 0 {
			fmt.Println("No running containers")
			return
		}
		showStatsLoop(containers, *noStream)
		return
	}

	// Show specific container(s)
	name := remainder[0]
	c := container.FindByName(name)
	if c == nil {
		// Try loading by ID prefix
		all, err := container.List(true)
		if err == nil {
			for _, ct := range all {
				if ct.ID == name || shortID(ct.ID) == name {
					c = ct
					break
				}
			}
		}
	}
	if c == nil {
		fmt.Fprintf(os.Stderr, "Container not found: %s\n", name)
		os.Exit(1)
	}
	showStatsLoop([]*container.Container{c}, *noStream)
}

func showStatsLoop(containers []*container.Container, noStream bool) {
	prevSnapshots := make(map[string]*container.StatsSnapshot)

	for {
		showHeader := true
		for _, c := range containers {
			s, err := container.ReadContainerStats(c)
			if err != nil {
				if prevSnapshots[string(c.ID)] == nil {
					fmt.Fprintf(os.Stderr, "Failed to read stats for %s: %v\n", c.Name, err)
				}
				continue
			}
			prev := prevSnapshots[c.ID]
			container.PrintContainerStats(s, prev, showHeader)
			prevSnapshots[c.ID] = &container.StatsSnapshot{
				CPUUsage:  s.CPUUsage,
				Timestamp: s.Timestamp,
			}
			showHeader = false
		}

		if noStream {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
