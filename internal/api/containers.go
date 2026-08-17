//go:build linux

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"dck/internal/container"
	"dck/internal/image"
	"dck/internal/state"
)

func handleContainersList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	all := r.URL.Query().Get("all") == "1"
	limitStr := r.URL.Query().Get("limit")

	ctrs, err := container.List(all)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("list containers: %v", err))
		return
	}

	// Apply limit
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 && limit < len(ctrs) {
			ctrs = ctrs[:limit]
		}
	}

	result := make([]Container, 0, len(ctrs))
	for _, c := range ctrs {
		result = append(result, containerToSummary(c))
	}

	writeJSON(w, 200, result)
}

func mountMode(v container.VolumeMount) string {
	var options []string
	if v.ReadOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	if v.Propagation != "" {
		options = append(options, v.Propagation)
	}
	if v.SELinuxRelabel != "" {
		options = append(options, v.SELinuxRelabel)
	}
	if v.NoCopy {
		options = append(options, "nocopy")
	}
	return strings.Join(options, ",")
}

func mountSpec(v container.VolumeMount) string {
	value := v.Source + ":" + v.Target
	var options []string
	if v.ReadOnly {
		options = append(options, "ro")
	} else if v.Propagation != "" || v.SELinuxRelabel != "" || v.NoCopy {
		options = append(options, "rw")
	}
	if v.Propagation != "" {
		options = append(options, v.Propagation)
	}
	if v.SELinuxRelabel != "" {
		options = append(options, v.SELinuxRelabel)
	}
	if v.NoCopy {
		options = append(options, "nocopy")
	}
	if len(options) > 0 {
		value += ":" + strings.Join(options, ",")
	}
	return value
}

func containerToSummary(c *container.Container) Container {
	imageName := fmt.Sprintf("%s:%s", c.ImageName, c.ImageTag)
	cmd := strings.Join(c.Cmd, " ")

	var ports []Port
	for _, p := range c.Ports {
		ports = append(ports, Port{
			PrivatePort: uint16(p.ContainerPort),
			PublicPort:  uint16(p.HostPort),
			Type:        p.Protocol,
		})
	}

	var mounts []Mount
	for _, v := range c.Volumes {
		mountType := string(v.Type)
		if mountType == "" {
			mountType = "volume"
		}
		mounts = append(mounts, Mount{
			Type:        mountType,
			Source:      v.Source,
			Destination: v.Target,
			Mode:        mountMode(v),
			RW:          !v.ReadOnly,
		})
	}

	status := "exited"
	humanStatus := "Stopped"
	if c.Status == container.Running {
		status = "running"
		humanStatus = "Up"
	}

	netSettings := &NetworkSettings{
		IPAddress: c.IP,
	}
	if c.IP != "" {
		netSettings.Networks = map[string]NetworkEntry{
			"bridge": {
				IPAddress: c.IP,
				NetworkID: "dck0",
			},
		}
	}

	return Container{
		ID:              c.ID,
		Names:           []string{"/" + c.Name},
		Image:           imageName,
		ImageID:         "sha256:" + c.ImageTag,
		Command:         cmd,
		Created:         newTimestamp(c.CreatedAt),
		State:           status,
		Status:          humanStatus,
		Ports:           ports,
		Labels:          c.Labels,
		Mounts:          mounts,
		NetworkSettings: netSettings,
	}
}

func containerToInspect(c *container.Container) *ContainerInspect {
	created := newDate(c.CreatedAt)

	stateObj := &ContainerState{
		Pid:       c.PID,
		Status:    "exited",
		Running:   false,
		StartedAt: newDate(c.CreatedAt),
	}
	if c.Status == container.Running {
		stateObj.Status = "running"
		stateObj.Running = true
	} else if c.Status == container.Stopped {
		stateObj.Status = "exited"
		stateObj.ExitCode = 0
		stateObj.FinishedAt = newDate(time.Now())
	}

	env := c.Env
	if env == nil {
		env = []string{}
	}

	imageName := fmt.Sprintf("%s:%s", c.ImageName, c.ImageTag)
	var exposedPorts map[string]struct{}
	var portBindings map[string][]PortBinding
	if len(c.Ports) > 0 {
		exposedPorts = make(map[string]struct{})
		portBindings = make(map[string][]PortBinding)
		for _, p := range c.Ports {
			key := fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
			exposedPorts[key] = struct{}{}
			portBindings[key] = []PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(p.HostPort),
			}}
		}
	}

	var restartPolicy *RestartPolicy
	switch c.Restart {
	case "always":
		restartPolicy = &RestartPolicy{Name: "always"}
	case "on-failure":
		restartPolicy = &RestartPolicy{Name: "on-failure"}
	case "unless-stopped":
		restartPolicy = &RestartPolicy{Name: "unless-stopped"}
	case "no", "":
		restartPolicy = &RestartPolicy{Name: "no"}
	}

	networkMode := c.NetworkMode
	if networkMode == "" {
		networkMode = "bridge"
	}

	config := &ContainerConfig{
		Hostname:     c.Hostname,
		User:         c.User,
		ExposedPorts: exposedPorts,
		Env:          env,
		Cmd:          c.Cmd,
		Image:        imageName,
		WorkingDir:   c.WorkingDir,
		Entrypoint:   nil,
		Labels:       c.Labels,
		Volumes:      make(map[string]struct{}),
	}
	for _, v := range c.Volumes {
		config.Volumes[v.Target] = struct{}{}
	}

	hostConfig := &HostConfig{
		Binds:          make([]string, len(c.Volumes)),
		PortBindings:   portBindings,
		RestartPolicy:  restartPolicy,
		NetworkMode:    networkMode,
		Privileged:     false,
		ReadonlyRootfs: c.ReadonlyRootfs,
		DNS:            c.DNS,
		CapAdd:         c.CapAdd,
		CapDrop:        c.CapDrop,
		Memory:         c.MemoryLimit,
		NanoCPUs:       int64(c.CPUCount * 1_000_000_000),
		DiskLimit:      c.DiskLimit,
	}
	for i, v := range c.Volumes {
		hostConfig.Binds[i] = mountSpec(v)
	}

	var mounts []Mount
	for _, v := range c.Volumes {
		mountType := string(v.Type)
		if mountType == "" {
			mountType = "volume"
		}
		mounts = append(mounts, Mount{
			Type:        mountType,
			Source:      v.Source,
			Destination: v.Target,
			Mode:        mountMode(v),
			RW:          !v.ReadOnly,
		})
	}

	netSettings := &InspectNetwork{
		IPAddress: c.IP,
	}
	if c.IP != "" {
		netSettings.Networks = map[string]InspectNetworkEntry{
			"bridge": {
				IPAddress: c.IP,
				NetworkID: "dck0",
			},
		}
	}

	return &ContainerInspect{
		ID:              c.ID,
		Name:            "/" + c.Name,
		Created:         created,
		State:           stateObj,
		Image:           imageName,
		Config:          config,
		HostConfig:      hostConfig,
		NetworkSettings: netSettings,
		Mounts:          mounts,
	}
}

func handleContainersCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	var req CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Image == "" {
		writeError(w, 400, "image name required")
		return
	}

	// Pull the image
	img, err := image.Pull(req.Image)
	if err != nil {
		writeError(w, 404, fmt.Sprintf("image %s not found: %v", req.Image, err))
		return
	}

	name := req.Hostname
	if name == "" {
		// Generate from image name
		parts := strings.Split(img.Name, "/")
		name = parts[len(parts)-1]
		if existing := container.FindByName(name); existing != nil {
			name = name + "_" + strconv.FormatInt(time.Now().UnixNano()%10000, 10)
		}
	} else if existing := container.FindByName(name); existing != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("container name already in use: %s", name))
		return
	}

	// Parse port bindings from HostConfig
	var ports []container.PortMap
	if req.HostConfig != nil {
		for hostKey, bindings := range req.HostConfig.PortBindings {
			containerPortStr := strings.Split(hostKey, "/")[0]
			proto := "tcp"
			if strings.Contains(hostKey, "/") {
				proto = strings.Split(hostKey, "/")[1]
			}
			containerPort, _ := strconv.Atoi(containerPortStr)

			for _, b := range bindings {
				hostPort := 0
				if b.HostPort != "" {
					hostPort, _ = strconv.Atoi(b.HostPort)
				}
				ports = append(ports, container.PortMap{
					HostPort:      hostPort,
					ContainerPort: containerPort,
					Protocol:      proto,
				})
			}
		}
	}

	// Fallback: parse ExposedPorts
	if len(ports) == 0 && len(req.ExposedPorts) > 0 {
		for key := range req.ExposedPorts {
			portStr := strings.Split(key, "/")[0]
			proto := "tcp"
			if strings.Contains(key, "/") {
				proto = strings.Split(key, "/")[1]
			}
			p, _ := strconv.Atoi(portStr)
			if p > 0 {
				ports = append(ports, container.PortMap{
					ContainerPort: p,
					Protocol:      proto,
				})
			}
		}
	}	// Parse volumes from HostConfig.Binds
	var volumes []container.VolumeMount
	if req.HostConfig != nil {
		for _, bind := range req.HostConfig.Binds {
			spec, parseErr := container.ParseVolumeSpec(bind)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid bind %q: %v", bind, parseErr))
				return
			}
			// Ensure the mount target is an absolute path.
			if spec.Target != "" && !strings.HasPrefix(spec.Target, "/") {
				spec.Target = "/" + spec.Target
			}
			volumes = append(volumes, container.VolumeMount{
				Type:           spec.Type,
				Source:         spec.Source,
				Target:         spec.Target,
				ReadOnly:       spec.ReadOnly,
				Propagation:    spec.Propagation,
				SELinuxRelabel: spec.SELinuxRelabel,
				NoCopy:         spec.NoCopy,
			})
		}
	}

	// Parse restart policy
	restart := ""
	if req.HostConfig != nil && req.HostConfig.RestartPolicy != nil {
		switch req.HostConfig.RestartPolicy.Name {
		case "always", "on-failure", "unless-stopped", "no":
			restart = req.HostConfig.RestartPolicy.Name
		default:
			restart = "no"
		}
	}

	// Env
	env := req.Env
	if env == nil {
		env = []string{}
	}

	// Labels
	labels := req.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	// DNS
	var dns []string
	if req.HostConfig != nil {
		dns = req.HostConfig.DNS
	}

	// Capabilities
	var capAdd, capDrop []string
	if req.HostConfig != nil {
		capAdd = req.HostConfig.CapAdd
		capDrop = req.HostConfig.CapDrop
	}

	// Resource limits
	var memLimit int64
	var cpuCount float64
	var diskLimit int64
	if req.HostConfig != nil {
		memLimit = req.HostConfig.Memory
		if req.HostConfig.NanoCPUs > 0 {
			cpuCount = float64(req.HostConfig.NanoCPUs) / 1_000_000_000
		}
		diskLimit = req.HostConfig.DiskLimit
	}

	// Network mode
	networkMode := "bridge"
	if req.HostConfig != nil && req.HostConfig.NetworkMode != "" {
		networkMode = req.HostConfig.NetworkMode
	}

	// Entrypoint
	var entrypoint string
	if len(req.Entrypoint) > 0 {
		entrypoint = strings.Join(req.Entrypoint, " ")
	}

	// Working dir
	workdir := req.WorkingDir

	opts := container.CreateOpts{
		Name:        name,
		Cmd:         req.Cmd,
		Ports:       ports,
		Volumes:     volumes,
		Env:         env,
		Hostname:    name,
		Restart:     restart,
		Detach:      true,
		Labels:      labels,
		CapAdd:      capAdd,
		CapDrop:     capDrop,
		MemoryLimit: memLimit,
		CPUCount:    cpuCount,
		DiskLimit:   diskLimit,
		NetworkMode: networkMode,
		Entrypoint:  entrypoint,
		WorkingDir:  workdir,
		DNS:         dns,
		User:        req.User,
	}

	c := container.New(img, opts)
	if err := c.Save(); err != nil {
		writeError(w, 500, fmt.Sprintf("save container: %v", err))
		return
	}

	if err := c.Start(); err != nil {
		// Clean up the saved container file so it does not appear as a
		// permanent ghost in the container list. Best-effort; ignore error.
		_ = c.Remove(true)
		writeError(w, 500, fmt.Sprintf("start container: %v", err))
		return
	}

	writeJSON(w, 201, CreateContainerResponse{ID: c.ID})
}

func handleContainersRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/containers/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		writeError(w, 400, "missing container ID")
		return
	}

	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if id == "json" && action == "" {
		handleContainersList(w, r)
		return
	}
	if id == "create" && action == "" {
		handleContainersCreate(w, r)
		return
	}

	c, err := container.Load(id)
	if err != nil {
		// Fallback: scan the containers directory and try prefix + name match
		allContainers, listErr := container.List(true)
		if listErr == nil {
			for _, candidate := range allContainers {
				if candidate.ID == id || strings.HasPrefix(candidate.ID, id) || candidate.Name == id {
					c = candidate
					err = nil
					break
				}
			}
		}
		if c == nil {
			writeError(w, 404, fmt.Sprintf("container %s not found", id))
			return
		}
	}

	switch action {
	case "json":
		handleContainerInspect(w, r, c)
	case "start":
		handleContainerStart(w, r, c)
	case "stop":
		handleContainerStop(w, r, c)
	case "restart":
		handleContainerRestart(w, r, c)
	case "kill":
		handleContainerKill(w, r, c)
	case "remove":
		handleContainerRemove(w, r, c)
	case "logs":
		handleContainerLogs(w, r, c)
	case "top":
		handleContainerTop(w, r, c)
	case "stats":
		handleContainerStats(w, r, c)
	case "rename":
		handleContainerRename(w, r, c)
	case "exec":
		handleContainerExec(w, r, c)
	case "wait":
		handleContainerWait(w, r, c)
	case "export":
		handleContainerExport(w, r, c)
	case "changes":
		handleContainerChanges(w, r, c)
	default:
		writeError(w, 404, fmt.Sprintf("unknown action: %s", action))
	}
}

func handleContainerInspect(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	writeJSON(w, 200, containerToInspect(c))
}

func handleContainerStart(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	if c.Status == container.Running {
		writeJSON(w, 304, nil)
		return
	}
	if err := c.Start(); err != nil {
		writeError(w, 500, fmt.Sprintf("start: %v", err))
		return
	}
	writeJSON(w, 204, nil)
}

func handleContainerStop(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	if c.Status != container.Running {
		writeJSON(w, 304, nil)
		return
	}
	if err := c.Stop(); err != nil {
		writeError(w, 500, fmt.Sprintf("stop: %v", err))
		return
	}
	writeJSON(w, 204, nil)
}

func handleContainerRestart(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	if c.Status == container.Running {
		if err := c.Stop(); err != nil {
			writeError(w, 500, fmt.Sprintf("restart stop: %v", err))
			return
		}
	}
	c.Status = container.Created
	if err := c.Start(); err != nil {
		writeError(w, 500, fmt.Sprintf("restart: %v", err))
		return
	}
	writeJSON(w, 204, nil)
}

func handleContainerKill(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	if err := c.Stop(); err != nil {
		writeError(w, 500, fmt.Sprintf("kill: %v", err))
		return
	}
	writeJSON(w, 204, nil)
}

func handleContainerRemove(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "DELETE" {
		writeError(w, 405, "method not allowed")
		return
	}
	force := r.URL.Query().Get("force") == "1"
	if err := c.Remove(force); err != nil {
		writeError(w, 500, fmt.Sprintf("remove: %v", err))
		return
	}
	writeJSON(w, 204, nil)
}

func handleContainerLogs(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	tailStr := r.URL.Query().Get("tail")
	follow := r.URL.Query().Get("follow") == "1"
	stdout := r.URL.Query().Get("stdout") != "0"
	stderr := r.URL.Query().Get("stderr") != "0"
	_ = follow

	logPath := state.LogPath(c.ID)
	data, err := os.ReadFile(logPath)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("read logs: %v", err))
		return
	}

	// Apply tail
	if tailStr != "" && tailStr != "all" {
		tail, err := strconv.Atoi(tailStr)
		if err == nil && tail > 0 {
			lines := strings.Split(string(data), "\n")
			if tail < len(lines) {
				lines = lines[len(lines)-tail:]
			}
			data = []byte(strings.Join(lines, "\n"))
		}
	}

	// Docker API log format: 8-byte header + data per frame
	// For simplicity, just return raw text
	if stdout && !stderr {
		w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}
	w.WriteHeader(200)
	if _, err := w.Write(data); err != nil {
		return
	}
}

func handleContainerTop(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	if c.Status != container.Running {
		writeError(w, 409, "container is not running")
		return
	}

	psArgs := r.URL.Query().Get("ps_args")
	if psArgs == "" {
		psArgs = "aux"
	}

	out, err := c.TopString(psArgs)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("top: %v", err))
		return
	}

	// Docker API top returns structured data
	type TopResponse struct {
		Titles  []string `json:"Titles"`
		Process []string `json:"Processes"`
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	resp := TopResponse{}
	if len(lines) > 0 {
		resp.Titles = strings.Fields(lines[0])
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line != "" {
				resp.Process = append(resp.Process, line)
			}
		}
	}

	writeJSON(w, 200, resp)
}

func handleContainerStats(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	stream := r.URL.Query().Get("stream") != "0"

	if stream {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for i := 0; i < 30; i++ {
			stats, err := container.ReadContainerStats(c)
			if err != nil {
				break
			}
			if err := json.NewEncoder(w).Encode(stats); err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(1 * time.Second)
		}
	} else {
		stats, err := container.ReadContainerStats(c)
		if err != nil {
			writeError(w, 500, fmt.Sprintf("stats: %v", err))
			return
		}
		writeJSON(w, 200, stats)
	}
}

func handleContainerRename(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	newName := r.URL.Query().Get("name")
	if newName == "" {
		writeError(w, 400, "name parameter required")
		return
	}
	oldName := c.Name
	c.Name = newName
	if err := c.Save(); err != nil {
		c.Name = oldName
		writeError(w, 500, fmt.Sprintf("rename: %v", err))
		return
	}
	writeJSON(w, 200, OKResponse{Message: "renamed"})
}

func handleContainerExec(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	var req struct {
		Cmd          []string `json:"Cmd"`
		AttachStdin  bool     `json:"AttachStdin"`
		AttachStdout bool     `json:"AttachStdout"`
		AttachStderr bool     `json:"AttachStderr"`
		Tty          bool     `json:"Tty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if len(req.Cmd) == 0 {
		writeError(w, 400, "Cmd required")
		return
	}

	if err := c.ExecOpts(req.Cmd, req.AttachStdin, req.Tty); err != nil {
		writeError(w, 500, fmt.Sprintf("exec: %v", err))
		return
	}
	writeJSON(w, 200, map[string]interface{}{"Id": shortID(c.ID, 12) + "_exec"})
}

func handleContainerWait(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}
	writeJSON(w, 200, map[string]int{"StatusCode": 0})
}

func handleContainerExport(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	writeError(w, 501, "not implemented")
}

func handleContainerChanges(w http.ResponseWriter, r *http.Request, c *container.Container) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	writeJSON(w, 200, []ContainerChange{})
}

// ListAllContainers returns all containers as ContainerInspect for system info
func ListAllContainers() ([]*ContainerInspect, error) {
	ctrs, err := container.List(true)
	if err != nil {
		return nil, err
	}
	result := make([]*ContainerInspect, len(ctrs))
	for i, c := range ctrs {
		result[i] = containerToInspect(c)
	}
	return result, nil
}
