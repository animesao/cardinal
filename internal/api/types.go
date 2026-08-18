//go:build linux

package api

import (
	"time"
)

// Docker API compatible types (subset)

type Container struct {
	ID              string            `json:"Id"`
	Names           []string          `json:"Names"`
	Image           string            `json:"Image"`
	ImageID         string            `json:"ImageID"`
	Command         string            `json:"Command"`
	Created         int64             `json:"Created"`
	State           string            `json:"State"`
	Status          string            `json:"Status"`
	Ports           []Port            `json:"Ports"`
	Labels          map[string]string `json:"Labels"`
	Mounts          []Mount           `json:"Mounts"`
	NetworkSettings *NetworkSettings  `json:"NetworkSettings,omitempty"`
}

type Port struct {
	IP          string `json:"IP,omitempty"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}

type Mount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

type NetworkSettings struct {
	Networks  map[string]NetworkEntry `json:"Networks,omitempty"`
	IPAddress string                  `json:"IPAddress,omitempty"`
}

type NetworkEntry struct {
	IPAddress  string `json:"IPAddress"`
	Gateway    string `json:"Gateway,omitempty"`
	NetworkID  string `json:"NetworkID,omitempty"`
	EndpointID string `json:"EndpointID,omitempty"`
	MacAddress string `json:"MacAddress,omitempty"`
}

type ContainerInspect struct {
	ID              string           `json:"Id"`
	Name            string           `json:"Name"`
	Created         string           `json:"Created"`
	Path            string           `json:"Path"`
	Args            []string         `json:"Args"`
	State           *ContainerState  `json:"State"`
	Image           string           `json:"Image"`
	ResolvConfPath  string           `json:"ResolvConfPath,omitempty"`
	HostnamePath    string           `json:"HostnamePath,omitempty"`
	Config          *ContainerConfig `json:"Config"`
	HostConfig      *HostConfig      `json:"HostConfig"`
	NetworkSettings *InspectNetwork  `json:"NetworkSettings"`
	Mounts          []Mount          `json:"Mounts,omitempty"`
}

type ContainerState struct {
	Status     string `json:"Status"`
	Running    bool   `json:"Running"`
	Paused     bool   `json:"Paused"`
	Restarting bool   `json:"Restarting"`
	OOMKilled  bool   `json:"OOMKilled"`
	Dead       bool   `json:"Dead"`
	Pid        int    `json:"Pid"`
	ExitCode   int    `json:"ExitCode"`
	StartedAt  string `json:"StartedAt"`
	FinishedAt string `json:"FinishedAt"`
}

type ContainerConfig struct {
	Hostname     string              `json:"Hostname"`
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	Healthcheck  interface{}         `json:"Healthcheck,omitempty"`
	StartupScript string              `json:"StartupScript,omitempty"`
}

type HostConfig struct {
	Binds          []string                 `json:"Binds,omitempty"`
	PortBindings   map[string][]PortBinding `json:"PortBindings,omitempty"`
	RestartPolicy  *RestartPolicy           `json:"RestartPolicy,omitempty"`
	NetworkMode    string                   `json:"NetworkMode,omitempty"`
	Privileged     bool                     `json:"Privileged,omitempty"`
	ReadonlyRootfs bool                     `json:"ReadonlyRootfs,omitempty"`
	DNS            []string                 `json:"Dns,omitempty"`
	CapAdd         []string                 `json:"CapAdd,omitempty"`
	CapDrop        []string                 `json:"CapDrop,omitempty"`
	Memory         int64                    `json:"Memory,omitempty"`
	NanoCPUs       int64                    `json:"NanoCpus,omitempty"`
	DiskLimit      int64                    `json:"DiskLimit,omitempty"`
	Runtime        string                   `json:"Runtime,omitempty"`
}

type PortBinding struct {
	HostIP   string `json:"HostIp,omitempty"`
	HostPort string `json:"HostPort,omitempty"`
}

type RestartPolicy struct {
	Name              string `json:"Name"`
	MaximumRetryCount int    `json:"MaximumRetryCount,omitempty"`
}

type InspectNetwork struct {
	IPAddress string                         `json:"IPAddress"`
	Gateway   string                         `json:"Gateway,omitempty"`
	Networks  map[string]InspectNetworkEntry `json:"Networks,omitempty"`
}

type InspectNetworkEntry struct {
	IPAddress  string `json:"IPAddress"`
	Gateway    string `json:"Gateway,omitempty"`
	NetworkID  string `json:"NetworkID,omitempty"`
	EndpointID string `json:"EndpointID,omitempty"`
	MacAddress string `json:"MacAddress,omitempty"`
}

type ImageSummary struct {
	ID          string            `json:"Id"`
	ParentID    string            `json:"ParentId,omitempty"`
	RepoTags    []string          `json:"RepoTags"`
	RepoDigests []string          `json:"RepoDigests,omitempty"`
	Created     int64             `json:"Created"`
	Size        int64             `json:"Size"`
	Labels      map[string]string `json:"Labels,omitempty"`
}

type ImageInspect struct {
	ID           string           `json:"Id"`
	RepoTags     []string         `json:"RepoTags"`
	Created      string           `json:"Created"`
	Size         int64            `json:"Size"`
	Architecture string           `json:"Architecture"`
	OS           string           `json:"Os"`
	Config       *ContainerConfig `json:"Config,omitempty"`
	RootFS       *ImageRootFS     `json:"RootFS,omitempty"`
}

type ImageRootFS struct {
	Type   string   `json:"Type"`
	Layers []string `json:"Layers"`
}

type SystemInfo struct {
	ID                string      `json:"ID"`
	Containers        int         `json:"Containers"`
	ContainersRunning int         `json:"ContainersRunning"`
	ContainersPaused  int         `json:"ContainersPaused"`
	ContainersStopped int         `json:"ContainersStopped"`
	Images            int         `json:"Images"`
	Driver            string      `json:"Driver"`
	DriverStatus      [][2]string `json:"DriverStatus"`
	SystemStatus      []string    `json:"SystemStatus,omitempty"`
	Plugins           struct {
		Volume  []string `json:"Volume"`
		Network []string `json:"Network"`
	} `json:"Plugins"`
	MemoryLimit        bool                   `json:"MemoryLimit"`
	SwapLimit          bool                   `json:"SwapLimit"`
	CPUCfsPeriod       bool                   `json:"CpuCfsPeriod"`
	CPUCfsQuota        bool                   `json:"CpuCfsQuota"`
	CPUShares          bool                   `json:"CpuShares"`
	CPUSet             bool                   `json:"Cpuset"`
	KernelVersion      string                 `json:"KernelVersion"`
	OperatingSystem    string                 `json:"OperatingSystem"`
	OSType             string                 `json:"OSType"`
	Architecture       string                 `json:"Architecture"`
	NCPU               int                    `json:"NCPU"`
	MemTotal           int64                  `json:"MemTotal"`
	DockerRootDir      string                 `json:"DockerRootDir"`
	Name               string                 `json:"Name"`
	ServerVersion      string                 `json:"ServerVersion"`
	HTTPProxy          string                 `json:"HttpProxy,omitempty"`
	HTTPSProxy         string                 `json:"HttpsProxy,omitempty"`
	NoProxy            string                 `json:"NoProxy,omitempty"`
	ExperimentalBuild  bool                   `json:"ExperimentalBuild"`
	DefaultRuntime     string                 `json:"DefaultRuntime"`
	LiveRestoreEnabled bool                   `json:"LiveRestoreEnabled"`
	IndexServerAddress string                 `json:"IndexServerAddress,omitempty"`
	Runtimes           map[string]RuntimeInfo `json:"Runtimes,omitempty"`
	InitBinary         string                 `json:"InitBinary,omitempty"`
	SecurityOptions    []string               `json:"SecurityOptions,omitempty"`
	CgroupDriver       string                 `json:"CgroupDriver"`
	CgroupVersion      string                 `json:"CgroupVersion"`
	Warnings           []string               `json:"Warnings,omitempty"`
}

type RuntimeInfo struct {
	Path        string   `json:"path"`
	RuntimeArgs []string `json:"runtimeArgs,omitempty"`
}

// CreateContainerRequest is the JSON body for POST /containers/create
type CreateContainerRequest struct {
	Hostname     string              `json:"Hostname"`
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Tty          bool                `json:"Tty"`
	StdinOnce    bool                `json:"StdinOnce"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	HostConfig   *CreateHostConfig   `json:"HostConfig,omitempty"`
	Healthcheck  interface{}         `json:"Healthcheck,omitempty"`
	StartupScript string              `json:"StartupScript,omitempty"`
}

type CreateHostConfig struct {
	Binds          []string                 `json:"Binds,omitempty"`
	PortBindings   map[string][]PortBinding `json:"PortBindings,omitempty"`
	RestartPolicy  *RestartPolicy           `json:"RestartPolicy,omitempty"`
	NetworkMode    string                   `json:"NetworkMode,omitempty"`
	Privileged     bool                     `json:"Privileged,omitempty"`
	ReadonlyRootfs bool                     `json:"ReadonlyRootfs,omitempty"`
	DNS            []string                 `json:"Dns,omitempty"`
	CapAdd         []string                 `json:"CapAdd,omitempty"`
	CapDrop        []string                 `json:"CapDrop,omitempty"`
	Memory         int64                    `json:"Memory,omitempty"`
	NanoCPUs       int64                    `json:"NanoCpus,omitempty"`
	DiskLimit      int64                    `json:"DiskLimit,omitempty"`
	ExtraHosts     []string                 `json:"ExtraHosts,omitempty"`
}

type CreateContainerResponse struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type OKResponse struct {
	Message string `json:"message"`
}

// Version response
type VersionResponse struct {
	Version       string `json:"Version"`
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion"`
	GitCommit     string `json:"GitCommit"`
	GoVersion     string `json:"GoVersion"`
	Os            string `json:"Os"`
	Arch          string `json:"Arch"`
	KernelVersion string `json:"KernelVersion"`
	BuildTime     string `json:"BuildTime"`
}

// ContainerChange
type ContainerChange struct {
	Path string `json:"Path"`
	Kind int    `json:"Kind"`
}

// ImageHistory
type ImageHistoryEntry struct {
	ID        string   `json:"Id"`
	Created   int64    `json:"Created"`
	CreatedBy string   `json:"CreatedBy"`
	Size      int64    `json:"Size"`
	Comment   string   `json:"Comment,omitempty"`
	Tags      []string `json:"Tags,omitempty"`
}

func shortID(id string, n int) string {
	if n <= 0 || len(id) <= n {
		return id
	}
	return id[:n]
}

func newDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func newTimestamp(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func containerStatus(c *ContainerState) string {
	switch {
	case c.Running:
		return "running"
	case c.Paused:
		return "paused"
	case c.Restarting:
		return "restarting"
	case c.Dead:
		return "dead"
	default:
		return "exited"
	}
}
