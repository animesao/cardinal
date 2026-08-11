//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"dck/internal/api"
	"dck/internal/container"
	"dck/internal/log"
	"dck/internal/orchestrator"
)

func Serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("p", 2375, "API port")
	host := fs.String("H", "127.0.0.1", "API host (external addresses require --token or DCK_TOKEN)")
	daemon := fs.Bool("d", false, "Run as daemon (background)")
	token := fs.String("token", "", "Authentication token (or DCK_TOKEN env)")
	certFile := fs.String("tls-cert", "", "TLS certificate file (requires --tls-key)")
	keyFile := fs.String("tls-key", "", "TLS private key file (requires --tls-cert)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing serve options: %v\n", err)
		os.Exit(1)
	}

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
	api.SetAuthToken(apiToken)
	api.SetServerVersion(version)
	orchestrator.SetAPIToken(apiToken)

	if *daemon {
		childArgs := []string{"serve", "-H", *host, "-p", fmt.Sprintf("%d", *port)}
		if *certFile != "" {
			childArgs = append(childArgs, "--tls-cert", *certFile, "--tls-key", *keyFile)
		}
		cmd := exec.Command("/proc/self/exe", append(childArgs, flag.Args()...)...)
		if apiToken != "" {
			cmd.Env = append(os.Environ(), "DCK_TOKEN="+apiToken)
		}
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
					log.Info("Stopping %s (%s)...", c.Name, shortID(c.ID))
					if err := c.Stop(); err != nil {
						log.Warn("failed to stop %s: %v", c.Name, err)
					}
				}
			}
		}
		container.CloseEvents()
		log.Info("Shutdown complete")
		os.Exit(0)
	}()

	if err := api.StartServerWithTLS(*port, *host, *certFile, *keyFile); err != nil {
		log.Error("Server error: %v", err)
		os.Exit(1)
	}
}

func parseHost(s string) (string, int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "tcp://")
	if host, portText, err := net.SplitHostPort(s); err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, fmt.Errorf("invalid port: %q", portText)
		}
		return host, port, nil
	}
	// Accept the common unbracketed IPv4/hostname form while requiring
	// brackets for IPv6 addresses (e.g. [::1]:2375).
	host, portText, ok := strings.Cut(s, ":")
	if !ok || strings.Contains(host, ":") {
		return "", 0, fmt.Errorf("invalid host format: %s (expected host:port)", s)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port: %q", portText)
	}
	return host, port, nil
}
