package config

type HealthcheckConfig struct {
	Cmd      string `toml:"cmd"`
	Interval int    `toml:"interval,omitempty"`
	Retries  int    `toml:"retries,omitempty"`
	Timeout  int    `toml:"timeout,omitempty"`
}

type DeployConfig struct {
	Replicas      int                  `toml:"replicas,omitempty"`
	Mode          string               `toml:"mode,omitempty"`
	Resources     *ResourcesConfig     `toml:"resources,omitempty"`
	RestartPolicy *RestartPolicyConfig `toml:"restart_policy,omitempty"`
	UpdateConfig  *UpdateConfig        `toml:"update_config,omitempty"`
	Placement     *PlacementConfig     `toml:"placement,omitempty"`
}

type ResourcesConfig struct {
	Limits       *ResourceSpec `toml:"limits,omitempty"`
	Reservations *ResourceSpec `toml:"reservations,omitempty"`
}

type ResourceSpec struct {
	CPUs   float64 `toml:"cpus,omitempty"`
	Memory string  `toml:"memory,omitempty"`
}

type RestartPolicyConfig struct {
	Condition   string `toml:"condition,omitempty"`
	Delay       string `toml:"delay,omitempty"`
	MaxAttempts int    `toml:"max_attempts,omitempty"`
	Window      string `toml:"window,omitempty"`
}

type UpdateConfig struct {
	Parallelism     int     `toml:"parallelism,omitempty"`
	Delay           string  `toml:"delay,omitempty"`
	FailureAction   string  `toml:"failure_action,omitempty"`
	Monitor         string  `toml:"monitor,omitempty"`
	MaxFailureRatio float64 `toml:"max_failure_ratio,omitempty"`
	Order           string  `toml:"order,omitempty"`
}

type PlacementConfig struct {
	Constraints []string `toml:"constraints,omitempty"`
	Preferences []string `toml:"preferences,omitempty"`
	MaxReplicas int      `toml:"max_replicas,omitempty"`
}

type SecretRef struct {
	Source string `toml:"source"`
	Target string `toml:"target,omitempty"`
	UID    string `toml:"uid,omitempty"`
	GID    string `toml:"gid,omitempty"`
	Mode   uint32 `toml:"mode,omitempty"`
}

type ConfigRef struct {
	Source string `toml:"source"`
	Target string `toml:"target,omitempty"`
	UID    string `toml:"uid,omitempty"`
	GID    string `toml:"gid,omitempty"`
	Mode   uint32 `toml:"mode,omitempty"`
}

type SecretSpec struct {
	File     string `toml:"file"`
	External bool   `toml:"external,omitempty"`
	Name     string `toml:"name,omitempty"`
}

type ConfigSpec struct {
	File     string `toml:"file"`
	External bool   `toml:"external,omitempty"`
	Name     string `toml:"name,omitempty"`
}

type DependsOnConfig map[string]string // service_name -> condition ("" | "service_started" | "service_healthy" | "service_completed_successfully")

type ContainerConfig struct {
	Image        string             `toml:"image"`
	Command      string             `toml:"command,omitempty"`
	Ports        []string           `toml:"ports,omitempty"`
	Volumes      []string           `toml:"volumes,omitempty"`
	Env          map[string]string  `toml:"env,omitempty"`
	EnvFile      string             `toml:"env_file,omitempty"`
	Restart      string             `toml:"restart,omitempty"`
	RestartDelay string             `toml:"restart_delay,omitempty" yaml:"restart_delay,omitempty"`
	Hostname     string             `toml:"hostname,omitempty"`
	Memory       string             `toml:"memory,omitempty"`
	CPUs         float64            `toml:"cpus,omitempty"`
	WorkDir      string             `toml:"workdir,omitempty"`
	Healthcheck  *HealthcheckConfig `toml:"healthcheck,omitempty"`
	Entrypoint   string             `toml:"entrypoint,omitempty"`
	NetworkMode  string             `toml:"network_mode,omitempty"`
	Network      string             `toml:"network,omitempty"`
	Labels       map[string]string  `toml:"labels,omitempty"`
	CapAdd       []string           `toml:"cap_add,omitempty"`
	CapDrop      []string           `toml:"cap_drop,omitempty"`
	User         string             `toml:"user,omitempty"`
	Readonly     bool               `toml:"readonly,omitempty"`
	NoNewPrivs   bool               `toml:"no_new_privs,omitempty"`
	Sysctls      map[string]string  `toml:"sysctls,omitempty"`
	Ulimits      map[string]string  `toml:"ulimits,omitempty"`
	DNS          []string           `toml:"dns,omitempty"`
	Replicas     int                `toml:"replicas,omitempty"`
	Deploy       *DeployConfig      `toml:"deploy,omitempty"`
	Secrets      []SecretRef        `toml:"secrets,omitempty"`
	Configs      []ConfigRef        `toml:"configs,omitempty"`
	DependsOn    DependsOnConfig    `toml:"depends_on,omitempty"`
}

type Config struct {
	Secrets   map[string]SecretSpec      `toml:"secrets,omitempty"`
	Configs   map[string]ConfigSpec      `toml:"configs,omitempty"`
	Container map[string]ContainerConfig `toml:"container"`
}
