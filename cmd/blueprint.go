//go:build linux

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dck/internal/container"
	"dck/internal/image"
)

type blueprintRegistry struct {
	Version     int                       `json:"version"`
	Description string                    `json:"description"`
	BaseURL     string                    `json:"base_url"`
	Blueprints  map[string]blueprintEntry `json:"blueprints"`
}

type blueprintEntry struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Image       string `json:"image"`
	File        string `json:"file"`

	Source  string `json:"-"`
	RepoURL string `json:"-"`
	Branch  string `json:"-"`
}

type templateJSON struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Tag         string `json:"tag"`
	Command     string `json:"command"`
	Env         string `json:"env"`
	Ports       string `json:"ports"`
	Volumes     string `json:"volumes"`
	CapAdd      string `json:"cap_add"`
	Network     string `json:"network"`
	Memory      string `json:"memory"`
	CPUs        string `json:"cpus"`
	Restart     string `json:"restart"`
}

func Blueprint(args []string) {
	if len(args) < 1 {
		printBlueprintUsage()
		os.Exit(1)
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list", "ls":
		blueprintList()
	case "info", "show":
		blueprintInfo(subArgs)
	case "install", "i":
		blueprintInstall(subArgs)
	case "repo", "repos":
		if len(subArgs) < 1 {
			blueprintRepoUsage()
			os.Exit(1)
		}
		switch subArgs[0] {
		case "list", "ls":
			blueprintRepoList()
		case "add":
			blueprintRepoAdd(subArgs[1:])
		case "remove", "rm":
			blueprintRepoRemove(subArgs[1:])
		case "--help", "-h", "help":
			blueprintRepoUsage()
		default:
			fmt.Printf("unknown repo subcommand: %s\n", subArgs[0])
			blueprintRepoUsage()
			os.Exit(1)
		}
	case "--help", "-h", "help":
		printBlueprintUsage()
	default:
		fmt.Printf("unknown blueprint subcommand: %s\n", sub)
		printBlueprintUsage()
		os.Exit(1)
	}
}

func blueprintInfo(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: dck blueprint info <name>")
		os.Exit(1)
	}

	bpName := args[0]

	reg, err := fetchBlueprintRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching blueprint registry: %v\n", err)
		os.Exit(1)
	}

	bp, ok := reg.Blueprints[bpName]
	if !ok {
		// Try matching by prefix
		for name := range reg.Blueprints {
			if strings.HasPrefix(name, bpName) {
				bp = reg.Blueprints[name]
				bpName = name
				ok = true
				break
			}
		}
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "Blueprint %q not found\n", bpName)
		fmt.Println("Use 'dck blueprint list' to see available blueprints.")
		os.Exit(1)
	}

	repoBase := bp.RepoURL
	repoBranch := bp.Branch
	if repoBase == "" {
		repoBase = blueprintRepoURL
		repoBranch = "main"
	}
	tplURL := fmt.Sprintf("%s/%s/%s", repoBase, repoBranch, bp.File)
	tplData, err := fetchURL(tplURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching blueprint %q: %v\n", bpName, err)
		os.Exit(1)
	}

	var tpl templateJSON
	if err := json.Unmarshal([]byte(tplData), &tpl); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing blueprint %q: %v\n", bpName, err)
		os.Exit(1)
	}

	fmt.Printf("  %s\n\n", strings.ToUpper(bpName))
	fmt.Printf("  %s\n", bp.Description)
	fmt.Printf("  Category: %s\n\n", bp.Category)

	fmt.Printf("  Image:\n")
	fmt.Printf("    %s:%s\n\n", tpl.Image, tpl.Tag)

	if tpl.Command != "" {
		fmt.Printf("  Command:\n")
		fmt.Printf("    %s\n\n", tpl.Command)
	}

	if tpl.Ports != "" {
		fmt.Printf("  Ports:\n")
		for _, p := range strings.Split(tpl.Ports, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				fmt.Printf("    -p %s\n", p)
			}
		}
		fmt.Println()
	}

	if tpl.Volumes != "" {
		fmt.Printf("  Volumes:\n")
		for _, v := range strings.Split(tpl.Volumes, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				fmt.Printf("    -v %s\n", v)
			}
		}
		fmt.Println()
	}

	if tpl.Env != "" {
		fmt.Printf("  Environment variables (edit with -e KEY=VAL):\n")
		var envPairs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(tpl.Env), &envPairs); err == nil {
			for _, p := range envPairs {
				fmt.Printf("    -e %s=%s\n", p.Key, p.Value)
			}
		}
		fmt.Println()
	}

	if tpl.Memory != "" || tpl.CPUs != "" || tpl.Restart != "" {
		fmt.Printf("  Resources:\n")
		if tpl.Memory != "" {
			fmt.Printf("    --memory %s\n", tpl.Memory)
		}
		if tpl.CPUs != "" {
			fmt.Printf("    --cpus %s\n", tpl.CPUs)
		}
		if tpl.Restart != "" {
			fmt.Printf("    --restart %s\n", tpl.Restart)
		}
		if tpl.CapAdd != "" {
			for _, c := range strings.Split(tpl.CapAdd, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					fmt.Printf("    --cap-add %s\n", c)
				}
			}
		}
		if tpl.Network != "" {
			fmt.Printf("    --network %s\n", tpl.Network)
		}
		fmt.Println()
	}

	fmt.Printf("  Quick install:\n")
	fmt.Printf("    dck blueprint install %s\n\n", bpName)

	fmt.Printf("  Manual run:\n")
	manualArgs := []string{"dck run -d --restart " + tpl.Restart}
	if tpl.Restart == "" {
		manualArgs[0] = "dck run -d"
	}
	for _, p := range strings.Split(tpl.Ports, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			manualArgs = append(manualArgs, "-p "+p)
		}
	}
	for _, v := range strings.Split(tpl.Volumes, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			manualArgs = append(manualArgs, "-v "+v)
		}
	}
	if tpl.Memory != "" {
		manualArgs = append(manualArgs, "--memory "+tpl.Memory)
	}
	if tpl.CPUs != "" {
		manualArgs = append(manualArgs, "--cpus "+tpl.CPUs)
	}
	if tpl.CapAdd != "" {
		for _, c := range strings.Split(tpl.CapAdd, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				manualArgs = append(manualArgs, "--cap-add "+c)
			}
		}
	}
	if tpl.Network != "" {
		manualArgs = append(manualArgs, "--network "+tpl.Network)
	}
	if tpl.Env != "" {
		var envPairs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(tpl.Env), &envPairs); err == nil {
			for _, p := range envPairs {
				manualArgs = append(manualArgs, "-e "+p.Key+"="+p.Value)
			}
		}
	}
	manualArgs = append(manualArgs, tpl.Image+":"+tpl.Tag)
	if tpl.Command != "" {
		manualArgs = append(manualArgs, tpl.Command)
	}

	fmt.Printf("    %s\n", strings.Join(manualArgs, " \\\n      "))
	fmt.Println()
}

func printBlueprintUsage() {
	fmt.Println(`Blueprint commands:
  dck blueprint list              List available blueprints
  dck blueprint ls                Alias for list
  dck blueprint info <name>       Show blueprint details with examples
  dck blueprint show <name>       Alias for info
  dck blueprint install <name>    Install a blueprint
  dck blueprint i <name>          Alias for install
  dck blueprint repo list         List configured repositories
  dck blueprint repo add <url>    Add a custom repository
  dck blueprint repo remove <n>   Remove a repository

Options for install:
  -n, --name <name>               Container name (default: blueprint name)
  -d                              Detach (background, default: true)
  --memory <limit>                Override memory limit
  --cpus <count>                  Override CPU count
  -e, --env KEY=VAL               Override environment variable (can repeat)
  -y, --yes                       Skip interactive prompts`)
}

func blueprintList() {
	reg, err := fetchBlueprintRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching blueprint registry: %v\n", err)
		os.Exit(1)
	}

	if len(reg.Blueprints) == 0 {
		fmt.Println("No blueprints available.")
		return
	}

	fmt.Println("Available blueprints:")
	fmt.Println()

	// Group by category
	type catEntry struct {
		Name        string
		Image       string
		Description string
		Source      string
	}
	categories := make(map[string][]catEntry)
	catOrder := make([]string, 0)

	for _, bp := range reg.Blueprints {
		if _, ok := categories[bp.Category]; !ok {
			catOrder = append(catOrder, bp.Category)
		}
		categories[bp.Category] = append(categories[bp.Category], catEntry{
			Name:        bp.Name,
			Image:       bp.Image,
			Description: bp.Description,
			Source:      bp.Source,
		})
	}

	for _, cat := range catOrder {
		entries := categories[cat]
		fmt.Printf("  %s:\n", strings.ToUpper(cat))
		for _, e := range entries {
			sourceTag := ""
			if e.Source != "" && e.Source != "official" {
				sourceTag = fmt.Sprintf(" [%s]", e.Source)
			}
			fmt.Printf("    \033[1m%-20s\033[0m %-30s %s%s\n", e.Name, e.Image, e.Description, sourceTag)
		}
		fmt.Println()
	}
}

func blueprintInstall(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: dck blueprint install <name> [--name <container-name>]")
		os.Exit(1)
	}

	bpName := args[0]
	containerName := bpName
	detach := true
	memoryOverride := ""
	cpusOverride := 0.0
	noPrompt := false
	var envOverrides []string

	// Parse remaining flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--name":
			if i+1 < len(args) {
				i++
				containerName = args[i]
			}
		case "-d":
			detach = true
		case "--memory":
			if i+1 < len(args) {
				i++
				memoryOverride = args[i]
			}
		case "--cpus":
			if i+1 < len(args) {
				i++
				cpusOverride, _ = strconv.ParseFloat(args[i], 64)
			}
		case "-e", "--env":
			if i+1 < len(args) {
				i++
				envOverrides = append(envOverrides, args[i])
			}
		case "-y", "--yes":
			noPrompt = true
		}
	}

	reg, err := fetchBlueprintRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching blueprint registry: %v\n", err)
		os.Exit(1)
	}

	bp, ok := reg.Blueprints[bpName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Blueprint %q not found in registry\n", bpName)
		fmt.Println("Use 'dck blueprint list' to see available blueprints.")
		os.Exit(1)
	}

	// Fetch the template JSON from the blueprint's source repository
	repoBase := bp.RepoURL
	repoBranch := bp.Branch
	if repoBase == "" {
		repoBase = blueprintRepoURL
		repoBranch = "main"
	}
	tplURL := fmt.Sprintf("%s/%s/%s", repoBase, repoBranch, bp.File)
	tplData, err := fetchURL(tplURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching blueprint %q: %v\n", bpName, err)
		os.Exit(1)
	}

	var tpl templateJSON
	if err := json.Unmarshal([]byte(tplData), &tpl); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing blueprint %q: %v\n", bpName, err)
		os.Exit(1)
	}

	fmt.Printf("Installing blueprint: %s\n", bpName)
	fmt.Printf("  Image: %s\n", bp.Image)
	fmt.Printf("  Description: %s\n", bp.Description)

	// Pull image
	imageRef := bp.Image
	img, err := image.Pull(imageRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error pulling image %s: %v\n", imageRef, err)
		os.Exit(1)
	}
	fmt.Printf("  Image pulled: %s\n", imageRef)

	// Check if container name already exists
	if existing := container.FindByName(containerName); existing != nil {
		fmt.Fprintf(os.Stderr, "Error: container with name %q already exists (%s)\n", containerName, existing.ID[:12])
		os.Exit(1)
	}

	// Parse command
	var cmd []string
	if tpl.Command != "" {
		cmd = strings.Fields(tpl.Command)
	}

	// Collect used host ports from existing containers
	usedPorts := make(map[int]bool)
	if allContainers, err := container.List(true); err == nil {
		for _, c := range allContainers {
			for _, p := range c.Ports {
				usedPorts[p.HostPort] = true
			}
		}
	}

	// Parse and resolve ports
	type portEntry struct {
		HostPort      int
		ContainerPort int
		Protocol      string
	}
	var portEntries []portEntry
	if tpl.Ports != "" {
		for _, p := range strings.Split(tpl.Ports, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			proto := "tcp"
			portSpec := p
			if parts := strings.SplitN(p, "/", 2); len(parts) == 2 {
				proto = parts[1]
				portSpec = parts[0]
			}
			parts := strings.SplitN(portSpec, ":", 2)
			if len(parts) == 2 {
				host, _ := strconv.Atoi(parts[0])
				cont, _ := strconv.Atoi(parts[1])
				if host > 0 && cont > 0 {
					portEntries = append(portEntries, portEntry{
						HostPort:      host,
						ContainerPort: cont,
						Protocol:      proto,
					})
				}
			}
		}
	}

	// Auto-resolve port conflicts and prompt
	for i := range portEntries {
		base := portEntries[i].HostPort
		for usedPorts[portEntries[i].HostPort] {
			portEntries[i].HostPort++
		}
		if portEntries[i].HostPort != base {
			fmt.Printf("  Port %d is in use, using %d instead\n", base, portEntries[i].HostPort)
		}
		usedPorts[portEntries[i].HostPort] = true
	}

	if len(portEntries) > 0 && !noPrompt {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println()
		fmt.Println("  Port mappings (press Enter to keep default):")
		for i := range portEntries {
			prompt := fmt.Sprintf("    %s port (host:%d container:%d): ",
				portEntries[i].Protocol, portEntries[i].HostPort, portEntries[i].ContainerPort)
			fmt.Print(prompt)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				parts := strings.SplitN(input, ":", 2)
				if len(parts) == 2 {
					h, err1 := strconv.Atoi(parts[0])
					c, err2 := strconv.Atoi(parts[1])
					if err1 == nil && err2 == nil && h > 0 && c > 0 {
						// Check if the new host port is free
						for usedPorts[h] {
							h++
						}
						usedPorts[portEntries[i].HostPort] = false
						usedPorts[h] = true
						portEntries[i].HostPort = h
						portEntries[i].ContainerPort = c
					}
				}
			}
		}
		fmt.Println()
	}

	var ports []container.PortMap
	for _, pe := range portEntries {
		ports = append(ports, container.PortMap{
			HostPort:      pe.HostPort,
			ContainerPort: pe.ContainerPort,
			Protocol:      pe.Protocol,
		})
	}

	// Parse volumes
	var volumes []container.VolumeMount
	if tpl.Volumes != "" {
		for _, v := range strings.Split(tpl.Volumes, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			parts := strings.SplitN(v, ":", 2)
			if len(parts) == 2 {
				volumes = append(volumes, container.VolumeMount{
					Source: parts[0],
					Target: parts[1],
				})
			}
		}
	}

	// Parse environment variables
	var envPairs []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if tpl.Env != "" {
		json.Unmarshal([]byte(tpl.Env), &envPairs)
	}

	// Apply CLI env overrides
	for _, o := range envOverrides {
		parts := strings.SplitN(o, "=", 2)
		if len(parts) == 2 {
			found := false
			for i := range envPairs {
				if envPairs[i].Key == parts[0] {
					envPairs[i].Value = parts[1]
					found = true
					break
				}
			}
			if !found {
				envPairs = append(envPairs, struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}{parts[0], parts[1]})
			}
		}
	}

	// Auto-detect public IP for host/domain vars
	publicIP := ""
	if !noPrompt {
		for _, p := range envPairs {
			key := strings.ToUpper(p.Key)
			if strings.Contains(key, "HOST") || strings.Contains(key, "DOMAIN") || strings.Contains(key, "IP") || strings.Contains(key, "ADDRESS") || strings.Contains(key, "URL") {
				publicIP = getPublicIP()
				if publicIP != "" {
					break
				}
			}
		}
	}

	// Interactive env editing
	if len(envPairs) > 0 && !noPrompt {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println()
		fmt.Println("  Environment variables (press Enter to keep default):")
		for i := range envPairs {
			defaultVal := envPairs[i].Value
			key := strings.ToUpper(envPairs[i].Key)
			if strings.Contains(key, "HOST") || strings.Contains(key, "DOMAIN") || strings.Contains(key, "IP") || strings.Contains(key, "ADDRESS") || strings.Contains(key, "URL") {
				if publicIP != "" && (defaultVal == "" || defaultVal == "vpn.example.com" || strings.HasPrefix(defaultVal, "your-") || strings.Contains(defaultVal, "example")) {
					defaultVal = publicIP
				}
			}
			fmt.Printf("    %s [%s]: ", envPairs[i].Key, defaultVal)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				envPairs[i].Value = input
			} else if defaultVal != envPairs[i].Value {
				envPairs[i].Value = defaultVal
			}
		}
		fmt.Println()
	}

	var env []string
	for _, p := range envPairs {
		env = append(env, p.Key+"="+p.Value)
	}

	// Memory
	memoryLimit := int64(0)
	memStr := memoryOverride
	if memStr == "" {
		memStr = tpl.Memory
	}
	if memStr != "" {
		memoryLimit, _ = container.ParseMemoryString(memStr)
	}

	// CPUs
	cpus := cpusOverride
	if cpus == 0 && tpl.CPUs != "" {
		cpus, _ = strconv.ParseFloat(tpl.CPUs, 64)
	}

	// Network mode
	networkMode := tpl.Network

	// Restart policy
	restart := tpl.Restart
	if restart == "" {
		restart = "no"
	}

	// Parse capabilities
	var capAdd []string
	needsNetAdmin := false
	if tpl.CapAdd != "" {
		for _, c := range strings.Split(tpl.CapAdd, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				capAdd = append(capAdd, c)
				if strings.ToUpper(c) == "NET_ADMIN" {
					needsNetAdmin = true
				}
			}
		}
	}

	// Enable IP forwarding and UFW forwarding for VPN containers
	if needsNetAdmin {
		enableIPForward()
		enableUFWForward()
	}

	opts := container.CreateOpts{
		Name:        containerName,
		Cmd:         cmd,
		Ports:       ports,
		Volumes:     volumes,
		Env:         env,
		Restart:     restart,
		Detach:      detach,
		MemoryLimit: memoryLimit,
		CPUCount:    cpus,
		CapAdd:      capAdd,
		NetworkMode: networkMode,
	}

	c := container.New(img, opts)
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving container: %v\n", err)
		os.Exit(1)
	}

	if err := c.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Container created: %s (%s)\n", c.Name, c.ID[:12])

	// Auto-open ports in UFW/firewall
	opened := 0
	for _, pe := range portEntries {
		if ufwAllowPort(pe.HostPort, pe.Protocol) {
			if opened == 0 {
				fmt.Println("  Firewall rules:")
			}
			fmt.Printf("    ufw allow %d/%s\n", pe.HostPort, pe.Protocol)
			opened++
		}
	}

	fmt.Println("  Use 'dck ps' to see running containers")
	fmt.Println("  Use 'dck logs " + c.Name + "' to view logs")
}

func fetchBlueprintRegistry() (*blueprintRegistry, error) {
	cfg := loadBlueprintRepos()

	merged := &blueprintRegistry{
		Blueprints: make(map[string]blueprintEntry),
	}

	var errs []string

	for _, repo := range cfg.Repos {
		if !repo.Enabled {
			continue
		}
		registryURL := fmt.Sprintf("%s/%s/registry.json", repo.URL, repo.Branch)
		data, err := fetchURL(registryURL)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", repo.Name, err))
			continue
		}

		var reg blueprintRegistry
		if err := json.Unmarshal([]byte(data), &reg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parse error", repo.Name))
			continue
		}

		if merged.Description == "" && reg.Description != "" {
			merged.Description = reg.Description
		}

		for name, bp := range reg.Blueprints {
			bp.Source = repo.Name
			bp.RepoURL = repo.URL
			bp.Branch = repo.Branch

			if _, exists := merged.Blueprints[name]; exists {
				key := fmt.Sprintf("%s/%s", repo.Name, name)
				merged.Blueprints[key] = bp
			} else {
				merged.Blueprints[name] = bp
			}
		}
	}

	if len(merged.Blueprints) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("failed to fetch from all repositories:\n  %s", strings.Join(errs, "\n  "))
	}

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch %s\n", e)
	}

	return merged, nil
}

func enableUFWForward() {
	if _, err := exec.LookPath("ufw"); err != nil {
		return
	}
	def := "/etc/default/ufw"
	data, err := os.ReadFile(def)
	if err != nil {
		return
	}
	content := string(data)
	if strings.Contains(content, "DEFAULT_FORWARD_POLICY=\"ACCEPT\"") {
		return
	}
	content = strings.ReplaceAll(content, "DEFAULT_FORWARD_POLICY=\"DROP\"", "DEFAULT_FORWARD_POLICY=\"ACCEPT\"")
	os.WriteFile(def, []byte(content), 0644)
	exec.Command("ufw", "reload").Run()
	fmt.Println("  Enabled UFW forwarding (DEFAULT_FORWARD_POLICY=ACCEPT)")

	iface := defaultRouteInterface()
	if iface == "" {
		iface = "eth0"
	}
	if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-o", iface, "-j", "MASQUERADE").Run(); err != nil {
		exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", iface, "-j", "MASQUERADE").Run()
		fmt.Printf("  Added iptables MASQUERADE rule for %s\n", iface)
	}
	exec.Command("iptables", "-A", "FORWARD", "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()
	exec.Command("iptables", "-A", "FORWARD", "-i", "wg0", "-j", "ACCEPT").Run()
}

func defaultRouteInterface() string {
	out, err := exec.Command("ip", "route", "get", "8.8.8.8").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func ufwAllowPort(port int, proto string) bool {
	if _, err := exec.LookPath("ufw"); err != nil {
		return false
	}
	spec := fmt.Sprintf("%d/%s", port, proto)
	// Check if already allowed
	check := exec.Command("ufw", "status", "verbose")
	out, err := check.Output()
	if err == nil && strings.Contains(string(out), spec) {
		return false
	}
	cmd := exec.Command("ufw", "allow", spec)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func enableIPForward() {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return
	}
	if strings.TrimSpace(string(data)) == "1" {
		return
	}
	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	fmt.Println("  Enabled IP forwarding (net.ipv4.ip_forward = 1)")
}

func getPublicIP() string {
	// Try multiple services
	services := []string{
		"https://ifconfig.me",
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			ip := strings.TrimSpace(string(body))
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Fallback: try hostname -I
	cmd := exec.Command("hostname", "-I")
	out, err := cmd.Output()
	if err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			return fields[0]
		}
	}

	return ""
}
