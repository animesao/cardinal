//go:build linux

package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
		removeSystemdService()
		return
	}

	if install {
		installSystemdService()
	}

	network.EnsureNetBase()

	all, err := container.List(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, c := range all {
		if c.Restart != "always" {
			continue
		}
		fmt.Printf("  Starting %s (%s)... ", c.ID[:12], c.Name)
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

func ensureBootstrap() {
	if os.Geteuid() != 0 {
		return
	}
	if _, err := os.Stat("/etc/systemd/system/dck-bootstrap.service"); err == nil {
		return
	}
	installSystemdService()
}

func installSystemdService() {
	path, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting dck path: %v\n", err)
		os.Exit(1)
	}

	unit := fmt.Sprintf(`[Unit]
Description=dck containers bootstrap
After=network.target

[Service]
Type=oneshot
KillMode=process
ExecStart=%s bootstrap

[Install]
WantedBy=multi-user.target
`, path)

	unitPath := "/etc/systemd/system/dck-bootstrap.service"

	f, err := os.Create(unitPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", unitPath, err)
		fmt.Fprintf(os.Stderr, "Try running as root: sudo dck bootstrap --install\n")
		os.Exit(1)
	}
	f.WriteString(unit)
	f.Close()

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "dck-bootstrap").Run()

	fmt.Println("Systemd service installed. Enabled for next boot.")
}

func removeSystemdService() {
	unitPath := "/etc/systemd/system/dck-bootstrap.service"

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		fmt.Println("Systemd service not found.")
		return
	}

	exec.Command("systemctl", "stop", "dck-bootstrap").Run()
	exec.Command("systemctl", "disable", "dck-bootstrap").Run()
	os.Remove(unitPath)
	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Println("Systemd service stopped and removed: dck-bootstrap")
}
