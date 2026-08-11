//go:build linux

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dck/internal/builder"
	"dck/internal/container"
	"dck/internal/image"
	"dck/internal/log"
	"dck/internal/state"
)

// ScheduleReplica places a container on a node and starts it
func ScheduleReplica(ctx context.Context, serviceName string, svc *Service) error {
	nodes, err := ListNodes()
	if err != nil || len(nodes) == 0 {
		return fmt.Errorf("no available nodes")
	}

	var active []*Node
	for _, n := range nodes {
		if n.State == NodeStateActive && n.ID != clusterConf.NodeID {
			active = append(active, n)
		}
	}

	localNode, _ := GetNode()
	if localNode != nil && localNode.State == NodeStateActive {
		active = append(active, localNode)
	}

	if len(active) == 0 {
		return fmt.Errorf("no active nodes")
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].MemAvail > active[j].MemAvail
	})

	target := active[0]
	log.Info("[scheduler] placing replica of %s on %s (%s:%d)",
		serviceName, target.Name, target.Address, target.APIPort)

	if target.ID == clusterConf.NodeID {
		return startLocalReplica(ctx, serviceName, svc)
	}

	return startRemoteReplica(ctx, serviceName, svc, target)
}

func startLocalReplica(ctx context.Context, serviceName string, svc *Service) error {
	log.Info("[scheduler] starting local replica of %s", serviceName)

	img, err := image.Pull(svc.Image)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", svc.Image, err)
	}

	var ports []container.PortMap
	for _, p := range svc.Ports {
		hp := p.Port
		if hp == 0 {
			hp = p.TargetPort
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		ports = append(ports, container.PortMap{
			HostPort:      hp,
			ContainerPort: p.TargetPort,
			Protocol:      proto,
		})
	}

	var volumes []container.VolumeMount
	for _, v := range svc.Volumes {
		mount, parseErr := container.VolumeMountFromSpec(v)
		if parseErr != nil {
			return fmt.Errorf("parse volume %q: %w", v, parseErr)
		}
		volumes = append(volumes, mount)
	}

	env := make([]string, 0, len(svc.Env))
	for k, v := range svc.Env {
		env = append(env, k+"="+v)
	}

	labels := svc.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["dck.service"] = serviceName

	restart := svc.Restart
	if restart == "" {
		restart = "always"
	}

	replicaID := generateID()

	opts := container.CreateOpts{
		Name:    serviceName + "." + shortID(replicaID, 8),
		Ports:   ports,
		Volumes: volumes,
		Env:     env,
		Restart: restart,
		Detach:  true,
		Labels:  labels,
	}
	if svc.Command != "" {
		opts.Cmd = builder.SplitShellWords(svc.Command)
	}

	c := container.New(img, opts)
	if err := c.Save(); err != nil {
		return fmt.Errorf("save container: %w", err)
	}

	if err := c.Start(); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	saveReplica(serviceName, replicaID, c.ID, clusterConf.NodeID)

	log.Info("[scheduler] local replica %s running (container %s)", shortID(replicaID, 8), shortID(c.ID, 12))
	return nil
}

func startRemoteReplica(ctx context.Context, serviceName string, svc *Service, node *Node) error {
	replicaID := generateID()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"service_name": serviceName,
		"replica_id":   replicaID,
		"image":        svc.Image,
		"ports":        svc.Ports,
		"env":          svc.Env,
		"volumes":      svc.Volumes,
		"command":      svc.Command,
		"restart":      svc.Restart,
	})

	url := fmt.Sprintf("http://%s:%d/cluster/replicas", node.Address, node.APIPort)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth := clusterAuthHeader(); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("schedule on %s: %w", node.Name, err)
	}
	defer resp.Body.Close()

	var result struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response from %s: %w", node.Name, err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("schedule on %s: status %d", node.Name, resp.StatusCode)
	}

	saveReplica(serviceName, replicaID, result.ContainerID, node.ID)

	log.Info("[scheduler] remote replica %s on %s (container %s)",
		shortID(replicaID, 8), node.Name, shortID(result.ContainerID, 12))
	return nil
}

// RemoveRemoteReplica stops a container on a remote node
func RemoveRemoteReplica(ctx context.Context, nodeID, containerID string) error {
	clusterLock.RLock()
	node, ok := clusterConf.Nodes[nodeID]
	clusterLock.RUnlock()
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	if nodeID == clusterConf.NodeID {
		c, err := container.Load(containerID)
		if err != nil {
			return fmt.Errorf("load local container %s: %w", containerID, err)
		}
		if err := c.Remove(true); err != nil {
			return fmt.Errorf("remove local container %s: %w", containerID, err)
		}
		log.Info("[scheduler] stopped local container %s", shortID(containerID, 12))
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("http://%s:%d/cluster/replicas/%s", node.Address, node.APIPort, containerID),
		nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if auth := clusterAuthHeader(); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("remove on %s: %w", node.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("remove on %s: status %d", node.Name, resp.StatusCode)
	}

	log.Info("[scheduler] removed remote container %s on %s", shortID(containerID, 12), node.Name)
	return nil
}

// AutoHealServices checks service replicas and replaces failed ones
func AutoHealServices(ctx context.Context) {
	AutoHealTotal.Inc()
	clusterLock.RLock()
	services := make(map[string]*Service)
	for k, v := range clusterConf.Services {
		services[k] = v
	}
	clusterLock.RUnlock()

	for name, svc := range services {
		replicas, _ := GetServiceReplicas(name)
		running := 0

		for _, r := range replicas {
			if r.Status == "running" {
				running++
			}
		}

		if running < svc.Replicas {
			needed := svc.Replicas - running
			log.Info("[heal] service %s: running=%d desired=%d, scheduling %d new replicas",
				name, running, svc.Replicas, needed)
			AutoHealReplicasCreated.Add(float64(needed))

			for i := 0; i < needed; i++ {
				if ctx.Err() != nil {
					return
				}
				if err := ScheduleReplica(ctx, name, svc); err != nil {
					reason := "unknown"
					if strings.Contains(err.Error(), "no available nodes") || strings.Contains(err.Error(), "no active nodes") {
						reason = "no_nodes"
					} else if strings.Contains(err.Error(), "pull image") {
						reason = "image_pull"
					} else if strings.Contains(err.Error(), "container") {
						reason = "container_error"
					}
					ScheduleReplicaErrors.WithLabelValues(name, reason).Inc()
					log.Error("[heal] schedule error for %s: %v", name, err)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(500 * time.Millisecond):
				}
			}
		}
	}
}

// StartAutoHealer runs the auto-heal loop
func StartAutoHealer(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			AutoHealServices(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// RollingUpdateService performs a rolling update of a service
func RollingUpdateService(ctx context.Context, name, newImage string, opts ServiceOpts) error {
	svc, err := GetService(name)
	if err != nil {
		return err
	}

	parallelism := 1
	if svc.UpdateConfig != nil && svc.UpdateConfig.Parallelism > 0 {
		parallelism = svc.UpdateConfig.Parallelism
	}

	delaySec := 0
	if svc.UpdateConfig != nil && svc.UpdateConfig.Delay > 0 {
		delaySec = svc.UpdateConfig.Delay
	}

	order := "stop-first"
	if svc.UpdateConfig != nil && svc.UpdateConfig.Order != "" {
		order = svc.UpdateConfig.Order
	}

	replicas, _ := GetServiceReplicas(name)
	log.Info("[rolling] updating %s: %s -> %s (parallel=%d, order=%s)",
		name, svc.Image, newImage, parallelism, order)

	batch := 0

	for i := 0; i < len(replicas); i += parallelism {
		end := i + parallelism
		if end > len(replicas) {
			end = len(replicas)
		}
		batch++

		batchReps := replicas[i:end]
		log.Info("[rolling] batch %d: updating %d replicas", batch, len(batchReps))

		for _, r := range batchReps {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if order == "start-first" {
				log.Info("[rolling] starting new replica of %s (image: %s)", name, newImage)
				oldSvc := *svc
				oldSvc.Image = newImage
				if err := ScheduleReplica(ctx, name, &oldSvc); err != nil {
					return fmt.Errorf("start replacement for replica %s: %w", r.ID, err)
				}
				if err := RemoveRemoteReplica(ctx, r.NodeID, r.ContainerID); err != nil {
					return fmt.Errorf("remove old replica %s: %w", r.ID, err)
				}
				removeReplicaState(name, r.ID)
			} else {
				log.Info("[rolling] stopping replica %s", r.ID)
				if err := RemoveRemoteReplica(ctx, r.NodeID, r.ContainerID); err != nil {
					return fmt.Errorf("remove old replica %s: %w", r.ID, err)
				}
				oldSvc := *svc
				oldSvc.Image = newImage
				if err := ScheduleReplica(ctx, name, &oldSvc); err != nil {
					return fmt.Errorf("start replacement for replica %s: %w", r.ID, err)
				}
				removeReplicaState(name, r.ID)
			}
		}

		if delaySec > 0 && batch < (len(replicas)+parallelism-1)/parallelism {
			log.Info("[rolling] waiting %ds before next batch...", delaySec)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(delaySec) * time.Second):
			}
		}
	}

	svc.Image = newImage
	svc.UpdatedAt = time.Now()

	serviceLock.Lock()
	clusterConf.Services[name] = svc
	if err := saveServices(); err != nil {
		serviceLock.Unlock()
		return fmt.Errorf("save updated service: %w", err)
	}
	serviceLock.Unlock()

	log.Info("[rolling] update complete: %s now using %s", name, newImage)
	return nil
}

// --- replica persistence ---

func removeReplicaState(serviceName, replicaID string) {
	path := filepath.Join(state.DataDir(), ServiceStateDir, serviceName, replicaID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Warn("[scheduler] remove replica state %s: %v", replicaID, err)
	}
}

func saveReplica(serviceName, replicaID, containerID, nodeID string) {
	dir := filepath.Join(state.DataDir(), ServiceStateDir, serviceName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Error("[scheduler] create replica state directory: %v", err)
		return
	}
	if err := os.Chmod(dir, 0700); err != nil {
		log.Error("[scheduler] secure replica state directory: %v", err)
		return
	}

	r := ServiceReplica{
		ID:          replicaID,
		ServiceName: serviceName,
		NodeID:      nodeID,
		ContainerID: containerID,
		Status:      "running",
		CreatedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Error("[scheduler] marshal replica state: %v", err)
		return
	}
	if err := state.WriteFileAtomic(filepath.Join(dir, replicaID+".json"), data, 0600); err != nil {
		log.Error("[scheduler] save replica state: %v", err)
	}
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}
