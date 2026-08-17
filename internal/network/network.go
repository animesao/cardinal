package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"dck/internal/state"
)

func EnsureSysctl() {
	if err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: sysctl ip_forward: %v\n", err)
	}
	if err := exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: sysctl route_localnet: %v\n", err)
	}

	_ = os.MkdirAll("/etc/sysctl.d", 0755)
	confPath := "/etc/sysctl.d/99-dck.conf"
	var entries []string
	data, err := os.ReadFile(confPath)
	if err == nil {
		entries = strings.Split(string(data), "\n")
	}
	need := map[string]string{
		"net.ipv4.ip_forward":              "1",
		"net.ipv4.conf.all.route_localnet": "1",
	}
	write := false
	for k, v := range need {
		found := false
		for _, line := range entries {
			if strings.Contains(line, k+"="+v) {
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, k+"="+v)
			write = true
		}
	}
	if write {
		f, err := os.OpenFile(confPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			_, _ = f.WriteString("# dck: container networking sysctls\n")
			for _, e := range entries {
				if e != "" {
					_, _ = f.WriteString(e + "\n")
				}
			}
			_ = f.Close()
		}
	}
}

func EnsureUFW() {
	if _, err := exec.Command("ufw", "status").Output(); err != nil {
		return
	}
	if err := exec.Command("ufw", "route", "allow", "in", "on", BridgeName).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ufw allow in: %v\n", err)
	}
	if err := exec.Command("ufw", "route", "allow", "out", "on", BridgeName).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ufw allow out: %v\n", err)
	}
}

func EnsureNetBase() {
	EnsureSysctl()
	EnsureUFW()
	_ = EnsureBridge()
}

const (
	BridgeName = "dck0"
	BridgeCIDR = "10.0.2.0/24"
	BridgeIP   = "10.0.2.1"
)

type ipPool struct {
	Allocated map[string]bool `json:"allocated"`
	mu        sync.Mutex
}

var (
	globalPool *ipPool
	poolOnce   sync.Once
)

func loadPool() *ipPool {
	poolOnce.Do(func() {
		path := filepath.Join(state.DataDir(), "networks", "ips.json")
		p := &ipPool{Allocated: make(map[string]bool)}
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, p)
		}
		globalPool = p
	})
	return globalPool
}

func savePool(p *ipPool) {
	path := filepath.Join(state.DataDir(), "networks", "ips.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = state.WriteFileAtomic(path, data, 0600)
}

func AllocateIP() (string, error) {
	p := loadPool()
	p.mu.Lock()
	defer p.mu.Unlock()

	_, cidr, _ := net.ParseCIDR(BridgeCIDR)
	ones, bits := cidr.Mask.Size()
	totalHosts := (1 << uint(bits-ones))

	for i := 2; i < totalHosts-1; i++ {
		ip := make(net.IP, len(cidr.IP))
		copy(ip, cidr.IP)
		ip[len(ip)-1] = byte(i)
		ipStr := ip.String()
		if !p.Allocated[ipStr] {
			p.Allocated[ipStr] = true
			savePool(p)
			return ipStr, nil
		}
	}
	return "", fmt.Errorf("no available IP addresses in %s", BridgeCIDR)
}

func ReleaseIP(ip string) {
	p := loadPool()
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.Allocated, ip)
	savePool(p)
}

func flushBridgeNeigh(ip string) {
	flushBridgeNeighOnBridge(BridgeName, ip)
}

func removeOrphanVeths() {
	out, err := exec.Command("ip", "-o", "link", "show", "master", BridgeName).Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "ve") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ifName := strings.TrimSuffix(fields[1], ":")
		prefix := strings.TrimPrefix(ifName, "ve")
		// Check if any container JSON exists with this prefix
		entries, err := os.ReadDir(state.ContainersDir())
		if err != nil {
			// Can't check, skip
			continue
		}
		hasContainer := false
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			if strings.HasPrefix(name, prefix) {
				hasContainer = true
				break
			}
		}
		if !hasContainer {
			if err := exec.Command("ip", "link", "delete", ifName).Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: ip link delete %s: %v\n", ifName, err)
			}
		}
	}
}

func EnsureBridge() error {
	removeOrphanVeths()

	if err := exec.Command("ip", "link", "show", BridgeName).Run(); err != nil {
		if err := exec.Command("ip", "link", "add", BridgeName, "type", "bridge").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: ip link add bridge: %v\n", err)
		}
		if err := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/24", BridgeIP), "dev", BridgeName).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: ip addr add bridge: %v\n", err)
		}
	}
	if err := exec.Command("ip", "link", "set", BridgeName, "up").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ip link set bridge up: %v\n", err)
	}

	if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", BridgeCIDR, "!", "-o", BridgeName, "-j", "MASQUERADE").Run(); err != nil {
		if err := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
			"-s", BridgeCIDR, "!", "-o", BridgeName, "-j", "MASQUERADE").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables MASQUERADE: %v\n", err)
		}
	}

	if err := exec.Command("iptables", "-C", "FORWARD", "-i", BridgeName, "-j", "ACCEPT").Run(); err != nil {
		if err := exec.Command("iptables", "-A", "FORWARD", "-i", BridgeName, "-j", "ACCEPT").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables FORWARD -i: %v\n", err)
		}
	}
	if err := exec.Command("iptables", "-C", "FORWARD", "-o", BridgeName, "-j", "ACCEPT").Run(); err != nil {
		if err := exec.Command("iptables", "-A", "FORWARD", "-o", BridgeName, "-j", "ACCEPT").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables FORWARD -o: %v\n", err)
		}
	}
	return nil
}

func networkShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func SetupVeth(containerID string, pid int, containerIP string) error {
	return SetupVethOnBridge(containerID, pid, containerIP, BridgeName, BridgeIP, "24")
}

// SetupVethOnBridge attaches a container namespace to a selected bridge.
// The caller must ensure the bridge and its gateway already exist.
func SetupVethOnBridge(containerID string, pid int, containerIP, bridge, gateway, prefix string) error {
	short := networkShortID(containerID)
	hostIf := fmt.Sprintf("ve%s", short)
	contIf := fmt.Sprintf("vc%s", short)

	if err := exec.Command("ip", "link", "add", hostIf, "type", "veth", "peer", "name", contIf).Run(); err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			_ = exec.Command("ip", "link", "delete", hostIf).Run()
		}
	}()

	if err := exec.Command("ip", "link", "set", contIf, "netns", fmt.Sprintf("%d", pid)).Run(); err != nil {
		return fmt.Errorf("move veth to netns: %w", err)
	}
	if err := exec.Command("ip", "link", "set", hostIf, "master", bridge).Run(); err != nil {
		return fmt.Errorf("attach veth to bridge: %w", err)
	}
	if err := exec.Command("ip", "link", "set", hostIf, "up").Run(); err != nil {
		return fmt.Errorf("set host veth up: %w", err)
	}

	if err := runInNetns(pid, "ip", "link", "set", "lo", "up"); err != nil {
		return fmt.Errorf("enable loopback in netns: %w", err)
	}
	if err := runInNetns(pid, "ip", "link", "set", contIf, "name", "eth0"); err != nil {
		return fmt.Errorf("rename container interface: %w", err)
	}
	if err := runInNetns(pid, "ip", "addr", "add", fmt.Sprintf("%s/%s", containerIP, prefix), "dev", "eth0"); err != nil {
		return fmt.Errorf("configure container address: %w", err)
	}
	if err := runInNetns(pid, "ip", "link", "set", "eth0", "up"); err != nil {
		return fmt.Errorf("enable container interface: %w", err)
	}
	if err := runInNetns(pid, "ip", "route", "add", "default", "via", gateway); err != nil {
		return fmt.Errorf("configure container route: %w", err)
	}

	flushBridgeNeighOnBridge(bridge, containerIP)
	rollback = false
	return nil
}

func flushBridgeNeighOnBridge(bridge, ip string) {
	if err := exec.Command("ip", "neigh", "flush", "dev", bridge, "to", ip).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ip neigh flush: %v\n", err)
	}
}

func runInNetns(pid int, args ...string) error {
	base := []string{"-t", fmt.Sprintf("%d", pid), "-n", "--"}
	return exec.Command("nsenter", append(base, args...)...).Run()
}

type PortRule struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	ContainerIP   string `json:"container_ip"`
}

func removeExistingDNAT(chain string, hostPort int, protocol string) {
	out, err := exec.Command("iptables-save", "-t", "nat").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "-A "+chain) {
			continue
		}
		if !strings.Contains(line, fmt.Sprintf("--dport %d", hostPort)) {
			continue
		}
		if !strings.Contains(line, "-j DNAT") {
			continue
		}
		del := strings.Replace(line, "-A", "-D", 1)
		if err := exec.Command("iptables", append([]string{"-t", "nat"}, strings.Fields(del)...)...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables delete DNAT: %v\n", err)
		}
	}
}

func ufwAllowPort(hostPort int, protocol string) {
	if _, err := exec.Command("ufw", "status").Output(); err != nil {
		return
	}
	if err := exec.Command("ufw", "allow", fmt.Sprintf("%d/%s", hostPort, protocol)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ufw allow port %d/%s: %v\n", hostPort, protocol, err)
	}
}

func ufwDenyPort(hostPort int, protocol string) {
	if _, err := exec.Command("ufw", "status").Output(); err != nil {
		return
	}
	if err := exec.Command("ufw", "delete", "allow", fmt.Sprintf("%d/%s", hostPort, protocol)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ufw deny port %d/%s: %v\n", hostPort, protocol, err)
	}
}

func AddPortForwarding(containerIP string, hostPort, containerPort int, protocol string) error {
	removeExistingDNAT("PREROUTING", hostPort, protocol)
	removeExistingDNAT("OUTPUT", hostPort, protocol)

	dnat := []string{
		"-t", "nat", "-A", "PREROUTING",
		"-p", protocol, "--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}
	if err := exec.Command("iptables", dnat...).Run(); err != nil {
		// iptables DNAT failed — try socat fallback for port forwarding
		if fwdErr := startSocatForward(hostPort, containerIP, containerPort, protocol); fwdErr != nil {
			return fmt.Errorf("DNAT: %v; socat fallback: %v", err, fwdErr)
		}
		return nil
	}

	output := []string{
		"-t", "nat", "-A", "OUTPUT",
		"-p", protocol, "--dport", fmt.Sprintf("%d", hostPort),
		"-m", "addrtype", "--dst-type", "LOCAL",
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}
	if err := exec.Command("iptables", output...).Run(); err != nil {
		rollback := append([]string{"-t", "nat", "-D"}, dnat[3:]...)
		_ = exec.Command("iptables", rollback...).Run()
		return fmt.Errorf("OUTPUT DNAT: %w", err)
	}

	fwd := []string{
		"-A", "FORWARD",
		"-p", protocol, "-d", containerIP, "--dport", fmt.Sprintf("%d", containerPort),
		"-j", "ACCEPT",
	}
	if err := exec.Command("iptables", fwd...).Run(); err != nil {
		rollback := append([]string{"-t", "nat", "-D"}, dnat[3:]...)
		if err := exec.Command("iptables", rollback...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables rollback DNAT: %v\n", err)
		}
		rollback2 := append([]string{"-t", "nat", "-D"}, output[3:]...)
		if err := exec.Command("iptables", rollback2...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: iptables rollback OUTPUT: %v\n", err)
		}
		return fmt.Errorf("FORWARD: %w", err)
	}

	// When Docker is running, its DOCKER-FORWARD chain drops packets before
	// our FORWARD rule is evaluated. Insert into DOCKER-USER which Docker
	// processes first — this is the intended hook for custom firewall rules.
	ensureDockerUserRule(containerIP, containerPort, protocol)

	ufwAllowPort(hostPort, protocol)

	return nil
}

// ensureDockerUserRule inserts an ACCEPT rule into Docker's DOCKER-USER
// chain so that Docker's own FORWARD-DROP doesn't block DCK container traffic.
// DOCKER-USER is processed before any Docker chain, making it the correct
// place for rules that should bypass Docker's default filtering.
func ensureDockerUserRule(containerIP string, containerPort int, protocol string) {
	// First, check if the DOCKER-USER chain exists (Docker installed)
	check := exec.Command("iptables", "-L", "DOCKER-USER", "-n", "-v")
	if check.Run() != nil {
		// No Docker or no DOCKER-USER chain — nothing to do
		return
	}

	// Check if a rule for this container IP already exists
	checkStr := fmt.Sprintf("-d %s", containerIP)
	list := exec.Command("iptables", "-L", "DOCKER-USER", "-n", "--line-numbers")
	if out, err := list.Output(); err == nil {
		if strings.Contains(string(out), checkStr) {
			// Rule already exists
			return
		}
	}

	// Insert ACCEPT rule at position 1 (before Docker's DROP rules)
	fwd := []string{
		"-I", "DOCKER-USER", "1",
		"-p", protocol,
		"-d", containerIP,
		"--dport", fmt.Sprintf("%d", containerPort),
		"-j", "ACCEPT",
	}
	if err := exec.Command("iptables", fwd...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: DOCKER-USER insert: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "DOCKER-USER: ACCEPT %s %s:%d\n", protocol, containerIP, containerPort)
	}
}

// removeDockerUserRule removes a specific DOCKER-USER rule for a container IP.
func removeDockerUserRule(containerIP string, containerPort int, protocol string) {
	fwd := []string{
		"-D", "DOCKER-USER",
		"-p", protocol,
		"-d", containerIP,
		"--dport", fmt.Sprintf("%d", containerPort),
		"-j", "ACCEPT",
	}
	if err := exec.Command("iptables", fwd...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: DOCKER-USER delete: %v\n", err)
	}
}

// socatProxies tracks running socat port-forwarding processes.
var socatProxies = map[string]*exec.Cmd{}

func startSocatForward(hostPort int, containerIP string, containerPort int, protocol string) error {
	// Check if socat is available
	if _, err := exec.LookPath("socat"); err != nil {
		return fmt.Errorf("socat not found: install with: apt install socat")
	}
	key := fmt.Sprintf("%d:%s:%d:%s", hostPort, containerIP, containerPort, protocol)
	// Kill existing proxy for this key if any
	if existing, ok := socatProxies[key]; ok {
		_ = existing.Process.Kill()
		delete(socatProxies, key)
	}
	listenAddr := fmt.Sprintf("TCP-LISTEN:%d,fork,reuseaddr", hostPort)
	connectAddr := fmt.Sprintf("TCP:%s:%d", containerIP, containerPort)
	cmd := exec.Command("socat", listenAddr, connectAddr)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("socat start: %w", err)
	}
	socatProxies[key] = cmd
	// Detach — don't wait
	go cmd.Wait()
	fmt.Fprintf(os.Stderr, "socat: port %d -> %s:%d (%s)\n", hostPort, containerIP, containerPort, protocol)
	return nil
}

func RemovePortForwarding(containerIP string, hostPort, containerPort int, protocol string) {
	// Kill socat proxy if running for this port
	key := fmt.Sprintf("%d:%s:%d:%s", hostPort, containerIP, containerPort, protocol)
	if cmd, ok := socatProxies[key]; ok {
		_ = cmd.Process.Kill()
		delete(socatProxies, key)
	}
	if err := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", protocol, "--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: iptables delete PREROUTING DNAT: %v\n", err)
	}

	if err := exec.Command("iptables", "-t", "nat", "-D", "OUTPUT",
		"-p", protocol, "--dport", fmt.Sprintf("%d", hostPort),
		"-m", "addrtype", "--dst-type", "LOCAL",
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: iptables delete OUTPUT DNAT: %v\n", err)
	}

	if err := exec.Command("iptables", "-D", "FORWARD",
		"-p", protocol, "-d", containerIP, "--dport", fmt.Sprintf("%d", containerPort),
		"-j", "ACCEPT").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: iptables delete FORWARD: %v\n", err)
	}

	// Also remove the DOCKER-USER rule
	removeDockerUserRule(containerIP, containerPort, protocol)

	ufwDenyPort(hostPort, protocol)
}

func RemoveVeth(containerID string) {
	hostIf := fmt.Sprintf("ve%s", networkShortID(containerID))
	if out, err := exec.Command("ip", "link", "show", hostIf).CombinedOutput(); err != nil {
		if !isMissingNetworkInterface(out) {
			fmt.Fprintf(os.Stderr, "Warning: ip link show %s: %v\n", hostIf, err)
		}
		return
	}
	if err := exec.Command("ip", "link", "delete", hostIf).Run(); err != nil {
		// The interface can disappear between the existence check and delete
		// when the container network namespace exits concurrently.
		if out, showErr := exec.Command("ip", "link", "show", hostIf).CombinedOutput(); showErr != nil && isMissingNetworkInterface(out) {
			return
		}
		fmt.Fprintf(os.Stderr, "Warning: ip link delete %s: %v\n", hostIf, err)
	}
}

func isMissingNetworkInterface(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "cannot find device") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "no such device")
}

func CleanupContainerNetwork(containerID, containerIP string, ports []PortRule) {
	for _, p := range ports {
		RemovePortForwarding(containerIP, p.HostPort, p.ContainerPort, p.Protocol)
	}
	ReleaseIP(containerIP)
	RemoveVeth(containerID)
}
