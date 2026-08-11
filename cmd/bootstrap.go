//go:build linux

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dck/internal/container"
	"dck/internal/network"
)

func Bootstrap(args []string) {
	install := false
	remove := false
	for _, a := range args {
		switch a {
		case "--install", "-i":
			install = true
		case "--remove", "-r":
			remove = true
		}
	}

	if remove {
		if err := removeSystemdService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing systemd service: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if install {
		if err := installSystemdService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing systemd service: %v\n", err)
			os.Exit(1)
		}
	}

	network.EnsureNetBase()

	all, err := container.List(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, c := range all {
		if !shouldBootstrap(c.Restart, c.StoppedByUser) {
			continue
		}
		fmt.Printf("  Starting %s (%s)... ", shortID(c.ID), c.Name)
		c.Status = container.Created
		if err := c.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println("OK")
		count++
	}

	fmt.Printf("Bootstrap complete: %d containers started\n", count)
}

func shouldBootstrap(policy string, stoppedByUser bool) bool {
	switch policy {
	case "always":
		return true
	case "unless-stopped":
		return !stoppedByUser
	default:
		return false
	}
}

func ensureBootstrap() {
	if os.Geteuid() != 0 {
		return
	}
	unitPath := "/etc/systemd/system/dck-bootstrap.service"
	if data, err := os.ReadFile(unitPath); err == nil && !strings.Contains(string(data), " supervisor") {
		if err := installSystemdService(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not migrate systemd service: %v\n", err)
		}
		return
	}
	if _, err := os.Stat(unitPath); err == nil {
		if exec.Command("systemctl", "is-active", "--quiet", "dck-bootstrap").Run() == nil {
			return
		}
		if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not reload systemd: %v\n", err)
			return
		}
		if err := exec.Command("systemctl", "start", "dck-bootstrap").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not start dck supervisor: %v\n", err)
		}
		return
	}
	if err := installSystemdService(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not install systemd service: %v\n", err)
	}
}

func installSystemdService() error {
	path, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get dck path: %w", err)
	}

	unit := fmt.Sprintf(`[Unit]
Description=dck container supervisor
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s supervisor
Restart=always
RestartSec=5s
KillMode=process
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, path)

	unitPath := "/etc/systemd/system/dck-bootstrap.service"
	previous, readErr := os.ReadFile(unitPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read existing %s: %w", unitPath, readErr)
	}
	hadPrevious := readErr == nil
	restoreUnit := func() error {
		if hadPrevious {
			return os.WriteFile(unitPath, previous, 0644)
		}
		return os.Remove(unitPath)
	}

	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("create %s: %w (try running as root: sudo dck bootstrap --install)", unitPath, err)
	}
	if _, err := f.WriteString(unit); err != nil {
		_ = f.Close()
		if restoreErr := restoreUnit(); restoreErr != nil {
			return fmt.Errorf("write %s: %w; restore failed: %v", unitPath, err, restoreErr)
		}
		return fmt.Errorf("write %s: %w", unitPath, err)
	}
	if err := f.Close(); err != nil {
		if restoreErr := restoreUnit(); restoreErr != nil {
			return fmt.Errorf("close %s: %w; restore failed: %v", unitPath, err, restoreErr)
		}
		return fmt.Errorf("close %s: %w", unitPath, err)
	}

	cleanupUnit := func(cause error) error {
		if restoreErr := restoreUnit(); restoreErr != nil {
			return fmt.Errorf("%w; restore %s: %v", cause, unitPath, restoreErr)
		}
		return cause
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return cleanupUnit(fmt.Errorf("systemctl daemon-reload: %w", err))
	}
	if err := exec.Command("systemctl", "enable", "dck-bootstrap").Run(); err != nil {
		return cleanupUnit(fmt.Errorf("enable dck-bootstrap: %w", err))
	}
	if err := exec.Command("systemctl", "restart", "dck-bootstrap").Run(); err != nil {
		return cleanupUnit(fmt.Errorf("restart dck-bootstrap: %w", err))
	}

	fmt.Println("Systemd service installed and started. Enabled for next boot.")
	return nil
}

func removeSystemdService() error {
	unitPath := "/etc/systemd/system/dck-bootstrap.service"

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		fmt.Println("Systemd service not found.")
		return nil
	}

	if err := exec.Command("systemctl", "stop", "dck-bootstrap").Run(); err != nil {
		return fmt.Errorf("stop dck-bootstrap: %w", err)
	}
	if err := exec.Command("systemctl", "disable", "dck-bootstrap").Run(); err != nil {
		return fmt.Errorf("disable dck-bootstrap: %w", err)
	}
	if err := os.Remove(unitPath); err != nil {
		return fmt.Errorf("remove %s: %w", unitPath, err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}

	fmt.Println("Systemd service stopped and removed: dck-bootstrap")
	return nil
}
