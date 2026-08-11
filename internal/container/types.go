//go:build linux

package container

import (
	"context"
	"os"
	"sync"
	"time"

	"dck/internal/state"
)

// stoppedContainers is a shared map used to signal container stop events
// across goroutines without relying on disk I/O or stale in-memory state.
var stoppedContainers sync.Map

type HealthcheckConfig struct {
	Cmd      string `json:"cmd" toml:"cmd"`
	Interval int    `json:"interval,omitempty" toml:"interval,omitempty"`
	Retries  int    `json:"retries,omitempty" toml:"retries,omitempty"`
	Timeout  int    `json:"timeout,omitempty" toml:"timeout,omitempty"`
}

type Ulimit struct {
	Name string `json:"name"`
	Soft uint64 `json:"soft"`
	Hard uint64 `json:"hard"`
}

type Container struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	ImageName           string             `json:"image_name"`
	ImageTag            string             `json:"image_tag"`
	PID                 int                `json:"pid"`
	MountNamespace      uint64             `json:"mount_namespace,omitempty"`
	PIDNamespace        uint64             `json:"pid_namespace,omitempty"`
	NetworkNamespace    uint64             `json:"network_namespace,omitempty"`
	IPCNamespace        uint64             `json:"ipc_namespace,omitempty"`
	UTSNamespace        uint64             `json:"uts_namespace,omitempty"`
	Status              Status             `json:"status"`
	Cmd                 []string           `json:"cmd"`
	StartupScript       string             `json:"startup_script,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	Ports               []PortMap          `json:"ports,omitempty"`
	Volumes             []VolumeMount      `json:"volumes,omitempty"`
	Env                 []string           `json:"env,omitempty"`
	Hostname            string             `json:"hostname,omitempty"`
	Restart             string             `json:"restart,omitempty"`
	RestartDelay        string             `json:"restart_delay,omitempty"`
	RestartMaxAttempts  int                `json:"restart_max_attempts,omitempty"`
	RestartWindow       string             `json:"restart_window,omitempty"`
	RestartAttempts     int                `json:"restart_attempts,omitempty"`
	RestartWindowStart  time.Time          `json:"restart_window_start,omitempty"`
	LastStartedAt       time.Time          `json:"last_started_at,omitempty"`
	RestartBlocked      bool               `json:"restart_blocked,omitempty"`
	AutoBackup          bool               `json:"auto_backup,omitempty"`
	BackupInterval      string             `json:"backup_interval,omitempty"`
	BackupRetention     int                `json:"backup_retention,omitempty"`
	BackupDir           string             `json:"backup_dir,omitempty"`
	LastBackupAt        time.Time          `json:"last_backup_at,omitempty"`
	BackupNextAttemptAt time.Time          `json:"backup_next_attempt_at,omitempty"`
	IP                  string             `json:"ip,omitempty"`
	Detach              bool               `json:"detach,omitempty"`
	Interactive         bool               `json:"interactive,omitempty"`
	TTY                 bool               `json:"tty,omitempty"`
	RemoveOnExit        bool               `json:"remove_on_exit,omitempty"`
	StoppedByUser       bool               `json:"stopped_by_user,omitempty"`
	LastExitAt          time.Time          `json:"last_exit_at,omitempty"`
	MemoryLimit         int64              `json:"memory_limit,omitempty"`
	CPUCount            float64            `json:"cpu_count,omitempty"`
	DiskLimit           int64              `json:"disk_limit,omitempty"`
	CgroupPath          string             `json:"cgroup_path,omitempty"`
	WorkingDir          string             `json:"working_dir,omitempty"`
	Healthcheck         *HealthcheckConfig `json:"healthcheck,omitempty"`
	Labels              map[string]string  `json:"labels,omitempty"`
	CapAdd              []string           `json:"cap_add,omitempty"`
	CapDrop             []string           `json:"cap_drop,omitempty"`
	User                string             `json:"user,omitempty"`
	ReadonlyRootfs      bool               `json:"readonly_rootfs,omitempty"`
	NoNewPrivileges     bool               `json:"no_new_privileges,omitempty"`
	Sysctls             map[string]string  `json:"sysctls,omitempty"`
	DNS                 []string           `json:"dns,omitempty"`
	NetworkMode         string             `json:"network_mode,omitempty"`
	NetworkIP           string             `json:"network_ip,omitempty"`
	Entrypoint          string             `json:"entrypoint,omitempty"`
	Ulimits             []Ulimit           `json:"ulimits,omitempty"`

	ConsoleServePID int           `json:"console_serve_pid,omitempty"`
	PortForwardPIDs []int         `json:"port_forward_pids,omitempty"`
	Secrets         []SecretMount `json:"secrets,omitempty"`
	Configs         []SecretMount `json:"configs,omitempty"`

	// Runtime-only (not persisted)
	cancelHealth   context.CancelFunc `json:"-"`
	mu             sync.Mutex         `json:"-"`
	lifecycleMu    sync.Mutex         `json:"-"`
	dataMu         sync.RWMutex       `json:"-"`
	cleanupStarted bool               `json:"-"`
}

type SecretMount struct {
	Source string `json:"source"`
	Target string `json:"target"`
	UID    string `json:"uid,omitempty"`
	GID    string `json:"gid,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
	Data   string `json:"data,omitempty"`
}

type Status string

const (
	Created Status = "created"
	Running Status = "running"
	Stopped Status = "stopped"
)

type PortMap struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type VolumeMount struct {
	Type           VolumeType `json:"type,omitempty"`
	Source         string     `json:"source"`
	Target         string     `json:"target"`
	ReadOnly       bool       `json:"read_only,omitempty"`
	Propagation    string     `json:"propagation,omitempty"`
	SELinuxRelabel string     `json:"selinux_relabel,omitempty"`
	NoCopy         bool       `json:"no_copy,omitempty"`
}

type CreateOpts struct {
	Name               string
	Cmd                []string
	StartupScript      string
	Ports              []PortMap
	Volumes            []VolumeMount
	Env                []string
	Hostname           string
	Restart            string
	RestartDelay       string
	RestartMaxAttempts int
	RestartWindow      string
	AutoBackup         bool
	BackupInterval     string
	BackupRetention    int
	BackupDir          string
	Detach             bool
	Interactive        bool
	TTY                bool
	RemoveOnExit       bool
	MemoryLimit        int64
	CPUCount           float64
	DiskLimit          int64
	WorkingDir         string
	Healthcheck        *HealthcheckConfig
	Labels             map[string]string
	CapAdd             []string
	CapDrop            []string
	User               string
	ReadonlyRootfs     bool
	NoNewPrivileges    bool
	Sysctls            map[string]string
	DNS                []string
	NetworkMode        string
	NetworkIP          string
	Entrypoint         string
	Ulimits            []Ulimit
}

func shortID(id string, n int) string {
	if n <= 0 || len(id) <= n {
		return id
	}
	return id[:n]
}

func (c *Container) Save() error {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	if err := os.MkdirAll(state.ContainersDir(), 0700); err != nil {
		return err
	}
	if err := os.Chmod(state.ContainersDir(), 0700); err != nil {
		return err
	}
	return state.WriteJSON(state.ContainerPath(c.ID), c)
}

func (c *Container) DeleteState() error {
	return os.Remove(state.ContainerPath(c.ID))
}

func (c *Container) LogFile() string {
	return state.LogPath(c.ID)
}

func (c *Container) OverlayDirs() (upper, work, merged string) {
	return state.OverlayDirs(c.ID)
}
