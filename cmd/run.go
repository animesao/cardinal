//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"dck/internal/builder"
	"dck/internal/container"
	"dck/internal/image"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func reorderRunFlags(args []string, fs *flag.FlagSet) []string {
	var flags, positional []string
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			positional = append(positional, args[i:]...)
			break
		}
		if !strings.HasPrefix(args[i], "-") {
			positional = append(positional, args[i])
			i++
			continue
		}
		name := strings.TrimLeft(args[i], "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			flags = append(flags, args[i])
			i++
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			positional = append(positional, args[i])
			i++
			continue
		}
		flags = append(flags, args[i])
		if ib, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && ib.IsBoolFlag() {
			i++
		} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i += 2
		} else {
			i++
		}
	}
	return append(flags, positional...)
}

func Run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	args = reorderRunFlags(args, fs)
	detach := fs.Bool("d", false, "Detach mode")
	name := fs.String("n", "", "Container name")
	interactive := fs.Bool("i", false, "Interactive mode")
	tty := fs.Bool("t", false, "Allocate TTY")
	rm := fs.Bool("rm", false, "Remove container on exit")
	hostname := fs.String("h", "", "Container hostname")
	restart := fs.String("restart", "", "Restart policy")
	var envVars stringSlice
	fs.Var(&envVars, "e", "Environment variables (key=val)")
	envFile := fs.String("env-file", "", "Path to .env file")
	portMapping := fs.String("p", "", "Port mapping (host:container[/protocol])")
	portsAlias := fs.String("ports", "", "Port mapping (host:container[/protocol])")
	volumeMounts := fs.String("v", "", "Volume mounts (src:dst)")
	volumeAlias := fs.String("volume", "", "Volume mounts (src:dst)")
	volAlias := fs.String("vol", "", "Volume mounts (src:dst)")
	memory := fs.String("memory", "", "Memory limit (e.g. 512m, 1g)")
	ramAlias := fs.String("ram", "", "Memory limit (e.g. 512m, 1g)")
	cpus := fs.Float64("cpus", 0, "CPU limit (number of CPUs, e.g. 1.5)")
	cpuAlias := fs.Float64("cpu", 0, "CPU limit (number of CPUs, e.g. 1.5)")
	disk := fs.String("disk", "", "Disk limit (e.g. 1G, 512M, 2T)")
	workdir := fs.String("workdir", "", "Working directory inside container")
	imageFlag := fs.String("image", "", "Container image")
	cmdFlag := fs.String("cmd", "", "Container command")
	commandFlag := fs.String("command", "", "Container command")

	// New flags
	entrypoint := fs.String("entrypoint", "", "Override image entrypoint")
	networkMode := fs.String("network", "", "Network mode (bridge/host/none)")
	var labels stringSlice
	fs.Var(&labels, "label", "Container labels (key=val)")
	fs.Var(&labels, "l", "Container labels (key=val)")
	var capAdd stringSlice
	fs.Var(&capAdd, "cap-add", "Add Linux capabilities (e.g. NET_ADMIN)")
	var capDrop stringSlice
	fs.Var(&capDrop, "cap-drop", "Drop Linux capabilities (e.g. ALL)")
	user := fs.String("user", "", "Username or UID:GID")
	readonly := fs.Bool("readonly", false, "Make rootfs read-only")
	noNewPrivs := fs.Bool("no-new-privs", false, "Disable acquiring new privileges")
	var sysctls stringSlice
	fs.Var(&sysctls, "sysctl", "Sysctl options (key=val)")
	var ulimits stringSlice
	fs.Var(&ulimits, "ulimit", "Ulimit options (name=soft:hard)")
	var dns stringSlice
	fs.Var(&dns, "dns", "DNS server (can repeat)")

	// Healthcheck flags
	healthcheckCmd := fs.String("healthcheck-cmd", "", "Health check command")
	healthcheckInterval := fs.Int("healthcheck-interval", 0, "Health check interval (seconds)")
	healthcheckRetries := fs.Int("healthcheck-retries", 0, "Health check retries")
	healthcheckTimeout := fs.Int("healthcheck-timeout", 0, "Health check timeout (seconds)")

	startupScript := fs.String("startup", "", "Startup script (inline script or @filepath)")

	fs.Parse(args)

	freeArgs := fs.Args()

	// Merge aliases
	if *portMapping == "" && *portsAlias != "" {
		portMapping = portsAlias
	}
	if *volumeMounts == "" && *volumeAlias != "" {
		volumeMounts = volumeAlias
	}
	if *volumeMounts == "" && *volAlias != "" {
		volumeMounts = volAlias
	}
	if *memory == "" && *ramAlias != "" {
		memory = ramAlias
	}
	if *cpus == 0 && *cpuAlias != 0 {
		cpus = cpuAlias
	}

	imageRef := *imageFlag
	hasImageFlag := *imageFlag != ""
	if !hasImageFlag {
		if len(freeArgs) < 1 {
			fmt.Println("Usage: dck run [opts] <image> [cmd...]")
			os.Exit(1)
		}
		imageRef = freeArgs[0]
	}

	var cmd []string
	if *cmdFlag != "" {
		cmd = builder.SplitShellWords(*cmdFlag)
	} else if *commandFlag != "" {
		cmd = builder.SplitShellWords(*commandFlag)
	} else if hasImageFlag && len(freeArgs) > 0 {
		cmd = freeArgs
	} else if !hasImageFlag && len(freeArgs) > 1 {
		cmd = freeArgs[1:]
	}

	img, err := image.Pull(imageRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
		os.Exit(1)
	}

	parsePort := func(s string) (container.PortMap, error) {
		proto := "tcp"
		if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
			proto = parts[1]
			s = parts[0]
		}
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return container.PortMap{}, fmt.Errorf("invalid port mapping: %s", s)
		}
		host, err := strconv.Atoi(parts[0])
		if err != nil {
			return container.PortMap{}, fmt.Errorf("invalid host port %q: %w", parts[0], err)
		}
		cont, err := strconv.Atoi(parts[1])
		if err != nil {
			return container.PortMap{}, fmt.Errorf("invalid container port %q: %w", parts[1], err)
		}
		return container.PortMap{HostPort: host, ContainerPort: cont, Protocol: proto}, nil
	}

	var ports []container.PortMap
	if *portMapping != "" {
		for _, p := range strings.Split(*portMapping, ",") {
			pm, err := parsePort(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			ports = append(ports, pm)
		}
	}

	var volumes []container.VolumeMount
	if *volumeMounts != "" {
		for _, v := range strings.Split(*volumeMounts, ",") {
			parts := strings.Split(v, ":")
			if len(parts) == 2 {
				volumes = append(volumes, container.VolumeMount{Source: parts[0], Target: parts[1]})
			}
		}
	}

	var env []string
	for _, e := range envVars {
		env = append(env, e)
	}
	if *envFile != "" {
		fileEnv, err := container.ParseEnvFile(*envFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading env file: %v\n", err)
			os.Exit(1)
		}
		env = append(env, fileEnv...)
	}

	memoryLimit, _ := container.ParseMemoryString(*memory)
	if *memory != "" && memoryLimit == 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid memory value: %s\n", *memory)
		os.Exit(1)
	}

	diskLimit, _ := container.ParseDiskString(*disk)
	if *disk != "" && diskLimit == 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid disk value: %s\n", *disk)
		os.Exit(1)
	}

	if *name != "" {
		if existing := container.FindByName(*name); existing != nil {
			fmt.Fprintf(os.Stderr, "Error: container with name %q already exists (%s)\n", *name, existing.ID[:12])
			os.Exit(1)
		}
	}

	// Parse labels
	labelMap := make(map[string]string)
	for _, l := range labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labelMap[parts[0]] = parts[1]
		}
	}

	// Parse sysctls
	sysctlMap := make(map[string]string)
	for _, s := range sysctls {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			sysctlMap[parts[0]] = parts[1]
		}
	}

	// Parse ulimits
	var parsedUlimits []container.Ulimit
	for _, u := range ulimits {
		parts := strings.SplitN(u, "=", 2)
		if len(parts) == 2 {
			limits := strings.SplitN(parts[1], ":", 2)
			if len(limits) == 2 {
				soft, _ := strconv.ParseUint(limits[0], 10, 64)
				hard, _ := strconv.ParseUint(limits[1], 10, 64)
				parsedUlimits = append(parsedUlimits, container.Ulimit{
					Name: parts[0],
					Soft: soft,
					Hard: hard,
				})
			}
		}
	}

	// Build healthcheck config
	var hc *container.HealthcheckConfig
	if *healthcheckCmd != "" {
		hc = &container.HealthcheckConfig{
			Cmd:      *healthcheckCmd,
			Interval: *healthcheckInterval,
			Retries:  *healthcheckRetries,
			Timeout:  *healthcheckTimeout,
		}
	}

	// Process startup script flag
	startupScriptVal := *startupScript
	if startupScriptVal != "" {
		if strings.HasPrefix(startupScriptVal, "@") {
			path := startupScriptVal[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading startup script file %q: %v\n", path, err)
				os.Exit(1)
			}
			startupScriptVal = string(data)
		}
	}

	opts := container.CreateOpts{
		Name:          *name,
		Cmd:           cmd,
		StartupScript: startupScriptVal,
		Ports:       ports,
		Volumes:     volumes,
		Env:         env,
		Hostname:    *hostname,
		Restart:     *restart,
		Detach:      *detach,
		Interactive: *interactive || *tty,
		TTY:         *tty,
		RemoveOnExit: *rm,
		MemoryLimit:  memoryLimit,
		CPUCount:     *cpus,
		DiskLimit:    diskLimit,
		WorkingDir:   *workdir,
		Healthcheck:  hc,
		Labels:       labelMap,
		CapAdd:       capAdd,
		CapDrop:      capDrop,
		User:         *user,
		ReadonlyRootfs: *readonly,
		NoNewPrivileges: *noNewPrivs,
		Sysctls:      sysctlMap,
		DNS:          dns,
		NetworkMode:  *networkMode,
		Entrypoint:   *entrypoint,
		Ulimits:      parsedUlimits,
	}

	c := container.New(img, opts)
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving container: %v\n", err)
		os.Exit(1)
	}

	if *restart == "always" || *restart == "unless-stopped" {
		ensureBootstrap()
	}

	if err := c.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
		os.Exit(1)
	}
}
