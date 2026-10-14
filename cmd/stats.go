//go:build linux

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cardinal/internal/client"
	"cardinal/internal/container"
)

func Stats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	noStream := fs.Bool("no-stream", false, "Show one-time stats and exit")
	mustParse(fs, args, "stats")

	remainder := fs.Args()
	if host := remoteHostResolved(); host != "" {
		statsRemote(host, remainder, *noStream)
		return
	}
	if len(remainder) == 0 {
		// Show all running containers
		containers, err := container.List(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitFunc(1)
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
		exitFunc(1)
	}
	showStatsLoop([]*container.Container{c}, *noStream)
}

func showStatsLoop(containers []*container.Container, noStream bool) {
	prevSnapshots := make(map[string]*container.StatsSnapshot)
	redraw := !noStream && statsLiveRedraw()
	prevLines := 0
	first := true

	for {
		if !first && redraw {
			fmt.Printf("\x1b[%dA\x1b[J", prevLines)
		}
		lines := 0
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
			if showHeader {
				lines += 2
			} else {
				lines++
			}
			prevSnapshots[c.ID] = &container.StatsSnapshot{
				CPUUsage:  s.CPUUsage,
				Timestamp: s.Timestamp,
			}
			showHeader = false
		}
		prevLines = lines
		first = false

		if noStream {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func statsLiveRedraw() bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// statsRemote polls the one-shot stats endpoint through `cardinal serve`
// (the server has no usable remote stream, so the CLI polls ?stream=0
// every second instead of subscribing). The no-arg form lists containers
// remotely; with --no-stream it prints a single snapshot and returns,
// otherwise it polls until SIGINT.
func statsRemote(host string, names []string, noStream bool) {
	c := client.NewClient(host, remoteTokenResolved())
	if len(names) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		list, err := c.ListContainers(ctx, false)
		cancel()
		if err != nil {
			failf("remote %s: %v", host, err)
			return
		}
		for _, item := range list {
			name := ""
			if len(item.Names) > 0 {
				name = strings.TrimPrefix(item.Names[0], "/")
			}
			if name == "" {
				name = item.ID
			}
			names = append(names, name)
		}
		if len(names) == 0 {
			fmt.Println("No running containers")
			return
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	prevSnapshots := make(map[string]*container.StatsSnapshot)
	redraw := !noStream && statsLiveRedraw()
	prevLines := 0
	first := true
	for {
		if !first && redraw {
			fmt.Printf("\x1b[%dA\x1b[J", prevLines)
		}
		lines := 0
		showHeader := true
		for _, name := range names {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			st, err := c.Stats(ctx, name)
			cancel()
			if err != nil {
				if first {
					failf("remote %s: %v", host, err)
					return
				}
				continue
			}
			s := &container.ContainerStats{
				ContainerID:   st.ContainerID,
				Name:          st.Name,
				MemoryUsage:   st.MemoryUsage,
				MemoryLimit:   st.MemoryLimit,
				MemoryPercent: st.MemoryPercent,
				CPUCount:      st.CPUCount,
				PIDsCurrent:   st.PIDsCurrent,
				IOReadBytes:   st.IOReadBytes,
				IOWriteBytes:  st.IOWriteBytes,
				DiskUsage:     st.DiskUsage,
				Timestamp:     time.Now().UnixNano(),
			}
			prev := prevSnapshots[s.ContainerID]
			container.PrintContainerStats(s, prev, showHeader)
			if showHeader {
				lines += 2
			} else {
				lines++
			}
			prevSnapshots[s.ContainerID] = &container.StatsSnapshot{
				CPUUsage:  s.CPUUsage,
				Timestamp: s.Timestamp,
			}
			showHeader = false
		}
		prevLines = lines

		first = false
		if noStream {
			return
		}
		select {
		case <-sig:
			fmt.Fprintf(os.Stderr, "\n")
			return
		case <-time.After(1 * time.Second):
		}
	}
}
