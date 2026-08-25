//go:build linux

package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"cardinal/internal/state"
)

const (
	DockerAPIVersion = "1.44"
	maxRequestBody   = 8 << 20
)

var (
	authToken     string
	serverVersion = "dev"
)

// SetServerVersion sets the application version returned by the API.
func SetServerVersion(version string) {
	if version != "" {
		serverVersion = version
	}
}

// SetAuthToken sets the Bearer token required for API access.
// When empty, authentication is disabled.
func SetAuthToken(token string) {
	authToken = token
}

func StartServer(port int, host string) error {
	return StartServerWithTLS(port, host, "", "")
}

// StartServerWithTLS starts the API and optionally serves HTTPS when both
// certificate and key paths are provided. External binds still require a
// Bearer token even when TLS is enabled.
func StartServerWithTLS(port int, host, certFile, keyFile string) error {
	if (certFile == "") != (keyFile == "") {
		return fmt.Errorf("TLS requires both certificate and key files")
	}
	if isExternalHost(host) && authToken == "" {
		return fmt.Errorf("refusing to expose API on %s without an authentication token; use --token or CARDINAL_TOKEN", host)
	}

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
	mux.Handle("/metrics", metricsHandler())

	// Raw handler
	mux.HandleFunc("/", handleRoot)

	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	fmt.Printf("cardinal API server listening on %s\n", addr)
	scheme := "http"
	if certFile != "" {
		scheme = "https"
	}
	fmt.Printf("Docker API compatible (Portainer: Settings > Docker > %s://%s)\n", scheme, addr)
	fmt.Printf("  curl %s://%s/version\n", scheme, addr)
	fmt.Printf("  curl %s://%s/containers/json\n", scheme, addr)
	fmt.Printf("  curl %s://%s/images/json\n", scheme, addr)

	server := &http.Server{
		Handler:           corsMiddleware(rateLimiter(authMiddleware(jsonContentType(mux)))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if certFile != "" {
		return server.ServeTLS(listener, certFile, keyFile)
	}
	return server.Serve(listener)
}

func isExternalHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	if host == "localhost" {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	return true
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + authToken
		if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, "Forbidden: invalid or missing authentication token")
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not advertise credential-capable API operations to arbitrary web
		// origins. Native clients use Authorization directly.
		origin := r.Header.Get("Origin")
		if !isAllowedCORSOrigin(origin) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var allowedCORSOrigins []string

func init() {
	// Default: localhost and loopback only.
	allowedCORSOrigins = []string{"localhost", "127.0.0.1", "::1"}
	// CARDINAL_CORS_ORIGINS is a comma-separated list of additional hostnames or IPs
	// allowed as CORS origins (e.g. "myapp.example.com,10.0.0.5").
	if env := os.Getenv("CARDINAL_CORS_ORIGINS"); env != "" {
		for _, h := range strings.Split(env, ",") {
			h = strings.TrimSpace(h)
			if h != "" {
				allowedCORSOrigins = append(allowedCORSOrigins, h)
			}
		}
	}
}

func isAllowedCORSOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	for _, allowed := range allowedCORSOrigins {
		if host == allowed {
			return true
		}
	}
	return false
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a tiny dependency-free per-IP token bucket. It is keyed by
// the originating host and refreshed at `rate` tokens per second with a
// burst of `bucket`. Because the runtime is privileged, requests are gated
// more strictly than a normal HTTP service — a low cap protects the host
// from accidental DoS loops in user scripts. The limiter exempts the
// loopback interface because `cardinal exec`/`cardinal port` etc. all hit the API
// multiple times per command and a slow token bucket there would hurt UX.
type rateBucket struct {
	tokens float64
	last   time.Time
}

func rateLimiter(next http.Handler) http.Handler {
	const (
		rate      = 25.0 // tokens per second
		bucket    = 50   // initial burst
		maxClients = 4096 // hard cap on tracked IPs
	)
	var (
		mu      sync.Mutex
		clients = map[string]*rateBucket{}
	)
	prune := func(maxAge time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		pruneLocked(clients, maxAge)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			next.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		state, ok := clients[host]
		if !ok {
			// Evict stale entries before admitting a new client when at capacity.
			if len(clients) >= maxClients {
				pruneLocked(clients, 10*time.Minute)
			}
			state = &rateBucket{tokens: bucket, last: time.Now()}
			clients[host] = state
		}
		now := time.Now()
		state.tokens += now.Sub(state.last).Seconds() * rate
		if state.tokens > bucket {
			state.tokens = bucket
		}
		state.last = now
		if state.tokens < 1 {
			mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded; retry later")
			return
		}
		state.tokens--
		mu.Unlock()

		// Periodic prune on every request to prevent unbounded growth.
		if len(clients) > maxClients/2 {
			go prune(10 * time.Minute)
		}
		next.ServeHTTP(w, r)
	})
}

// pruneLocked removes stale entries while the mutex is already held.
func pruneLocked(clients map[string]*rateBucket, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	for k, v := range clients {
		if v.last.Before(cutoff) {
			delete(clients, k)
		}
	}
}

// metricsHandler restricts the Prometheus endpoint behind the API bearer
// token. By default metrics end up leaking internal container state
// (cgroup counters, image pull timestamps) that an attacker could use to
// plan DoS attacks, so we require the same authentication that the rest of
// the API needs. Loopback callers can opt out of the strict default by
// setting CARDINAL_METRICS_REQUIRES_AUTH=0.
func metricsHandler() http.Handler {
	strict := os.Getenv("CARDINAL_METRICS_REQUIRES_AUTH") != "0"
	if !strict {
		return promhttp.Handler()
	}
	return authMiddleware(promhttp.Handler())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
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
	if _, err := w.Write([]byte("OK")); err != nil {
		return
	}
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	v := VersionResponse{
		Version:       serverVersion,
		APIVersion:    DockerAPIVersion,
		MinAPIVersion: "1.24",
		GitCommit:     "cardinal",
		GoVersion:     runtime.Version(),
		Os:            "linux",
		Arch:          "amd64",
		BuildTime:     "",
	}
	writeJSON(w, 200, v)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, 200, map[string]string{
			"message":    "cardinal API server",
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
		MemoryLimit:     true,
		SwapLimit:       true,
		CPUCfsPeriod:    true,
		CPUCfsQuota:     true,
		CPUShares:       true,
		CPUSet:          true,
		KernelVersion:   kernelVer,
		OperatingSystem: readOSRelease(),
		OSType:          "linux",
		Architecture:    "x86_64",
		NCPU:            readCPUCount(),
		MemTotal:        readMemTotal(),
		DockerRootDir:   state.DataDir(),
		Name:            hostname, ServerVersion: serverVersion + "-cardinal",
		HTTPProxy:          os.Getenv("HTTP_PROXY"),
		HTTPSProxy:         os.Getenv("HTTPS_PROXY"),
		NoProxy:            os.Getenv("NO_PROXY"),
		ExperimentalBuild:  false,
		DefaultRuntime:     "runc",
		LiveRestoreEnabled: false,
		IndexServerAddress: "https://index.docker.io/v1/",
		InitBinary:         "",
		SecurityOptions:    []string{"name=seccomp,profile=default"},
		CgroupDriver:       cgroupDriver,
		CgroupVersion:      cgroupVer,
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
				if _, err := fmt.Sscanf(parts[1], "%d", &val); err != nil {
					continue
				}
				return val * 1024 // kB to bytes
			}
		}
	}
	return 0
}
