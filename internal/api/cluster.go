//go:build linux

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"dck/internal/builder"
	"dck/internal/container"
	"dck/internal/image"
)

func handleClusterRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/cluster/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		writeError(w, 400, "missing cluster endpoint")
		return
	}

	endpoint := parts[0]
	switch endpoint {
	case "replicas":
		handleReplicas(w, r)
	case "health":
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeError(w, 404, fmt.Sprintf("unknown cluster endpoint: %s", endpoint))
	}
}

type CreateReplicaRequest struct {
	ServiceName string            `json:"service_name"`
	Image       string            `json:"image"`
	ReplicaID   string            `json:"replica_id"`
	Ports       []ReplicaPort     `json:"ports,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Command     string            `json:"command,omitempty"`
	Restart     string            `json:"restart,omitempty"`
	Memory      string            `json:"memory,omitempty"`
	CPUs        float64           `json:"cpus,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ReplicaPort struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`
}

func handleReplicas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleReplicaCreate(w, r)
	case http.MethodDelete:
		handleReplicaRemove(w, r)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func handleReplicaCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateReplicaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Image == "" {
		writeError(w, 400, "image required")
		return
	}

	img, err := image.Pull(req.Image)
	if err != nil {
		writeError(w, 404, fmt.Sprintf("pull image %s: %v", req.Image, err))
		return
	}

	var ports []container.PortMap
	for _, p := range req.Ports {
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
	for _, v := range req.Volumes {
		mount, parseErr := container.VolumeMountFromSpec(v)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid volume %q: %v", v, parseErr))
			return
		}
		volumes = append(volumes, mount)
	}

	env := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	labels := req.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	if req.ServiceName != "" {
		labels["dck.service"] = req.ServiceName
	}
	if req.ReplicaID != "" {
		labels["dck.replica"] = req.ReplicaID
	}

	restart := req.Restart
	if restart == "" {
		restart = "no"
	}

	name := req.ServiceName
	if name == "" {
		name = img.Name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
	}
	if req.ReplicaID != "" {
		name = name + "." + shortID(req.ReplicaID, 8)
	}

	opts := container.CreateOpts{
		Name:    name,
		Ports:   ports,
		Volumes: volumes,
		Env:     env,
		Restart: restart,
		Detach:  true,
		Labels:  labels,
	}
	if req.Command != "" {
		opts.Cmd = builder.SplitShellWords(req.Command)
	}

	c := container.New(img, opts)
	if err := c.Save(); err != nil {
		writeError(w, 500, fmt.Sprintf("save container: %v", err))
		return
	}

	if err := c.Start(); err != nil {
		writeError(w, 500, fmt.Sprintf("start container: %v", err))
		return
	}

	writeJSON(w, 201, map[string]interface{}{
		"container_id": c.ID,
		"name":         c.Name,
		"status":       "running",
	})
}

func handleReplicaRemove(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/cluster/replicas/")
	containerID := strings.TrimSuffix(path, "/")

	if containerID == "" {
		writeError(w, 400, "container ID required")
		return
	}

	c, err := container.Load(containerID)
	if err != nil {
		writeError(w, 404, fmt.Sprintf("container %s not found", containerID))
		return
	}

	if err := c.Remove(true); err != nil {
		writeError(w, 500, fmt.Sprintf("remove container: %v", err))
		return
	}

	writeJSON(w, 200, map[string]string{"status": "removed"})
}

func handleListContainersOnNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	ctrs, err := container.List(true)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("list containers: %v", err))
		return
	}

	type containerInfo struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Image  string            `json:"image"`
		Status string            `json:"status"`
		Labels map[string]string `json:"labels,omitempty"`
	}

	result := make([]containerInfo, 0, len(ctrs))
	for _, c := range ctrs {
		status := "exited"
		if c.Status == container.Running {
			status = "running"
		}
		result = append(result, containerInfo{
			ID:     c.ID,
			Name:   c.Name,
			Image:  c.ImageName + ":" + c.ImageTag,
			Status: status,
			Labels: c.Labels,
		})
	}

	writeJSON(w, 200, result)
}
