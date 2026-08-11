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
	if err := exec.Command("ip", "neigh", "flush", "dev", BridgeName, "to", ip).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ip neigh flush: %v\n", err)
	}
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
	short := networkShortID(containerID)
	hostIf := fmt.Sprintf("ve%s", short)
	contIf := fmt.Sprintf("vc%s", short)

	if err := exec.Command("ip", "link", "add", hostIf, "type", "veth", "peer", "name", contIf).Run(); err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}

	if err := exec.Command("ip", "link", "set", contIf, "netns", fmt.Sprintf("%d", pid)).Run(); err != nil {
		return fmt.Errorf("move veth to netns: %w", err)
	}
	if err := exec.Command("ip", "link", "set", hostIf, "master", BridgeName).Run(); err != nil {
		return fmt.Errorf("attach veth to bridge: %w", err)
	}
	if err := exec.Command("ip", "link", "set", hostIf, "up").Run(); err != nil {
		return fmt.Errorf("set host veth up: %w", err)
	}

	_ = runInNetns(pid, "ip", "link", "set", "lo", "up")
	_ = runInNetns(pid, "ip", "link", "set", contIf, "name", "eth0")
	runInNetns(pid, "ip", "addr", "add", fmt.Sprintf("%s/24", containerIP), "dev", "eth0")
	runInNetns(pid, "ip", "link", "set", "eth0", "up")
	runInNetns(pid, "ip", "route", "add", "default", "via", BridgeIP)

	flushBridgeNeigh(containerIP)

	return nil
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
		return fmt.Errorf("DNAT: %w", err)
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

	ufwAllowPort(hostPort, protocol)

	return nil
}

func RemovePortForwarding(containerIP string, hostPort, containerPort int, protocol string) {
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

	ufwDenyPort(hostPort, protocol)
}

func RemoveVeth(containerID string) {
	hostIf := fmt.Sprintf("ve%s", networkShortID(containerID))
	if err := exec.Command("ip", "link", "delete", hostIf).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ip link delete %s: %v\n", hostIf, err)
	}
}

func CleanupContainerNetwork(containerID, containerIP string, ports []PortRule) {
	for _, p := range ports {
		RemovePortForwarding(containerIP, p.HostPort, p.ContainerPort, p.Protocol)
	}
	ReleaseIP(containerIP)
	RemoveVeth(containerID)
}
