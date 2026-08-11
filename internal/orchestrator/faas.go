//go:build linux

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"dck/internal/container"
	"dck/internal/image"
	"dck/internal/log"
	"dck/internal/state"
)

var fnLock sync.RWMutex

// functionContainers tracks function name -> container IDs for active instances
var functionContainers = make(map[string][]string)

// DeployFunction deploys a serverless function
func DeployFunction(ctx context.Context, name, imageName string, port int, opts FnOpts) (*Function, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fnLock.Lock()
	defer fnLock.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	_ = loadFunctions()

	if _, exists := allFunctions[name]; exists {
		return nil, fmt.Errorf("function %q already exists", name)
	}

	fn := &Function{
		Name:        name,
		Image:       imageName,
		Handler:     opts.Handler,
		Port:        port,
		Env:         opts.Env,
		Timeout:     opts.Timeout,
		IdleTimeout: opts.IdleTimeout,
		Memory:      opts.Memory,
		CPUs:        opts.CPUs,
		Replicas:    opts.Replicas,
		Labels:      opts.Labels,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if fn.Timeout == 0 {
		fn.Timeout = 30
	}
	if fn.IdleTimeout == 0 {
		fn.IdleTimeout = 300
	}

	allFunctions[name] = fn
	if err := saveFunctions(); err != nil {
		delete(allFunctions, name)
		return nil, fmt.Errorf("save function: %w", err)
	}

	return fn, nil
}

// FnOpts contains optional function configuration
type FnOpts struct {
	Handler     string
	Env         map[string]string
	Timeout     int
	IdleTimeout int
	Memory      string
	CPUs        float64
	Replicas    int
	Labels      map[string]string
}

var allFunctions = make(map[string]*Function)

// ListFunctions returns all deployed functions
func ListFunctions() ([]*Function, error) {
	fnLock.Lock()
	defer fnLock.Unlock()

	if err := loadFunctions(); err != nil {
		return nil, err
	}

	fns := make([]*Function, 0, len(allFunctions))
	for _, f := range allFunctions {
		fns = append(fns, f)
	}

	sort.Slice(fns, func(i, j int) bool {
		return fns[i].CreatedAt.Before(fns[j].CreatedAt)
	})

	return fns, nil
}

// GetFunction returns a function by name
func GetFunction(name string) (*Function, error) {
	fnLock.Lock()
	defer fnLock.Unlock()

	if err := loadFunctions(); err != nil {
		return nil, err
	}

	fn, ok := allFunctions[name]
	if !ok {
		return nil, fmt.Errorf("function %q not found", name)
	}
	return fn, nil
}

// RemoveFunction removes a deployed function
func RemoveFunction(ctx context.Context, name string) error {
	fnLock.Lock()
	defer fnLock.Unlock()

	if err := loadFunctions(); err != nil {
		return err
	}

	fn, exists := allFunctions[name]
	if !exists {
		return fmt.Errorf("function %q not found", name)
	}

	scaleDownFunction(ctx, fn)

	delete(allFunctions, name)
	if err := saveFunctions(); err != nil {
		return fmt.Errorf("save functions: %w", err)
	}

	return nil
}

// InvokeFunction calls a deployed function (starts container if needed)
func InvokeFunction(ctx context.Context, name string, payload []byte) ([]byte, error) {
	fn, err := GetFunction(name)
	if err != nil {
		return nil, err
	}

	fnLock.Lock()
	fn.LastUsed = time.Now()
	fnLock.Unlock()

	if fn.ActiveContainers == 0 {
		replicas := fn.Replicas
		if replicas < 1 {
			replicas = 1
		}
		if err := scaleUpFunction(ctx, fn, replicas); err != nil {
			FaaSInvokeErrors.WithLabelValues(name, "scale_up").Inc()
			return nil, fmt.Errorf("scale up function %s: %w", name, err)
		}
	}

	invokeStart := time.Now()
	result, err := forwardToFunction(ctx, fn, payload)
	FaaSInvokeCount.WithLabelValues(name).Inc()
	FaaSInvokeDuration.WithLabelValues(name).Observe(time.Since(invokeStart).Seconds())
	if err != nil {
		FaaSInvokeErrors.WithLabelValues(name, "forward").Inc()
		return nil, err
	}

	fnLock.Lock()
	fn.InvokeCount++
	if err := saveFunctions(); err != nil {
		fnLock.Unlock()
		return nil, fmt.Errorf("save function invocation state: %w", err)
	}
	fnLock.Unlock()

	return result, nil
}

// StartFunctionGC starts a background goroutine for function auto-scaling
func StartFunctionGC(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gcFunctions(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func gcFunctions(ctx context.Context) {
	fnLock.RLock()
	fns := make([]*Function, 0, len(allFunctions))
	for _, f := range allFunctions {
		fns = append(fns, f)
	}
	fnLock.RUnlock()

	for _, fn := range fns {
		if ctx.Err() != nil {
			return
		}
		if fn.ActiveContainers > fn.Replicas {
			scaleDownFunction(ctx, fn)
		}
		if fn.IdleTimeout > 0 && fn.ActiveContainers > 0 && !fn.LastUsed.IsZero() {
			if time.Since(fn.LastUsed) > time.Duration(fn.IdleTimeout)*time.Second {
				log.Info("[faas] scaling down %s (idle for >%ds)", fn.Name, fn.IdleTimeout)
				scaleDownFunction(ctx, fn)
			}
		}
	}
}

func scaleUpFunction(ctx context.Context, fn *Function, count int) error {
	if err := ctx.Err(); err != nil {
		ScaleUpErrors.WithLabelValues(fn.Name, "function").Inc()
		return err
	}

	start := time.Now()
	log.Info("[faas] scaling up %s: +%d", fn.Name, count)

	img, err := image.Pull(fn.Image)
	if err != nil {
		ScaleUpErrors.WithLabelValues(fn.Name, "function").Inc()
		return fmt.Errorf("pull image %s: %w", fn.Image, err)
	}

	var memoryLimit int64
	if fn.Memory != "" {
		var mem int64
		if _, err := fmt.Sscanf(fn.Memory, "%d", &mem); err != nil {
			ScaleUpErrors.WithLabelValues(fn.Name, "function").Inc()
			return fmt.Errorf("parse memory limit %q: %w", fn.Memory, err)
		}
		memoryLimit = mem * 1024 * 1024
	}

	created := 0
	createdIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			rollbackFunctionContainers(createdIDs, fn)
			return ctx.Err()
		}
		replicaID := generateID()
		cName := fmt.Sprintf("fn_%s_%s", fn.Name, shortID(replicaID, 8))

		port := fn.Port
		if port == 0 {
			port = 8080
		}

		opts := container.CreateOpts{
			Name:   cName,
			Detach: true,
			Labels: map[string]string{
				"dck.function": fn.Name,
			},
			Ports: []container.PortMap{
				{HostPort: 0, ContainerPort: port, Protocol: "tcp"},
			},
		}

		for k, v := range fn.Env {
			opts.Env = append(opts.Env, k+"="+v)
		}
		if memoryLimit > 0 {
			opts.MemoryLimit = memoryLimit
		}
		if fn.CPUs > 0 {
			opts.CPUCount = fn.CPUs
		}
		if fn.Handler != "" {
			opts.Cmd = []string{fn.Handler}
		}

		c := container.New(img, opts)
		if err := c.Save(); err != nil {
			ScaleUpErrors.WithLabelValues(fn.Name, "function").Inc()
			rollbackFunctionContainers(createdIDs, fn)
			return fmt.Errorf("save container: %w", err)
		}
		createdIDs = append(createdIDs, c.ID)
		if err := c.Start(); err != nil {
			ScaleUpErrors.WithLabelValues(fn.Name, "function").Inc()
			rollbackFunctionContainers(createdIDs, fn)
			return fmt.Errorf("start container: %w", err)
		}

		fn.ActiveContainers++
		functionContainers[fn.Name] = append(functionContainers[fn.Name], c.ID)
		created++
	}

	log.Info("[faas] scaled up %s: %d containers running", fn.Name, created)
	RecordScaleUp(fn.Name, "function", time.Since(start).Seconds(), nil)
	return nil
}

func rollbackFunctionContainers(createdIDs []string, fn *Function) {
	if len(createdIDs) == 0 {
		return
	}

	created := make(map[string]struct{}, len(createdIDs))
	failed := make(map[string]struct{})
	for _, id := range createdIDs {
		created[id] = struct{}{}
		c, err := container.Load(id)
		if err != nil {
			failed[id] = struct{}{}
			log.Error("[faas] rollback could not load container %s: %v", shortID(id, 12), err)
			continue
		}
		if err := c.Remove(true); err != nil {
			failed[id] = struct{}{}
			log.Error("[faas] rollback failed for container %s: %v", shortID(id, 12), err)
		}
	}

	remaining := make([]string, 0, len(functionContainers[fn.Name])+len(failed))
	for _, id := range functionContainers[fn.Name] {
		if _, wasCreated := created[id]; !wasCreated || hasID(failed, id) {
			remaining = append(remaining, id)
		}
	}
	for id := range failed {
		if !hasIDInSlice(remaining, id) {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == 0 {
		delete(functionContainers, fn.Name)
	} else {
		functionContainers[fn.Name] = remaining
	}
	fn.ActiveContainers = len(remaining)
}

func hasID(ids map[string]struct{}, id string) bool {
	_, ok := ids[id]
	return ok
}

func hasIDInSlice(ids []string, id string) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func scaleDownFunction(ctx context.Context, fn *Function) {
	start := time.Now()
	containers := functionContainers[fn.Name]
	removed := 0
	for _, cid := range containers {
		if ctx.Err() != nil {
			log.Warn("[faas] context cancelled, %d/%d containers removed for %s", removed, len(containers), fn.Name)
			RecordScaleDown(fn.Name, "function", time.Since(start).Seconds(), ctx.Err())
			return
		}
		c, err := container.Load(cid)
		if err == nil {
			if err := c.Remove(true); err != nil {
				log.Error("[faas] error removing container %s: %v", shortID(cid, 12), err)
			} else {
				removed++
			}
		}
	}
	delete(functionContainers, fn.Name)
	fn.ActiveContainers = 0

	log.Info("[faas] scaled down %s (%d containers)", fn.Name, removed)
	RecordScaleDown(fn.Name, "function", time.Since(start).Seconds(), nil)
}

func forwardToFunction(ctx context.Context, fn *Function, payload []byte) ([]byte, error) {
	containers := functionContainers[fn.Name]
	if len(containers) == 0 {
		return nil, fmt.Errorf("no active containers for function %s", fn.Name)
	}

	c, err := container.Load(containers[0])
	if err != nil {
		return nil, fmt.Errorf("load container: %w", err)
	}

	port := fn.Port
	if port == 0 {
		port = 8080
	}

	targetURL := fmt.Sprintf("http://%s:%d", c.IP, port)
	if c.IP == "" {
		targetURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forward request to %s: %w", targetURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return result, nil
}

// --- internal I/O ---

func loadFunctions() error {
	dir := filepath.Join(state.DataDir(), FunctionStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "functions.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			allFunctions = make(map[string]*Function)
			return nil
		}
		return err
	}

	var fns map[string]*Function
	if err := json.Unmarshal(data, &fns); err != nil {
		return err
	}

	allFunctions = fns
	if allFunctions == nil {
		allFunctions = make(map[string]*Function)
	}

	return nil
}

func saveFunctions() error {
	dir := filepath.Join(state.DataDir(), FunctionStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(allFunctions, "", "  ")
	if err != nil {
		return err
	}

	return state.WriteFileAtomic(filepath.Join(dir, "functions.json"), data, 0600)
}

// CleanIdleFunctions stops functions that have been idle too long
func CleanIdleFunctions(ctx context.Context) {
	fnLock.RLock()
	fns := make([]*Function, 0, len(allFunctions))
	for _, f := range allFunctions {
		fns = append(fns, f)
	}
	fnLock.RUnlock()

	for _, fn := range fns {
		if ctx.Err() != nil {
			return
		}
		if fn.IdleTimeout > 0 && fn.ActiveContainers > 0 && !fn.LastUsed.IsZero() {
			if time.Since(fn.LastUsed) > time.Duration(fn.IdleTimeout)*time.Second {
				log.Info("[faas] auto-scaling down %s (idle)", fn.Name)
				scaleDownFunction(ctx, fn)
			}
		}
		if fn.ActiveContainers > fn.Replicas {
			scaleDownFunction(ctx, fn)
		}
	}
}
