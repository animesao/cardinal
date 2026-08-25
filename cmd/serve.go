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

	"cardinal/internal/api"
	"cardinal/internal/container"
	"cardinal/internal/log"
	"cardinal/internal/orchestrator"
)

const cardinalServeServiceUnit = `[Unit]
Description=cardinal Docker-compatible API server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s serve -H %s -p %d%s
Restart=always
RestartSec=5s
KillMode=process
StandardOutput=journal
StandardError=journal
Environment=CARDINAL_TOKEN=%s

[Install]
WantedBy=multi-user.target
`

func Serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("p", 2375, "API port")
	host := fs.String("H", "127.0.0.1", "API host (external addresses require --token or CARDINAL_TOKEN)")
	daemon := fs.Bool("d", false, "Run as daemon (background)")
	token := fs.String("token", "", "Authentication token (or CARDINAL_TOKEN env)")
	certFile := fs.String("tls-cert", "", "TLS certificate file (requires --tls-key)")
	keyFile := fs.String("tls-key", "", "TLS private key file (requires --tls-cert)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing serve options: %v\n", err)
		os.Exit(1)
	}

	// Handle subcommands: on / off
	if len(fs.Args()) > 0 {
		switch fs.Args()[0] {
		case "on", "install":
			serveOn(*port, *host, *token, *certFile, *keyFile)
			return
		case "off", "remove":
			serveOff()
			return
		case "status":
			serveStatus()
			return
		}
	}

	// Allow override via CARDINAL_HOST env
	if envHost := os.Getenv("CARDINAL_HOST"); envHost != "" {
		if h, p, err := parseHost(envHost); err == nil {
			*host = h
			*port = p
		}
	}

	// Token: flag > env var > disabled
	apiToken := *token
	if apiToken == "" {
		apiToken = os.Getenv("CARDINAL_TOKEN")
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
			cmd.Env = append(os.Environ(), "CARDINAL_TOKEN="+apiToken)
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

const serveSystemdUnit = "/etc/systemd/system/cardinal-serve.service"

func serveOn(port int, host, token, certFile, keyFile string) {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: must run as root (sudo cardinal serve on)\n")
		os.Exit(1)
	}

	path, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting cardinal path: %v\n", err)
		os.Exit(1)
	}

	tlsArgs := ""
	if certFile != "" && keyFile != "" {
		tlsArgs = fmt.Sprintf(" --tls-cert %s --tls-key %s", certFile, keyFile)
	}

	tokenVal := token
	if tokenVal == "" {
		tokenVal = os.Getenv("CARDINAL_TOKEN")
	}

	unit := fmt.Sprintf(cardinalServeServiceUnit, path, host, port, tlsArgs, tokenVal)

	// Backup existing unit if present
	previous, readErr := os.ReadFile(serveSystemdUnit)
	hadPrevious := readErr == nil

	restoreUnit := func() error {
		if hadPrevious {
			return os.WriteFile(serveSystemdUnit, previous, 0644)
		}
		return os.Remove(serveSystemdUnit)
	}

	f, err := os.Create(serveSystemdUnit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", serveSystemdUnit, err)
		os.Exit(1)
	}
	if _, err := f.WriteString(unit); err != nil {
		_ = f.Close()
		_ = restoreUnit()
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", serveSystemdUnit, err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		_ = restoreUnit()
		fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", serveSystemdUnit, err)
		os.Exit(1)
	}

	run := func(args ...string) bool {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s %v: %v\n", args[0], args[1:], err)
			_ = restoreUnit()
			return false
		}
		return true
	}

	if !run("systemctl", "daemon-reload") {
		os.Exit(1)
	}
	if !run("systemctl", "enable", "cardinal-serve") {
		os.Exit(1)
	}
	if !run("systemctl", "start", "cardinal-serve") {
		os.Exit(1)
	}

	fmt.Println("cardinal-serve service installed and started.")
	fmt.Printf("  Port:    %d\n", port)
	fmt.Printf("  Host:    %s\n", host)
	fmt.Println("  Enabled: yes (starts on boot)")
	fmt.Println()
	fmt.Println("Stop:  cardinal serve off")
	fmt.Println("Logs:  journalctl -u cardinal-serve -f")
}

func serveOff() {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: must run as root (sudo cardinal serve off)\n")
		os.Exit(1)
	}

	if _, err := os.Stat(serveSystemdUnit); os.IsNotExist(err) {
		fmt.Println("cardinal-serve service not installed.")
		return
	}

	ignore := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	ignore("systemctl", "stop", "cardinal-serve")
	ignore("systemctl", "disable", "cardinal-serve")
	_ = os.Remove(serveSystemdUnit)
	ignore("systemctl", "daemon-reload")

	fmt.Println("cardinal-serve service stopped and removed.")
}

func serveStatus() {
	cmd := exec.Command("systemctl", "is-active", "cardinal-serve")
	out, err := cmd.Output()
	status := strings.TrimSpace(string(out))
	if err != nil || status == "" {
		status = "inactive"
	}

	cmd = exec.Command("systemctl", "is-enabled", "cardinal-serve")
	out, err = cmd.Output()
	enabled := strings.TrimSpace(string(out))
	if err != nil || enabled == "" {
		enabled = "disabled"
	}

	fmt.Printf("cardinal-serve: %s (boot: %s)\n", status, enabled)
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
