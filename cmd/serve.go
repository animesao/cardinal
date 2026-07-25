//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"dck/internal/api"
	"dck/internal/container"
	"dck/internal/log"
)

func Serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("p", 2375, "API port")
	host := fs.String("H", "0.0.0.0", "API host")
	daemon := fs.Bool("d", false, "Run as daemon (background)")
	token := fs.String("token", "", "Authentication token (or DCK_TOKEN env)")

	fs.Parse(args)

	// Allow override via DCK_HOST env
	if envHost := os.Getenv("DCK_HOST"); envHost != "" {
		if h, p, err := parseHost(envHost); err == nil {
			*host = h
			*port = p
		}
	}

	// Token: flag > env var > disabled
	apiToken := *token
	if apiToken == "" {
		apiToken = os.Getenv("DCK_TOKEN")
	}
	if apiToken != "" {
		api.SetAuthToken(apiToken)
	}

	if *daemon {
		cmd := exec.Command("/proc/self/exe", append([]string{"serve",
			"-H", *host,
			"-p", fmt.Sprintf("%d", *port),
		}, flag.Args()...)...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Daemon started with PID %d\n", cmd.Process.Pid)
		os.Exit(0)
	}

	// Graceful shutdown: wait for SIGINT/SIGTERM, then cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Info("Received shutdown signal, stopping all containers...")
		containers, err := container.List(true)
		if err == nil {
			for _, c := range containers {
				if c.Status == container.Running {
					log.Info("Stopping %s (%s)...", c.Name, c.ID[:12])
					c.Stop()
				}
			}
		}
		container.CloseEvents()
		log.Info("Shutdown complete")
		os.Exit(0)
	}()

	if err := api.StartServer(*port, *host); err != nil {
		log.Error("Server error: %v", err)
		os.Exit(1)
	}
}

func parseHost(s string) (string, int, error) {
	s = strings.TrimPrefix(s, "tcp://")
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid host format: %s (expected host:port)", s)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %v", err)
	}
	return parts[0], port, nil
}
