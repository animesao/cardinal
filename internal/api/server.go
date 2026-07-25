//go:build linux

package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"dck/internal/state"
)

const DockerAPIVersion = "1.44"

var authToken string

// SetAuthToken sets the Bearer token required for API access.
// When empty, authentication is disabled.
func SetAuthToken(token string) {
	authToken = token
}

func StartServer(port int, host string) error {
	mux := http.NewServeMux()

	// Docker API compatibility layer
	mux.HandleFunc("/_ping", handlePing)
	mux.HandleFunc("/version", handleVersion)
	mux.HandleFunc("/info", handleInfo)

	// Container endpoints
	mux.HandleFunc("/containers/json", handleContainersList)
	mux.HandleFunc("/containers/create", handleContainersCreate)
	mux.HandleFunc("/containers/", handleContainersRouter)

	// Image endpoints
	mux.HandleFunc("/images/json", handleImagesList)
	mux.HandleFunc("/images/", handleImagesRouter)

	// System endpoints
	mux.HandleFunc("/system/prune", handleSystemPrune)

	// Cluster endpoints (for cross-node orchestration)
	mux.HandleFunc("/cluster/", handleClusterRouter)
	mux.HandleFunc("/cluster/containers", handleListContainersOnNode)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Raw handler
	mux.HandleFunc("/", handleRoot)

	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	fmt.Printf("dck API server listening on %s\n", addr)
	fmt.Printf("Docker API compatible (Portainer: Settings > Docker > http://%s)\n", addr)
	fmt.Printf("  curl http://%s/version\n", addr)
	fmt.Printf("  curl http://%s/containers/json\n", addr)
	fmt.Printf("  curl http://%s/images/json\n", addr)

	return http.Serve(listener, authMiddleware(corsMiddleware(jsonContentType(mux))))
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			auth = r.URL.Query().Get("token")
		}
		expected := "Bearer " + authToken
		if auth == expected || auth == authToken {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, "Forbidden: invalid or missing authentication token")
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Message: msg})
}

func writeOK(w http.ResponseWriter, msg string) {
	writeJSON(w, 200, OKResponse{Message: msg})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	v := VersionResponse{
		Version:    "24.0.0",
		APIVersion: DockerAPIVersion,
		MinAPIVersion: "1.24",
		GitCommit:  "dck",
		GoVersion:  "go1.18",
		Os:         "linux",
		Arch:       "amd64",
		BuildTime:  "",
	}
	writeJSON(w, 200, v)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, 200, map[string]string{
			"message":    "dck API server",
			"version":    DockerAPIVersion,
			"api":        "Docker API compatible",
			"repository": state.DataDir(),
		})
		return
	}
	writeError(w, 404, fmt.Sprintf("route %s not found", r.URL.Path))
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	kernelVer := readKernelVersion()

	var running, stopped, paused int
	containers, _ := ListAllContainers()
	for _, c := range containers {
		switch c.State.Status {
		case "running":
			running++
		case "paused":
			paused++
		default:
			stopped++
		}
	}

	images, _ := ListAllImages()

	cgroupVer := "1"
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		cgroupVer = "2"
	}

	cgroupDriver := "cgroupfs"
	if _, err := os.Stat("/sys/fs/cgroup/systemd"); err == nil {
		cgroupDriver = "systemd"
	}

	info := SystemInfo{
		Containers:        len(containers),
		ContainersRunning: running,
		ContainersPaused:  paused,
		ContainersStopped: stopped,
		Images:            len(images),
		Driver:            "overlay2",
		DriverStatus: [][2]string{
			{"Backing Filesystem", "extfs"},
			{"Supports d_type", "true"},
		},
		MemoryLimit:   true,
		SwapLimit:     true,
		CPUCfsPeriod:  true,
		CPUCfsQuota:   true,
		CPUShares:     true,
		CPUSet:        true,
		KernelVersion: kernelVer,
		OperatingSystem: readOSRelease(),
		OSType:          "linux",
		Architecture:    "x86_64",
		NCPU:            readCPUCount(),
		MemTotal:        readMemTotal(),
		DockerRootDir:   state.DataDir(),
		Name:            hostname,
		ServerVersion:   "24.0.0-dck",
		HTTPProxy:       os.Getenv("HTTP_PROXY"),
		HTTPSProxy:      os.Getenv("HTTPS_PROXY"),
		NoProxy:         os.Getenv("NO_PROXY"),
		ExperimentalBuild: false,
		DefaultRuntime:  "runc",
		LiveRestoreEnabled: false,
		IndexServerAddress: "https://index.docker.io/v1/",
		InitBinary:      "",
		SecurityOptions: []string{"name=seccomp,profile=default"},
		CgroupDriver:    cgroupDriver,
		CgroupVersion:   cgroupVer,
		Runtimes: map[string]RuntimeInfo{
			"runc": {Path: "runc"},
		},
	}
	info.Plugins.Volume = []string{"local"}
	info.Plugins.Network = []string{"bridge", "host", "none"}

	writeJSON(w, 200, info)
}

func readKernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		return parts[2]
	}
	return string(data)
}

func readOSRelease() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "Linux"
}

func readCPUCount() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func readMemTotal() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				val := int64(0)
				fmt.Sscanf(parts[1], "%d", &val)
				return val * 1024 // kB to bytes
			}
		}
	}
	return 0
}
