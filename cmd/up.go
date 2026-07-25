//go:build linux

package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"dck/internal/builder"
	"dck/internal/config"
	"dck/internal/container"
	"dck/internal/image"
)

const depTimeout = 120 * time.Second
const depPollInterval = 2 * time.Second

func Up(args []string) {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	configPath := fs.String("f", "", "Path to config file")
	generate := fs.Bool("generate", false, "Generate dck.toml from existing containers")
	fs.Parse(args)

	if *generate {
		outPath := *configPath
		if outPath == "" {
			outPath = "dck.toml"
		}
		all, err := container.List(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing containers: %v\n", err)
			os.Exit(1)
		}
		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", outPath, err)
			os.Exit(1)
		}
		genCfg := config.Config{Container: make(map[string]config.ContainerConfig)}
		written := 0
		for _, c := range all {
			if c.Name == "" {
				continue
			}
			cc := config.ContainerConfig{
				Image:       c.ImageName + ":" + c.ImageTag,
				Restart:     c.Restart,
				Hostname:    c.Hostname,
				WorkDir:     c.WorkingDir,
				User:        c.User,
				Readonly:    c.ReadonlyRootfs,
				NoNewPrivs:  c.NoNewPrivileges,
				NetworkMode: c.NetworkMode,
				Entrypoint:  c.Entrypoint,
				Labels:      c.Labels,
				CapAdd:      c.CapAdd,
				CapDrop:     c.CapDrop,
				DNS:         c.DNS,
			}
			if c.CPUCount > 0 {
				cc.CPUs = c.CPUCount
			}
			if c.MemoryLimit > 0 {
				cc.Memory = bytesToHuman(c.MemoryLimit)
			}
			if len(c.Cmd) > 0 {
				cc.Command = strings.Join(c.Cmd, " ")
			}
			if c.StartupScript != "" {
				cc.Command = c.StartupScript
			}
			for _, p := range c.Ports {
				proto := ""
				if p.Protocol != "" && p.Protocol != "tcp" {
					proto = "/" + p.Protocol
				}
				cc.Ports = append(cc.Ports, fmt.Sprintf("%d:%d%s", p.HostPort, p.ContainerPort, proto))
			}
			for _, v := range c.Volumes {
				cc.Volumes = append(cc.Volumes, v.Source+":"+v.Target)
			}
			if len(c.Env) > 0 {
				cc.Env = make(map[string]string)
				for _, e := range c.Env {
					if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
						cc.Env[parts[0]] = parts[1]
					}
				}
			}
			if len(c.Sysctls) > 0 {
				cc.Sysctls = make(map[string]string)
				for k, v := range c.Sysctls {
					cc.Sysctls[k] = v
				}
			}
			if len(c.Ulimits) > 0 {
				cc.Ulimits = make(map[string]string)
				for _, u := range c.Ulimits {
					cc.Ulimits[u.Name] = fmt.Sprintf("%d:%d", u.Soft, u.Hard)
				}
			}
			if c.Healthcheck != nil {
				cc.Healthcheck = &config.HealthcheckConfig{
					Cmd:      c.Healthcheck.Cmd,
					Interval: c.Healthcheck.Interval,
					Retries:  c.Healthcheck.Retries,
					Timeout:  c.Healthcheck.Timeout,
				}
			}
			genCfg.Container[c.Name] = cc
			written++
		}
		if err := toml.NewEncoder(f).Encode(genCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outPath, err)
			os.Exit(1)
		}
		f.Close()
		fmt.Printf("Generated %s with %d containers\n", outPath, written)
		return
	}

	freeArgs := fs.Args()
	var filter string
	if len(freeArgs) > 0 {
		filter = freeArgs[0]
	}

	cfg, path, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using config: %s\n", path)

	// Load compose state for rename tracking
	composeState := loadComposeState(path)

	// Build ordered list by depends_on (topological sort)
	svcOrder := sortByDependencies(cfg.Container, filter)

	started := 0
	ready := make(map[string]bool) // service_name -> started/healthy

	for _, name := range svcOrder {
		cc := cfg.Container[name]

		if cc.Image == "" {
			fmt.Fprintf(os.Stderr, "container %q: no image specified\n", name)
			continue
		}

		// Wait for dependencies
		if len(cc.DependsOn) > 0 {
			for dep, cond := range cc.DependsOn {
				if dep == name {
					continue
				}
				if !ready[dep] {
					fmt.Printf("  %s: waiting for %s (%s)...\n", name, dep, cond)
				}
				if err := waitForDependency(dep, cond, ready); err != nil {
					fmt.Fprintf(os.Stderr, "  %s: dependency %s not ready: %v\n", name, dep, err)
					continue
				}
			}
		}

		img, err := image.Pull(cc.Image)
		if err != nil {
			fmt.Fprintf(os.Stderr, "container %q: error pulling image %s: %v\n", name, cc.Image, err)
			continue
		}

		existing := container.FindByName(name)
		if existing == nil {
			if oldID, ok := composeState[name]; ok {
				if c := findContainerByID(oldID); c != nil {
					if c.Name != name {
						if err := c.Rename(name); err != nil {
							fmt.Fprintf(os.Stderr, "  %s: rename error: %v\n", name, err)
						} else {
							fmt.Printf("  %s: renamed from %s\n", name, oldID[:12])
						}
					}
					existing = c
				}
			}
		}
		if existing != nil {
			if existing.Status == container.Running {
				fmt.Printf("  %s: already running\n", name)
				composeState[name] = existing.ID
				ready[name] = true
				started++
				continue
			}
			fmt.Printf("  %s: starting existing container...\n", name)
			existing.Status = container.Created
			if err := existing.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "  %s: error starting: %v\n", name, err)
				continue
			}
			fmt.Printf("  %s: started\n", name)
			composeState[name] = existing.ID
			ready[name] = true
			started++
			continue
		}

		opts := container.CreateOpts{
			Name:    name,
			Detach:  true,
			Restart: cc.Restart,
		}
		if opts.Restart == "" {
			opts.Restart = "always"
		}

		if cc.Command != "" {
			opts.Cmd = builder.SplitShellWords(cc.Command)
		}
		if cc.Hostname != "" {
			opts.Hostname = cc.Hostname
		}
		if cc.Memory != "" {
			mem, err := container.ParseMemoryString(cc.Memory)
			if err == nil {
				opts.MemoryLimit = mem
			}
		}
		if cc.CPUs > 0 {
			opts.CPUCount = cc.CPUs
		}
		if cc.WorkDir != "" {
			opts.WorkingDir = cc.WorkDir
		}

		for _, p := range cc.Ports {
			proto := "tcp"
			portSpec := p
			if parts := strings.SplitN(p, "/", 2); len(parts) == 2 {
				proto = parts[1]
				portSpec = parts[0]
			}
			parts := strings.SplitN(portSpec, ":", 2)
			if len(parts) == 2 {
				var host, cont int
				fmt.Sscanf(parts[0], "%d", &host)
				fmt.Sscanf(parts[1], "%d", &cont)
				if host > 0 && cont > 0 {
					opts.Ports = append(opts.Ports, container.PortMap{
						HostPort:      host,
						ContainerPort: cont,
						Protocol:      proto,
					})
				}
			}
		}

		for _, v := range cc.Volumes {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) == 2 {
				opts.Volumes = append(opts.Volumes, container.VolumeMount{
					Source: parts[0],
					Target: parts[1],
				})
			}
		}

		for k, v := range cc.Env {
			opts.Env = append(opts.Env, k+"="+v)
		}
		if cc.EnvFile != "" {
			fileEnv, err := container.ParseEnvFile(cc.EnvFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s: error reading env_file %s: %v\n", name, cc.EnvFile, err)
				continue
			}
			opts.Env = append(opts.Env, fileEnv...)
		}

		if cc.Healthcheck != nil {
			opts.Healthcheck = &container.HealthcheckConfig{
				Cmd:      cc.Healthcheck.Cmd,
				Interval: cc.Healthcheck.Interval,
				Retries:  cc.Healthcheck.Retries,
				Timeout:  cc.Healthcheck.Timeout,
			}
		}

		if cc.Entrypoint != "" {
			opts.Entrypoint = cc.Entrypoint
		}
		if cc.NetworkMode != "" {
			opts.NetworkMode = cc.NetworkMode
		}
		if len(cc.Labels) > 0 {
			opts.Labels = cc.Labels
		}
		if len(cc.CapAdd) > 0 {
			opts.CapAdd = cc.CapAdd
		}
		if len(cc.CapDrop) > 0 {
			opts.CapDrop = cc.CapDrop
		}
		if cc.User != "" {
			opts.User = cc.User
		}
		if cc.Readonly {
			opts.ReadonlyRootfs = true
		}
		if cc.NoNewPrivs {
			opts.NoNewPrivileges = true
		}
		if len(cc.Sysctls) > 0 {
			opts.Sysctls = cc.Sysctls
		}
		if len(cc.DNS) > 0 {
			opts.DNS = cc.DNS
		}
		if len(cc.Ulimits) > 0 {
			for uname, val := range cc.Ulimits {
				parts := strings.SplitN(val, ":", 2)
				if len(parts) == 2 {
					soft, _ := strconv.ParseUint(parts[0], 10, 64)
					hard, _ := strconv.ParseUint(parts[1], 10, 64)
					opts.Ulimits = append(opts.Ulimits, container.Ulimit{
						Name: uname,
						Soft: soft,
						Hard: hard,
					})
				}
			}
		}

		c := container.New(img, opts)
		if len(cc.Secrets) > 0 || len(cc.Configs) > 0 {
			tmpVolumes, tmpSecrets := container.ParseSecretsToVolumes(cc, *cfg)
			opts.Volumes = append(opts.Volumes, tmpVolumes...)
			c = container.New(img, opts)
			c.Secrets = tmpSecrets
		}
		if err := c.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error saving: %v\n", name, err)
			continue
		}
		if err := c.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error starting: %v\n", name, err)
			continue
		}
		fmt.Printf("  %s: created and started (%s)\n", name, c.ID[:12])
		composeState[name] = c.ID
		started++

		// Wait for own healthcheck if someone depends on us with service_healthy
		needHealthy := false
		for _, other := range cfg.Container {
			if other.DependsOn != nil {
				if cond, ok := other.DependsOn[name]; ok && cond == "service_healthy" {
					needHealthy = true
					break
				}
			}
		}
		if needHealthy {
			fmt.Printf("  %s: waiting for healthcheck...\n", name)
			if c.Healthcheck != nil {
				waitForHealthy(c, depTimeout)
			} else {
				fmt.Printf("  %s: no healthcheck configured, marking ready\n", name)
			}
		}

		ready[name] = true
	}

	saveComposeState(path, composeState)
	fmt.Printf("Up complete: %d containers started\n", started)

	ensureBootstrap()
}

func bytesToHuman(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%dg", b/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%dm", b/(1024*1024))
	default:
		return fmt.Sprintf("%dk", b/1024)
	}
}

type composeState map[string]string // service_name -> container_id

func composeStatePath(configPath string) string {
	return configPath + ".state"
}

func loadComposeState(configPath string) composeState {
	sp := composeStatePath(configPath)
	data, err := os.ReadFile(sp)
	if err != nil {
		return make(composeState)
	}
	var s composeState
	if err := json.Unmarshal(data, &s); err != nil {
		return make(composeState)
	}
	return s
}

func saveComposeState(configPath string, s composeState) {
	data, _ := json.Marshal(s)
	os.WriteFile(composeStatePath(configPath), data, 0644)
}

func findContainerByID(id string) *container.Container {
	c, err := container.Load(id)
	if err != nil {
		return nil
	}
	return c
}

// sortByDependencies returns service names in dependency order (topological sort).
// Services without deps come first; circular deps are broken by inserting remaining names.
func sortByDependencies(services map[string]config.ContainerConfig, filter string) []string {
	// Collect all names, apply filter
	names := make(map[string]bool)
	for n := range services {
		if filter == "" || n == filter {
			names[n] = true
		}
	}
	// Also include transitive dependencies that might be filtered out
	if filter != "" {
		collectDeps(filter, services, names)
	}

	// Kahn's algorithm (topological sort)
	inDegree := make(map[string]int)
	deps := make(map[string][]string) // dep -> list of services that depend on it

	for n := range names {
		inDegree[n] = 0
	}
	for n := range names {
		cc := services[n]
		for dep := range cc.DependsOn {
			if !names[dep] {
				continue
			}
			deps[dep] = append(deps[dep], n)
			inDegree[n]++
		}
	}

	queue := make([]string, 0)
	for n := range names {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}

	sort.Strings(queue)

	result := make([]string, 0, len(names))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)
		for _, dependent := range deps[n] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
		sort.Strings(queue)
	}

	// Add any remaining (circular deps or missing)
	for n := range names {
		found := false
		for _, r := range result {
			if r == n {
				found = true
				break
			}
		}
		if !found {
			result = append(result, n)
		}
	}

	return result
}

func collectDeps(name string, services map[string]config.ContainerConfig, collected map[string]bool) {
	cc, ok := services[name]
	if !ok {
		return
	}
	for dep := range cc.DependsOn {
		if !collected[dep] {
			collected[dep] = true
			collectDeps(dep, services, collected)
		}
	}
}

func waitForDependency(dep string, cond string, ready map[string]bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), depTimeout)
	defer cancel()

	for {
		if ready[dep] {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s", dep)
		case <-time.After(depPollInterval):
		}
	}
}

func waitForHealthy(c *container.Container, timeout time.Duration) {
	if c.Healthcheck == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interval := c.Healthcheck.Interval
	if interval == 0 {
		interval = 10
	}

	for {
		c2, err := container.Load(c.ID)
		if err != nil {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		if c2.Status == container.Running {
			// Check health — if no healthcheck output, assume healthy
			return
		}
		select {
		case <-ctx.Done():
			fmt.Printf("    healthcheck timeout for %s, continuing anyway\n", c.Name)
			return
		case <-time.After(time.Duration(interval) * time.Second):
		}
	}
}
