package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Version  string                       `yaml:"version"`
	Services map[string]composeService    `yaml:"services"`
	Volumes  map[string]composeVolume     `yaml:"volumes"`
	Networks map[string]composeNetwork    `yaml:"networks"`
	Secrets  map[string]composeSecretSpec  `yaml:"secrets"`
	Configs  map[string]composeConfigSpec  `yaml:"configs"`
}

type composeService struct {
	Image       string                  `yaml:"image"`
	ContainerName string                `yaml:"container_name"`
	Ports       []string                `yaml:"ports"`
	Environment interface{}             `yaml:"environment"`
	EnvFile     interface{}             `yaml:"env_file"`
	Volumes     []interface{}           `yaml:"volumes"`
	DependsOn   interface{}             `yaml:"depends_on"`
	Restart     string                  `yaml:"restart"`
	Command     interface{}             `yaml:"command"`
	Entrypoint  interface{}             `yaml:"entrypoint"`
	WorkingDir  string                  `yaml:"working_dir"`
	User        string                  `yaml:"user"`
	DNS         interface{}             `yaml:"dns"`
	CapAdd      []string                `yaml:"cap_add"`
	CapDrop     []string                `yaml:"cap_drop"`
	Labels      map[string]string       `yaml:"labels"`
	Healthcheck *composeHealthcheck     `yaml:"healthcheck"`
	Networks    interface{}             `yaml:"networks"`
	NetworkMode string                  `yaml:"network_mode"`
	ReadOnly    bool                    `yaml:"read_only"`
	StdinOpen   bool                    `yaml:"stdin_open"`
	TTY         bool                    `yaml:"tty"`
	StopSignal  string                  `yaml:"stop_signal"`
	StopGracePeriod string              `yaml:"stop_grace_period"`
	Sysctls     map[string]string       `yaml:"sysctls"`
	Ulimits     map[string]composeUlimit `yaml:"ulimits"`
	Memory      string                  `yaml:"memory"`
	MemLimit    string                  `yaml:"mem_limit"`
	CPUs        float64                 `yaml:"cpus"`
	CPUSet      string                  `yaml:"cpuset"`
	Privileged  bool                    `yaml:"privileged"`
	Hostname    string                  `yaml:"hostname"`
	Expose      []string                `yaml:"expose"`
	ExtraHosts  []string                `yaml:"extra_hosts"`

	// Deploy section
	Deploy *composeDeployConfig `yaml:"deploy"`

	// Secrets / Configs (support both []string and []map)
	SecretsRaw interface{} `yaml:"secrets"`
	ConfigsRaw interface{} `yaml:"configs"`

	Build       interface{}            `yaml:"build"`
}

type composeDeployConfig struct {
	Mode          string                    `yaml:"mode"`
	Replicas      int                       `yaml:"replicas"`
	Resources     *composeResources         `yaml:"resources"`
	RestartPolicy *composeRestartPolicy     `yaml:"restart_policy"`
	UpdateConfig  *composeUpdateConfig      `yaml:"update_config"`
	Placement     *composePlacement         `yaml:"placement"`
	Labels        map[string]string         `yaml:"labels"`
}

type composeResources struct {
	Limits       *composeResourceSpec `yaml:"limits"`
	Reservations *composeResourceSpec `yaml:"reservations"`
}

type composeResourceSpec struct {
	CPUs   interface{} `yaml:"cpus"`   // string or float64
	Memory string      `yaml:"memory"`
}

type composeRestartPolicy struct {
	Condition   string `yaml:"condition"`
	Delay       string `yaml:"delay"`
	MaxAttempts int    `yaml:"max_attempts"`
	Window      string `yaml:"window"`
}

type composeUpdateConfig struct {
	Parallelism     int     `yaml:"parallelism"`
	Delay           string  `yaml:"delay"`
	FailureAction   string  `yaml:"failure_action"`
	Monitor         string  `yaml:"monitor"`
	MaxFailureRatio float64 `yaml:"max_failure_ratio"`
	Order           string  `yaml:"order"`
}

type composePlacement struct {
	Constraints []string `yaml:"constraints"`
	Preferences []string `yaml:"preferences"`
	MaxReplicas int      `yaml:"max_replicas"`
}

type composeSecretSpec struct {
	File     string `yaml:"file"`
	External bool   `yaml:"external"`
	Name     string `yaml:"name"`
}

type composeConfigSpec struct {
	File     string `yaml:"file"`
	External bool   `yaml:"external"`
	Name     string `yaml:"name"`
}

type composeSecretRef struct {
	Source string `yaml:"source"`
	Target string `yaml:"target,omitempty"`
	UID    string `yaml:"uid,omitempty"`
	GID    string `yaml:"gid,omitempty"`
	Mode   uint32 `yaml:"mode,omitempty"`
}

type composeConfigRef struct {
	Source string `yaml:"source"`
	Target string `yaml:"target,omitempty"`
	UID    string `yaml:"uid,omitempty"`
	GID    string `yaml:"gid,omitempty"`
	Mode   uint32 `yaml:"mode,omitempty"`
}

type composeVolume struct {
	Driver     string            `yaml:"driver"`
	DriverOpts map[string]string `yaml:"driver_opts"`
}

type composeNetwork struct {
	Driver     string            `yaml:"driver"`
	DriverOpts map[string]string `yaml:"driver_opts"`
}

type composeHealthcheck struct {
	Test     interface{} `yaml:"test"`
	Interval string      `yaml:"interval"`
	Timeout  string      `yaml:"timeout"`
	Retries  int         `yaml:"retries"`
}

type composeUlimit struct {
	Soft int `yaml:"soft"`
	Hard int `yaml:"hard"`
}

type composeVolumeMount struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

// LoadCompose loads a docker-compose.yaml file and returns a Config.
func LoadCompose(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse compose YAML: %w", err)
	}

	cfg := &Config{
		Secrets:    make(map[string]SecretSpec),
		Configs:    make(map[string]ConfigSpec),
		Container:  make(map[string]ContainerConfig),
	}

	// Top-level secrets
	for name, s := range cf.Secrets {
		cfg.Secrets[name] = SecretSpec(s)
	}

	// Top-level configs
	for name, c := range cf.Configs {
		cfg.Configs[name] = ConfigSpec(c)
	}

	for name, svc := range cf.Services {
		cc := ContainerConfig{
			Image:       svc.Image,
			Restart:     svc.Restart,
			Hostname:    svc.Hostname,
			WorkDir:     svc.WorkingDir,
			User:        svc.User,
			NetworkMode: svc.NetworkMode,
			Readonly:    svc.ReadOnly,
			NoNewPrivs:  false,
			Labels:      svc.Labels,
			CapAdd:      svc.CapAdd,
			CapDrop:     svc.CapDrop,
			Sysctls:     svc.Sysctls,
			DNS:         toStringSlice(svc.DNS),
		}

		if svc.ContainerName != "" {
			cc.Hostname = svc.ContainerName
		}

		mem := svc.Memory
		if mem == "" {
			mem = svc.MemLimit
		}
		cc.Memory = mem

		cc.CPUs = svc.CPUs

		for _, p := range svc.Ports {
			cc.Ports = append(cc.Ports, normalizePort(p))
		}

		cc.Env = parseComposeEnv(svc.Environment)
		cc.EnvFile = parseComposeEnvFile(svc.EnvFile)
		cc.Command = parseComposeCommand(svc.Command)

		if ep := parseComposeCommand(svc.Entrypoint); ep != "" {
			cc.Entrypoint = ep
		}

		for _, v := range svc.Volumes {
			switch vol := v.(type) {
			case string:
				cc.Volumes = append(cc.Volumes, vol)
			case map[string]interface{}:
				src, _ := vol["source"].(string)
				tgt, _ := vol["target"].(string)
				if src == "" {
					src, _ = vol["src"].(string)
				}
				if src != "" && tgt != "" {
					mount := src + ":" + tgt
					if ro, ok := vol["read_only"].(bool); ok && ro {
						mount += ":ro"
					}
					cc.Volumes = append(cc.Volumes, mount)
				}
			}
		}

		if svc.Healthcheck != nil {
			cc.Healthcheck = parseComposeHealthcheck(svc.Healthcheck)
		}

		if len(svc.Ulimits) > 0 {
			cc.Ulimits = make(map[string]string)
			for uname, ul := range svc.Ulimits {
				cc.Ulimits[uname] = fmt.Sprintf("%d:%d", ul.Soft, ul.Hard)
			}
		}

		cc.Restart = normalizeRestart(cc.Restart)

		// --- Deploy section ---
		if svc.Deploy != nil {
			cc.Deploy = parseDeployConfig(svc.Deploy)
			// Pull replicas from deploy section if set
			if svc.Deploy.Replicas > 0 {
				cc.Replicas = svc.Deploy.Replicas
			}
			// Pull resources from deploy section
			if svc.Deploy.Resources != nil {
				r := svc.Deploy.Resources
				if r.Limits != nil {
					if r.Limits.Memory != "" && cc.Memory == "" {
						cc.Memory = r.Limits.Memory
					}
				}
			}
		}

		// --- Depends on ---
		cc.DependsOn = parseComposeDependsOn(svc.DependsOn)

		// --- Secrets ---
		cc.Secrets = parseComposeSecrets(svc.SecretsRaw, cf.Secrets)

		// --- Configs ---
		cc.Configs = parseComposeConfigs(svc.ConfigsRaw, cf.Configs)

		cfg.Container[name] = cc
	}

	return cfg, nil
}

func parseDeployConfig(d *composeDeployConfig) *DeployConfig {
	dc := &DeployConfig{
		Replicas: d.Replicas,
		Mode:     d.Mode,
	}

	if d.Resources != nil {
		dc.Resources = &ResourcesConfig{}
		if d.Resources.Limits != nil {
			dc.Resources.Limits = &ResourceSpec{
				Memory: d.Resources.Limits.Memory,
				CPUs:   parseCPUs(d.Resources.Limits.CPUs),
			}
		}
		if d.Resources.Reservations != nil {
			dc.Resources.Reservations = &ResourceSpec{
				Memory: d.Resources.Reservations.Memory,
				CPUs:   parseCPUs(d.Resources.Reservations.CPUs),
			}
		}
	}

	if d.RestartPolicy != nil {
		dc.RestartPolicy = &RestartPolicyConfig{
			Condition:   d.RestartPolicy.Condition,
			Delay:       d.RestartPolicy.Delay,
			MaxAttempts: d.RestartPolicy.MaxAttempts,
			Window:      d.RestartPolicy.Window,
		}
	}

	if d.UpdateConfig != nil {
		dc.UpdateConfig = &UpdateConfig{
			Parallelism:     d.UpdateConfig.Parallelism,
			Delay:           d.UpdateConfig.Delay,
			FailureAction:   d.UpdateConfig.FailureAction,
			Monitor:         d.UpdateConfig.Monitor,
			MaxFailureRatio: d.UpdateConfig.MaxFailureRatio,
			Order:           d.UpdateConfig.Order,
		}
	}

	if d.Placement != nil {
		dc.Placement = &PlacementConfig{
			Constraints: d.Placement.Constraints,
			Preferences: d.Placement.Preferences,
			MaxReplicas: d.Placement.MaxReplicas,
		}
	}

	return dc
}

func parseCPUs(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		var c float64
		_, _ = fmt.Sscanf(val, "%f", &c)
		return c
	}
	return 0
}

func parseComposeSecrets(raw interface{}, topLevel map[string]composeSecretSpec) []SecretRef {
	if raw == nil {
		return nil
	}

	var refs []SecretRef

	switch val := raw.(type) {
	case []interface{}:
		for _, item := range val {
			switch s := item.(type) {
			case string:
				refs = append(refs, SecretRef{Source: s})
			case map[string]interface{}:
				ref := SecretRef{Source: getString(s, "source")}
				if ref.Source == "" {
					continue
				}
				ref.Target = getString(s, "target")
				ref.UID = getString(s, "uid")
				ref.GID = getString(s, "gid")
				if m, ok := s["mode"].(uint32); ok {
					ref.Mode = m
				} else if mf, ok := s["mode"].(float64); ok {
					ref.Mode = uint32(mf)
				}
				refs = append(refs, ref)
			}
		}
	case []string:
		for _, s := range val {
			refs = append(refs, SecretRef{Source: s})
		}
	}

	// Resolve file paths from top-level definitions
	for i, ref := range refs {
		if spec, ok := topLevel[ref.Source]; ok && spec.File != "" {
			if ref.Target == "" {
				refs[i].Target = "/run/secrets/" + ref.Source
			}
			if ref.Mode == 0 {
				refs[i].Mode = 0444
			}
		}
	}

	return refs
}

func parseComposeConfigs(raw interface{}, topLevel map[string]composeConfigSpec) []ConfigRef {
	if raw == nil {
		return nil
	}

	var refs []ConfigRef

	switch val := raw.(type) {
	case []interface{}:
		for _, item := range val {
			switch s := item.(type) {
			case string:
				refs = append(refs, ConfigRef{Source: s})
			case map[string]interface{}:
				ref := ConfigRef{Source: getString(s, "source")}
				if ref.Source == "" {
					continue
				}
				ref.Target = getString(s, "target")
				ref.UID = getString(s, "uid")
				ref.GID = getString(s, "gid")
				if m, ok := s["mode"].(uint32); ok {
					ref.Mode = m
				} else if mf, ok := s["mode"].(float64); ok {
					ref.Mode = uint32(mf)
				}
				refs = append(refs, ref)
			}
		}
	case []string:
		for _, s := range val {
			refs = append(refs, ConfigRef{Source: s})
		}
	}

	for i, ref := range refs {
		if spec, ok := topLevel[ref.Source]; ok && spec.File != "" {
			if ref.Target == "" {
				refs[i].Target = "/" + ref.Source
			}
			if ref.Mode == 0 {
				refs[i].Mode = 0444
			}
		}
	}

	return refs
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// LoadConfigOrCompose auto-detects the config file format and loads it.
func LoadConfigOrCompose(path string) (*Config, string, error) {
	if path != "" {
		cfg, p, err := tryLoad(path)
		return cfg, p, err
	}

	candidates := []string{
		"dck.toml",
		"compose.yaml",
		"compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates, home+"/.dck/dck.toml")
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			cfg, _, err := tryLoad(p)
			if err != nil {
				return nil, p, err
			}
			return cfg, p, nil
		}
	}

	return nil, "", fmt.Errorf("no config file found (looked for dck.toml, compose.yaml, compose.yml)")
}

func tryLoad(path string) (*Config, string, error) {
	switch {
	case strings.HasSuffix(path, ".toml"):
		cfg, err := loadFile(path)
		return cfg, path, err
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		cfg, err := LoadCompose(path)
		return cfg, path, err
	default:
		cfg, err := loadFile(path)
		if err == nil {
			return cfg, path, nil
		}
		cfg2, err2 := LoadCompose(path)
		if err2 == nil {
			return cfg2, path, nil
		}
		return nil, path, fmt.Errorf("cannot parse %s as TOML or YAML", path)
	}
}

func parseComposeEnv(env interface{}) map[string]string {
	result := make(map[string]string)
	if env == nil {
		return result
	}

	switch e := env.(type) {
	case []interface{}:
		for _, item := range e {
			if s, ok := item.(string); ok {
				if parts := strings.SplitN(s, "=", 2); len(parts) == 2 {
					result[parts[0]] = parts[1]
				}
			}
		}
	case map[string]interface{}:
		for k, v := range e {
			result[k] = fmt.Sprintf("%v", v)
		}
	case map[string]string:
		for k, v := range e {
			result[k] = v
		}
	}

	return result
}

func parseComposeEnvFile(ef interface{}) string {
	if ef == nil {
		return ""
	}
	switch e := ef.(type) {
	case string:
		return e
	case []interface{}:
		if len(e) > 0 {
			if s, ok := e[0].(string); ok {
				return s
			}
		}
	case []string:
		if len(e) > 0 {
			return e[0]
		}
	}
	return ""
}

func parseComposeCommand(cmd interface{}) string {
	if cmd == nil {
		return ""
	}
	switch c := cmd.(type) {
	case string:
		return c
	case []interface{}:
		parts := make([]string, len(c))
		for i, v := range c {
			parts[i] = fmt.Sprintf("%v", v)
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(c, " ")
	}
	return ""
}

func parseComposeHealthcheck(hc *composeHealthcheck) *HealthcheckConfig {
	cfg := &HealthcheckConfig{
		Interval: parseDurationToSeconds(hc.Interval),
		Timeout:  parseDurationToSeconds(hc.Timeout),
		Retries:  hc.Retries,
	}

	switch test := hc.Test.(type) {
	case string:
		cfg.Cmd = test
	case []interface{}:
		parts := make([]string, len(test))
		for i, v := range test {
			parts[i] = fmt.Sprintf("%v", v)
		}
		cfg.Cmd = strings.Join(parts, " ")
	case []string:
		cfg.Cmd = strings.Join(test, " ")
	}

	return cfg
}

func parseDurationToSeconds(d string) int {
	if d == "" {
		return 0
	}
	dur, err := time.ParseDuration(d)
	if err != nil {
		return 0
	}
	return int(dur.Seconds())
}

func normalizePort(p string) string {
	// Already in host:container format
	if strings.Contains(p, ":") {
		return p
	}
	// Container port only — expose on a random host port
	return "0:" + p
}

func normalizeRestart(r string) string {
	switch r {
	case "no", "always", "on-failure", "unless-stopped":
		return r
	case "":
		return "always"
	default:
		return r
	}
}

func parseComposeDependsOn(d interface{}) DependsOnConfig {
	if d == nil {
		return nil
	}
	result := make(DependsOnConfig)

	switch val := d.(type) {
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok {
				result[s] = "service_started"
			}
		}
	case []string:
		for _, s := range val {
			result[s] = "service_started"
		}
	case map[string]interface{}:
		for svc, cond := range val {
			switch c := cond.(type) {
			case string:
				result[svc] = c
			case map[string]interface{}:
				if condStr, ok := c["condition"].(string); ok {
					result[svc] = condStr
				} else {
					result[svc] = "service_started"
				}
			default:
				result[svc] = "service_started"
			}
		}
	case map[interface{}]interface{}:
		for svc, cond := range val {
			svcStr := fmt.Sprintf("%v", svc)
			switch c := cond.(type) {
			case string:
				result[svcStr] = c
			case map[interface{}]interface{}:
				if condStr, ok := c["condition"].(string); ok {
					result[svcStr] = condStr
				} else {
					result[svcStr] = "service_started"
				}
			default:
				result[svcStr] = "service_started"
			}
		}
	}

	return result
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	case []string:
		return val
	}
	return nil
}
