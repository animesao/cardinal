package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dck/internal/log"
	"dck/internal/state"
)

var serviceLock sync.RWMutex

// CreateService creates a new service definition
func CreateService(name, image string, replicas int, opts ServiceOpts) (*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	_ = loadServices()

	if _, exists := clusterConf.Services[name]; exists {
		return nil, fmt.Errorf("service %q already exists", name)
	}

	svc := &Service{
		Name:      name,
		Image:     image,
		Replicas:  replicas,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if opts.Ports != nil {
		svc.Ports = opts.Ports
	}
	if opts.Env != nil {
		svc.Env = opts.Env
	}
	if opts.Volumes != nil {
		svc.Volumes = opts.Volumes
	}
	if opts.Command != "" {
		svc.Command = opts.Command
	}
	if opts.Restart != "" {
		svc.Restart = opts.Restart
	}
	if opts.Memory != "" {
		svc.Memory = opts.Memory
	}
	if opts.CPUs != 0 {
		svc.CPUs = opts.CPUs
	}
	if opts.Labels != nil {
		svc.Labels = opts.Labels
	}
	if opts.UpdateConfig != nil {
		svc.UpdateConfig = opts.UpdateConfig
	}
	if opts.Healthcheck != nil {
		svc.Healthcheck = opts.Healthcheck
	}

	clusterConf.Services[name] = svc
	saveServices()

	return svc, nil
}

// ServiceOpts contains optional service configuration
type ServiceOpts struct {
	Ports       []ServicePort
	Env         map[string]string
	Volumes     []string
	Command     string
	Restart     string
	Memory      string
	CPUs        float64
	Labels      map[string]string
	UpdateConfig *UpdateConfig
	Healthcheck *ServiceHealthcheck
}

// ListServices returns all services
func ListServices() ([]*Service, error) {
	serviceLock.RLock()
	defer serviceLock.RUnlock()

	if err := loadServices(); err != nil {
		return nil, err
	}

	svcs := make([]*Service, 0, len(clusterConf.Services))
	for _, s := range clusterConf.Services {
		svcs = append(svcs, s)
	}

	sort.Slice(svcs, func(i, j int) bool {
		return svcs[i].CreatedAt.Before(svcs[j].CreatedAt)
	})

	return svcs, nil
}

// GetService returns a service by name
func GetService(name string) (*Service, error) {
	serviceLock.RLock()
	defer serviceLock.RUnlock()

	if err := loadServices(); err != nil {
		return nil, err
	}

	svc, ok := clusterConf.Services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return svc, nil
}

// RemoveService removes a service and all its replicas
func RemoveService(name string) error {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if err := loadServices(); err != nil {
		return err
	}

	if _, exists := clusterConf.Services[name]; !exists {
		return fmt.Errorf("service %q not found", name)
	}

	delete(clusterConf.Services, name)
	saveServices()

	return nil
}

// ScaleService changes the replica count for a service
func ScaleService(name string, replicas int) (*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if err := loadServices(); err != nil {
		return nil, err
	}

	svc, ok := clusterConf.Services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}

	if replicas < 0 {
		return nil, fmt.Errorf("replicas must be >= 0")
	}

	svc.Replicas = replicas
	svc.UpdatedAt = time.Now()
	saveServices()

	return svc, nil
}

// UpdateService applies a rolling update to a service
func UpdateService(ctx context.Context, name, image string, opts ServiceOpts) (*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if err := loadServices(); err != nil {
		return nil, err
	}

	svc, ok := clusterConf.Services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}

	oldImage := svc.Image
	svc.Image = image
	svc.UpdatedAt = time.Now()

	if opts.Ports != nil {
		svc.Ports = opts.Ports
	}
	if opts.Env != nil {
		svc.Env = opts.Env
	}
	if opts.Volumes != nil {
		svc.Volumes = opts.Volumes
	}
	if opts.Command != "" {
		svc.Command = opts.Command
	}
	if opts.Restart != "" {
		svc.Restart = opts.Restart
	}
	if opts.UpdateConfig != nil {
		svc.UpdateConfig = opts.UpdateConfig
	}

	saveServices()

	log.Info("Updated service %s: %s -> %s", name, oldImage, image)
	return svc, nil
}

// GetServiceReplicas returns the current replicas for a service across the cluster
func GetServiceReplicas(name string) ([]ServiceReplica, error) {
	replicas := make([]ServiceReplica, 0)

	// In a full implementation, each node reports its containers
	// For now, read from local state
	replicaDir := filepath.Join(state.DataDir(), ServiceStateDir, name)
	entries, err := os.ReadDir(replicaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return replicas, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(replicaDir, e.Name()))
		if err != nil {
			continue
		}
		var r ServiceReplica
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		replicas = append(replicas, r)
	}

	return replicas, nil
}

// --- service reconciliation ---

// ReconcileServices ensures the desired state matches actual state
func ReconcileServices(ctx context.Context) {
	serviceLock.RLock()
	services := make(map[string]*Service)
	for k, v := range clusterConf.Services {
		services[k] = v
	}
	serviceLock.RUnlock()

	for name, svc := range services {
		replicas, _ := GetServiceReplicas(name)
		running := 0
		for _, r := range replicas {
			if r.Status == "running" {
				running++
			}
		}

		if running < svc.Replicas {
			reconcileScaleUp(ctx, name, svc, svc.Replicas-running)
		} else if running > svc.Replicas {
			reconcileScaleDown(ctx, name, replicas, running-svc.Replicas)
		}
	}
}

func reconcileScaleUp(ctx context.Context, name string, svc *Service, count int) {
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			return
		}
		if err := ScheduleReplica(ctx, name, svc); err != nil {
			log.Error("[reconcile] error scheduling replica of %s: %v", name, err)
		}
	}
}

func reconcileScaleDown(ctx context.Context, name string, replicas []ServiceReplica, count int) {
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].CreatedAt.After(replicas[j].CreatedAt)
	})

	for i := 0; i < count && i < len(replicas); i++ {
		if ctx.Err() != nil {
			return
		}
		log.Info("[reconcile] removing replica %s of %s", replicas[i].ID, name)
		if err := RemoveRemoteReplica(ctx, replicas[i].NodeID, replicas[i].ContainerID); err != nil {
			log.Error("[reconcile] error removing replica %s: %v", replicas[i].ID, err)
		}
	}
}

// --- internal I/O ---

func loadServices() error {
	dir := filepath.Join(state.DataDir(), ServiceStateDir)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "services.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var svcs map[string]*Service
	if err := json.Unmarshal(data, &svcs); err != nil {
		return err
	}

	if clusterConf.Services == nil {
		clusterConf.Services = make(map[string]*Service)
	}
	for k, v := range svcs {
		clusterConf.Services[k] = v
	}

	return nil
}

func saveServices() error {
	dir := filepath.Join(state.DataDir(), ServiceStateDir)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(clusterConf.Services, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "services.json"), data, 0644)
}
