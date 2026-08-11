//go:build linux

package orchestrator

import "time"

func shortID(id string, n int) string {
	if n <= 0 || len(id) <= n {
		return id
	}
	return id[:n]
}

// NodeRole represents the role of a node in the cluster
type NodeRole string

const (
	NodeRoleLeader NodeRole = "leader"
	NodeRoleWorker NodeRole = "worker"
)

// NodeState represents the state of a cluster node
type NodeState string

const (
	NodeStateActive   NodeState = "active"
	NodeStateInactive NodeState = "inactive"
	NodeStateLeft     NodeState = "left"
)

// Node represents a cluster member
type Node struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	APIPort  int               `json:"api_port"`
	Role     NodeRole          `json:"role"`
	State    NodeState         `json:"state"`
	CPUCores int               `json:"cpu_cores"`
	MemTotal int64             `json:"mem_total"`
	MemAvail int64             `json:"mem_avail"`
	LastSeen time.Time         `json:"last_seen"`
	JoinedAt time.Time         `json:"joined_at"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// Service represents a managed service with replicas
type Service struct {
	Name         string              `json:"name"`
	Image        string              `json:"image"`
	Replicas     int                 `json:"replicas"`
	Ports        []ServicePort       `json:"ports,omitempty"`
	Env          map[string]string   `json:"env,omitempty"`
	Volumes      []string            `json:"volumes,omitempty"`
	Command      string              `json:"command,omitempty"`
	Restart      string              `json:"restart,omitempty"`
	Memory       string              `json:"memory,omitempty"`
	CPUs         float64             `json:"cpus,omitempty"`
	Labels       map[string]string   `json:"labels,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	UpdateConfig *UpdateConfig       `json:"update_config,omitempty"`
	Healthcheck  *ServiceHealthcheck `json:"healthcheck,omitempty"`
}

// ServicePort defines a port mapping for a service
type ServicePort struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"` // tcp, udp
}

// UpdateConfig defines rolling update parameters
type UpdateConfig struct {
	Parallelism   int    `json:"parallelism"`    // max replicas updated at once
	Delay         int    `json:"delay"`          // seconds between updates
	FailureAction string `json:"failure_action"` // pause, continue, rollback
	Order         string `json:"order"`          // start-first, stop-first
}

// ServiceHealthcheck defines a healthcheck for a service
type ServiceHealthcheck struct {
	Test     []string `json:"test"`
	Interval int      `json:"interval"`
	Timeout  int      `json:"timeout"`
	Retries  int      `json:"retries"`
}

// ServiceReplica represents a single container instance of a service
type ServiceReplica struct {
	ID          string    `json:"id"`
	ServiceName string    `json:"service_name"`
	NodeID      string    `json:"node_id"`
	ContainerID string    `json:"container_id"`
	Status      string    `json:"status"` // running, pending, failed
	CreatedAt   time.Time `json:"created_at"`
}

// ClusterConfig is the cluster configuration stored on each node
type ClusterConfig struct {
	ClusterID   string              `json:"cluster_id"`
	ClusterName string              `json:"cluster_name"`
	NodeID      string              `json:"node_id"`
	NodeName    string              `json:"node_name"`
	BindAddr    string              `json:"bind_addr"`
	BindPort    int                 `json:"bind_port"`
	Nodes       map[string]*Node    `json:"nodes"`
	Services    map[string]*Service `json:"services"`
	CreatedAt   time.Time           `json:"created_at"`
}

// Function represents a deployed serverless function
type Function struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Handler     string            `json:"handler"` // path to handler binary or script
	Port        int               `json:"port"`    // internal port the function listens on
	Env         map[string]string `json:"env,omitempty"`
	Timeout     int               `json:"timeout"`      // max execution time in seconds
	IdleTimeout int               `json:"idle_timeout"` // seconds before scale-to-zero
	Memory      string            `json:"memory,omitempty"`
	CPUs        float64           `json:"cpus,omitempty"`
	Replicas    int               `json:"replicas"` // warm replicas
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	InvokeCount int64             `json:"invoke_count"`
	// Runtime state
	ActiveContainers int       `json:"active_containers"`
	LastUsed         time.Time `json:"last_used"`
}

const (
	DefaultClusterPort = 7946
	DefaultAPIPort     = 2375
	ClusterStateDir    = "cluster"
	ServiceStateDir    = "services"
	FunctionStateDir   = "functions"
)
