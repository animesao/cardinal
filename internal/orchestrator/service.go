//go:build linux

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
var loadedServicesDir string
var loadedServicesFile bool

// CreateService creates a new service definition
func CreateService(name, image string, replicas int, opts ServiceOpts) (*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if err := loadServices(); err != nil {
		return nil, fmt.Errorf("load services: %w", err)
	}

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
	if err := saveServices(); err != nil {
		delete(clusterConf.Services, name)
		return nil, fmt.Errorf("save service: %w", err)
	}

	return svc, nil
}

// ServiceOpts contains optional service configuration
type ServiceOpts struct {
	Ports        []ServicePort
	Env          map[string]string
	Volumes      []string
	Command      string
	Restart      string
	Memory       string
	CPUs         float64
	Labels       map[string]string
	UpdateConfig *UpdateConfig
	Healthcheck  *ServiceHealthcheck
}

// ListServices returns all services
func ListServices() ([]*Service, error) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

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
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if err := loadServices(); err != nil {
		return nil, err
	}

	svc, ok := clusterConf.Services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return svc, nil
}

// RemoveService removes a service and all its replicas.
func RemoveService(name string) error {
	serviceLock.Lock()
	if err := loadServices(); err != nil {
		serviceLock.Unlock()
		return err
	}
	if _, exists := clusterConf.Services[name]; !exists {
		serviceLock.Unlock()
		return fmt.Errorf("service %q not found", name)
	}
	serviceLock.Unlock()

	replicas, err := GetServiceReplicas(name)
	if err != nil {
		return fmt.Errorf("list replicas for %s: %w", name, err)
	}
	for _, replica := range replicas {
		if err := RemoveRemoteReplica(context.Background(), replica.NodeID, replica.ContainerID); err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("remove replica %s: %w", replica.ID, err)
		}
		if err := os.Remove(filepath.Join(state.DataDir(), ServiceStateDir, name, replica.ID+".json")); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove replica state %s: %w", replica.ID, err)
		}
	}

	serviceLock.Lock()
	defer serviceLock.Unlock()
	if err := loadServices(); err != nil {
		return err
	}
	delete(clusterConf.Services, name)
	if err := saveServices(); err != nil {
		return fmt.Errorf("save services: %w", err)
	}
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
	if err := saveServices(); err != nil {
		return nil, fmt.Errorf("save service: %w", err)
	}

	return svc, nil
}

// UpdateService applies a rolling update to a service.
func UpdateService(ctx context.Context, name, image string, opts ServiceOpts) (*Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	serviceLock.Lock()
	if err := loadServices(); err != nil {
		serviceLock.Unlock()
		return nil, err
	}
	svc, ok := clusterConf.Services[name]
	if !ok {
		serviceLock.Unlock()
		return nil, fmt.Errorf("service %q not found", name)
	}
	oldImage := svc.Image
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
	svc.UpdatedAt = time.Now()
	if err := saveServices(); err != nil {
		serviceLock.Unlock()
		return nil, fmt.Errorf("save service: %w", err)
	}
	serviceLock.Unlock()

	if image != "" && image != oldImage {
		if err := RollingUpdateService(ctx, name, image, ServiceOpts{}); err != nil {
			return nil, err
		}
	}

	updated, err := GetService(name)
	if err != nil {
		return nil, err
	}
	log.Info("Updated service %s: %s -> %s", name, oldImage, updated.Image)
	return updated, nil
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
			continue
		}
		path := filepath.Join(state.DataDir(), ServiceStateDir, name, replicas[i].ID+".json")
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Warn("[reconcile] remove replica state %s: %v", replicas[i].ID, err)
		}
	}
}

// --- internal I/O ---

func loadServices() error {
	dir := filepath.Join(state.DataDir(), ServiceStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "services.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if loadedServicesDir != dir || loadedServicesFile {
				clusterConf.Services = make(map[string]*Service)
			}
			if clusterConf.Services == nil {
				clusterConf.Services = make(map[string]*Service)
			}
			loadedServicesDir = dir
			loadedServicesFile = false
			return nil
		}
		return err
	}

	var svcs map[string]*Service
	if err := json.Unmarshal(data, &svcs); err != nil {
		return err
	}

	clusterConf.Services = svcs
	if clusterConf.Services == nil {
		clusterConf.Services = make(map[string]*Service)
	}
	loadedServicesDir = dir
	loadedServicesFile = true

	return nil
}

func saveServices() error {
	dir := filepath.Join(state.DataDir(), ServiceStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(clusterConf.Services, "", "  ")
	if err != nil {
		return err
	}

	if err := state.WriteFileAtomic(filepath.Join(dir, "services.json"), data, 0600); err != nil {
		return err
	}
	loadedServicesDir = dir
	loadedServicesFile = true
	return nil
}
