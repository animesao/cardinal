package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"dck/internal/log"
	"dck/internal/state"
)

var (
	clusterLock sync.RWMutex
	clusterConf *ClusterConfig
)

func init() {
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// InitCluster initializes a new cluster on this node
func InitCluster(name string, bindAddr string, bindPort int) error {
	clusterLock.Lock()
	defer clusterLock.Unlock()

	if err := loadClusterConfig(); err == nil && clusterConf.ClusterID != "" {
		return fmt.Errorf("already part of cluster %s", clusterConf.ClusterID)
	}

	clusterConf.ClusterID = generateID()
	clusterConf.ClusterName = name
	clusterConf.NodeID = generateID()
	clusterConf.NodeName = hostname()
	clusterConf.BindAddr = bindAddr
	clusterConf.BindPort = bindPort
	clusterConf.CreatedAt = time.Now()

	node := &Node{
		ID:        clusterConf.NodeID,
		Name:      clusterConf.NodeName,
		Address:   bindAddr,
		APIPort:   bindPort,
		Role:      NodeRoleLeader,
		State:     NodeStateActive,
		CPUCores:  cpuCores(),
		MemTotal:  memTotal(),
		MemAvail:  memTotal(),
		LastSeen:  time.Now(),
		JoinedAt:  time.Now(),
	}

	clusterConf.Nodes[node.ID] = node

	if err := saveClusterConfig(); err != nil {
		return fmt.Errorf("save cluster config: %w", err)
	}

	log.Info("Initialized cluster %s (%s)", clusterConf.ClusterName, clusterConf.ClusterID)
	log.Info("  Node ID: %s", node.ID)
	log.Info("  Node name: %s", node.Name)
	log.Info("  Bind address: %s:%d", bindAddr, bindPort)

	return nil
}

// JoinCluster joins an existing cluster via a peer address
func JoinCluster(peerAddr string, bindAddr string, bindPort int) error {
	clusterLock.Lock()
	defer clusterLock.Unlock()

	if err := loadClusterConfig(); err == nil && clusterConf.ClusterID != "" {
		return fmt.Errorf("already part of cluster %s", clusterConf.ClusterID)
	}

	// Register ourselves with the peer
	tmpID := generateID()
	tmpName := hostname()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"node_id":   tmpID,
		"node_name": tmpName,
		"address":   bindAddr,
		"api_port":  bindPort,
	})

	resp, err := http.Post(
		fmt.Sprintf("http://%s/cluster/join", peerAddr),
		"application/json",
		strings.NewReader(string(reqBody)),
	)
	if err != nil {
		return fmt.Errorf("join request to %s: %w", peerAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join rejected by %s: status %d", peerAddr, resp.StatusCode)
	}

	var clusterInfo struct {
		ClusterID   string              `json:"cluster_id"`
		ClusterName string              `json:"cluster_name"`
		Nodes       map[string]*Node    `json:"nodes"`
		Services    map[string]*Service `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&clusterInfo); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	clusterConf.ClusterID = clusterInfo.ClusterID
	clusterConf.ClusterName = clusterInfo.ClusterName
	clusterConf.NodeID = tmpID
	clusterConf.NodeName = tmpName
	clusterConf.BindAddr = bindAddr
	clusterConf.BindPort = bindPort
	clusterConf.CreatedAt = time.Now()

	for id, n := range clusterInfo.Nodes {
		clusterConf.Nodes[id] = n
	}
	for name, s := range clusterInfo.Services {
		clusterConf.Services[name] = s
	}

	if err := saveClusterConfig(); err != nil {
		return fmt.Errorf("save cluster config: %w", err)
	}

	log.Info("Joined cluster %s (%s)", clusterConf.ClusterName, clusterConf.ClusterID)
	log.Info("  Node ID: %s", clusterConf.NodeID)
	log.Info("  Peers: %d", len(clusterConf.Nodes))

	return nil
}

// LeaveCluster removes this node from the cluster
func LeaveCluster() error {
	clusterLock.Lock()
	defer clusterLock.Unlock()

	if err := loadClusterConfig(); err != nil {
		return err
	}
	if clusterConf.ClusterID == "" {
		return fmt.Errorf("not part of a cluster")
	}

	// Notify peers
	for id, node := range clusterConf.Nodes {
		if id == clusterConf.NodeID {
			continue
		}
		http.Post(
			fmt.Sprintf("http://%s:%d/cluster/leave", node.Address, node.APIPort),
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"node_id":"%s"}`, clusterConf.NodeID)),
		)
	}

	// Reset local config
	backupPath := filepath.Join(state.DataDir(), ClusterStateDir, "cluster.json.bak")
	saveClusterConfigTo(backupPath)

	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	saveClusterConfig()

	log.Info("Left cluster %s", clusterConf.ClusterName)
	return nil
}

// ListNodes returns all cluster nodes
func ListNodes() ([]*Node, error) {
	clusterLock.RLock()
	defer clusterLock.RUnlock()

	if err := loadClusterConfig(); err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(clusterConf.Nodes))
	for _, n := range clusterConf.Nodes {
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].JoinedAt.Before(nodes[j].JoinedAt)
	})

	return nodes, nil
}

// GetNode returns the local node info
func GetNode() (*Node, error) {
	clusterLock.RLock()
	defer clusterLock.RUnlock()

	if err := loadClusterConfig(); err != nil {
		return nil, err
	}
	if clusterConf.NodeID == "" {
		return nil, fmt.Errorf("not part of a cluster")
	}
	return clusterConf.Nodes[clusterConf.NodeID], nil
}

// GetClusterInfo returns current cluster info
func GetClusterInfo() *ClusterConfig {
	clusterLock.RLock()
	defer clusterLock.RUnlock()
	loadClusterConfig()
	return clusterConf
}

// ClusterHandler is the HTTP handler for cluster API requests
func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/join") {
			handleJoin(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/leave") {
			handleLeave(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/heartbeat") {
			handleHeartbeat(w, r)
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusterConf)
	}
}

func handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID   string `json:"node_id"`
		NodeName string `json:"node_name"`
		Address  string `json:"address"`
		APIPort  int    `json:"api_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clusterLock.Lock()
	defer clusterLock.Unlock()

	node := &Node{
		ID:       req.NodeID,
		Name:     req.NodeName,
		Address:  req.Address,
		APIPort:  req.APIPort,
		Role:     NodeRoleWorker,
		State:    NodeStateActive,
		CPUCores: cpuCores(),
		MemTotal: memTotal(),
		MemAvail: memTotal(),
		LastSeen: time.Now(),
		JoinedAt: time.Now(),
	}
	clusterConf.Nodes[node.ID] = node
	saveClusterConfig()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"cluster_id":   clusterConf.ClusterID,
		"cluster_name": clusterConf.ClusterName,
		"node_id":      clusterConf.NodeID,
		"nodes":        clusterConf.Nodes,
		"services":     clusterConf.Services,
	})
}

func handleLeave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clusterLock.Lock()
	defer clusterLock.Unlock()

	if n, ok := clusterConf.Nodes[req.NodeID]; ok {
		n.State = NodeStateLeft
		saveClusterConfig()
	}

	w.WriteHeader(http.StatusOK)
}

func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID  string `json:"node_id"`
		MemAvail int64 `json:"mem_avail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clusterLock.Lock()
	if n, ok := clusterConf.Nodes[req.NodeID]; ok {
		n.LastSeen = time.Now()
		n.State = NodeStateActive
		if req.MemAvail > 0 {
			n.MemAvail = req.MemAvail
		}
	}
	clusterLock.Unlock()

	w.WriteHeader(http.StatusOK)

	// Propagate to other peers
	go propagateHeartbeat(req.NodeID)
}

func propagateHeartbeat(nodeID string) {
	clusterLock.RLock()
	nodes := make([]*Node, 0)
	for _, n := range clusterConf.Nodes {
		if n.ID != clusterConf.NodeID && n.State == NodeStateActive {
			nodes = append(nodes, n)
		}
	}
	clusterLock.RUnlock()

	b, _ := json.Marshal(map[string]string{"node_id": nodeID})
	for _, n := range nodes {
		http.Post(
			fmt.Sprintf("http://%s:%d/cluster/heartbeat", n.Address, n.APIPort),
			"application/json",
			strings.NewReader(string(b)),
		)
	}
}

// StartHeartbeat starts periodic heartbeat to cluster peers
func StartHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sendHeartbeat()
		case <-ctx.Done():
			return
		}
	}
}

func sendHeartbeat() {
	clusterLock.RLock()
	if clusterConf.ClusterID == "" || clusterConf.NodeID == "" {
		clusterLock.RUnlock()
		return
	}
	myID := clusterConf.NodeID
	peers := make([]*Node, 0)
	for _, n := range clusterConf.Nodes {
		if n.ID != myID && n.State == NodeStateActive {
			peers = append(peers, n)
		}
	}
	clusterLock.RUnlock()

	for _, peer := range peers {
		http.Post(
			fmt.Sprintf("http://%s:%d/cluster/heartbeat", peer.Address, peer.APIPort),
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"node_id":"%s","mem_avail":%d}`, myID, memAvail())),
		)
	}
}

// --- internal helpers ---

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func loadClusterConfig() error {
	path := filepath.Join(state.DataDir(), ClusterStateDir, "cluster.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not part of a cluster")
		}
		return err
	}
	var cfg ClusterConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.Nodes != nil {
		clusterConf.Nodes = cfg.Nodes
	}
	if cfg.Services != nil {
		clusterConf.Services = cfg.Services
	}
	clusterConf.ClusterID = cfg.ClusterID
	clusterConf.ClusterName = cfg.ClusterName
	clusterConf.NodeID = cfg.NodeID
	clusterConf.NodeName = cfg.NodeName
	clusterConf.BindAddr = cfg.BindAddr
	clusterConf.BindPort = cfg.BindPort
	clusterConf.CreatedAt = cfg.CreatedAt
	return nil
}

func saveClusterConfig() error {
	dir := filepath.Join(state.DataDir(), ClusterStateDir)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "cluster.json")
	return saveClusterConfigTo(path)
}

func saveClusterConfigTo(path string) error {
	data, err := json.MarshalIndent(clusterConf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func cpuCores() int {
	return runtime.NumCPU()
}

func memTotal() int64 {
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

func memAvail() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemAvailable:") {
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

// GetMyAddress returns the best address for peer communication
func GetMyAddress() string {
	clusterLock.RLock()
	defer clusterLock.RUnlock()
	if clusterConf.BindAddr != "" {
		return clusterConf.BindAddr
	}
	return resolveLocalAddr()
}

func resolveLocalAddr() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}
