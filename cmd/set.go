//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"dck/internal/container"
)

func Set(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck set <container> [options]")
		fmt.Println("  --memory <lim>  Memory limit")
		fmt.Println("  --ram <lim>     Memory limit (alias for --memory)")
		fmt.Println("  --cpus <num>    CPU limit")
		fmt.Println("  --cpu <num>     CPU limit (alias for --cpus)")
		fmt.Println("  --disk <lim>    Disk limit")
		fmt.Println("  --restart       Restart policy")
		fmt.Println("  --restart-delay <duration> Delay before automatic restart")
		fmt.Println("  --workdir <dir> Working directory")
		fmt.Println("  -e K=V          Environment variable")
		fmt.Println("  --entrypoint    Override entrypoint")
		fmt.Println("  --user <uid>    Username or UID:GID")
		fmt.Println("  --readonly      Read-only rootfs")
		fmt.Println("  --no-new-privs  Disable privilege escalation")
		fmt.Println("  -h <name>       Hostname")
		fmt.Println("  --network <m>   Network mode")
		fmt.Println("  --startup <s>   Startup script or @filepath")
		os.Exit(1)
	}

	containerName := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("set", flag.ExitOnError)
	memory := fs.String("memory", "", "Memory limit (e.g. 512m, 1g, 2g)")
	ramAlias := fs.String("ram", "", "Memory limit (alias for --memory)")
	cpus := fs.Float64("cpus", -1, "CPU limit (e.g. 1.5)")
	cpuAlias := fs.Float64("cpu", -1, "CPU limit (alias for --cpus)")
	disk := fs.String("disk", "", "Disk limit (e.g. 1G, 512M)")
	restart := fs.String("restart", "", "Restart policy (no, always, on-failure, unless-stopped)")
	restartDelay := fs.String("restart-delay", "", "Delay before automatic restart (e.g. 10s, 1m)")
	workdir := fs.String("workdir", "", "Working directory inside container")
	var envVars stringSlice
	fs.Var(&envVars, "e", "Environment variables (key=val)")
	entrypoint := fs.String("entrypoint", "", "Override image entrypoint")
	user := fs.String("user", "", "Username or UID:GID")
	readonly := fs.Bool("readonly", false, "Make rootfs read-only")
	noNewPrivs := fs.Bool("no-new-privs", false, "Disable acquiring new privileges")
	hostname := fs.String("h", "", "Container hostname")
	networkMode := fs.String("network", "", "Network mode (bridge/none/host)")
	startup := fs.String("startup", "", "Startup script (inline or @filepath)")

	if err := fs.Parse(flagArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing set options: %v\n", err)
		os.Exit(1)
	}

	c, err := container.Load(containerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *memory == "" && *ramAlias != "" {
		memory = ramAlias
	}
	if *cpus < 0 && *cpuAlias >= 0 {
		cpus = cpuAlias
	}

	changed := false

	if *memory != "" {
		mem, err := container.ParseMemoryString(*memory)
		if err != nil || mem == 0 {
			fmt.Fprintf(os.Stderr, "Error: invalid memory value: %s\n", *memory)
			os.Exit(1)
		}
		c.MemoryLimit = mem
		changed = true
	}

	if *cpus >= 0 {
		c.CPUCount = *cpus
		changed = true
	}

	if *disk != "" {
		diskLimit, err := container.ParseDiskString(*disk)
		if err != nil || diskLimit == 0 {
			fmt.Fprintf(os.Stderr, "Error: invalid disk value: %s\n", *disk)
			os.Exit(1)
		}
		c.DiskLimit = diskLimit
		changed = true
	}

	if *restart != "" {
		c.Restart = *restart
		changed = true
	}

	if *restartDelay != "" {
		delay, err := time.ParseDuration(*restartDelay)
		if err != nil || delay <= 0 {
			fmt.Fprintf(os.Stderr, "Error: invalid restart delay %q (use e.g. 10s, 1m)\n", *restartDelay)
			os.Exit(1)
		}
		c.RestartDelay = *restartDelay
		changed = true
	}

	if *workdir != "" {
		c.WorkingDir = *workdir
		changed = true
	}

	if len(envVars) > 0 {
		c.Env = append(c.Env, envVars...)
		changed = true
	}

	if *entrypoint != "" {
		c.Entrypoint = *entrypoint
		changed = true
	}

	if *user != "" {
		c.User = *user
		changed = true
	}

	if *readonly {
		c.ReadonlyRootfs = true
		changed = true
	}

	if *noNewPrivs {
		c.NoNewPrivileges = true
		changed = true
	}

	if *hostname != "" {
		c.Hostname = *hostname
		changed = true
	}

	if *networkMode != "" {
		c.NetworkMode = *networkMode
		changed = true
	}

	if *startup != "" {
		startupValue := *startup
		if strings.HasPrefix(startupValue, "@") {
			path := strings.TrimPrefix(startupValue, "@")
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading startup script file %q: %v\n", path, err)
				os.Exit(1)
			}
			startupValue = string(data)
		}
		if len(startupValue) > 1024*1024 {
			fmt.Fprintln(os.Stderr, "Error: startup script is too large (maximum 1 MiB)")
			os.Exit(1)
		}
		c.StartupScript = startupValue
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified")
		return
	}

	wasRunning := c.Status == container.Running

	if wasRunning {
		if err := c.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping container: %v\n", err)
			os.Exit(1)
		}
	}

	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  %s: updated\n", shortID(c.ID))

	if wasRunning {
		c.Status = container.Created
		if err := c.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s: restarted\n", shortID(c.ID))
	}
	if c.Restart == "always" || c.Restart == "unless-stopped" {
		ensureBootstrap()
	}
}
