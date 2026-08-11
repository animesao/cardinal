package network

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"dck/internal/state"
)

type UserNetwork struct {
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Driver    string          `json:"driver"`
	Subnet    string          `json:"subnet"`
	Gateway   string          `json:"gateway"`
	Bridge    string          `json:"bridge"`
	Allocated map[string]bool `json:"allocated,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

var userNetworkMu sync.Mutex
var networkNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,30}$`)

func networkStateDir() string {
	return filepath.Join(state.DataDir(), "networks")
}

func networkStatePath(name string) string {
	return filepath.Join(networkStateDir(), name+".json")
}

func validateNetworkName(name string) error {
	if !networkNamePattern.MatchString(name) || name == "bridge" || name == "host" || name == "none" || name == "ips" {
		return fmt.Errorf("invalid network name %q", name)
	}
	return nil
}

func normalizeNetworkCIDR(cidr string) (string, string, error) {
	if cidr == "" {
		return "", "", fmt.Errorf("subnet must not be empty")
	}
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil || ip.To4() == nil {
		return "", "", fmt.Errorf("subnet must be an IPv4 CIDR: %q", cidr)
	}
	prefix, bits := network.Mask.Size()
	if bits != 32 || prefix < 16 || prefix > 29 {
		return "", "", fmt.Errorf("subnet %q is unsupported; use /16 through /29", cidr)
	}
	network.IP = network.IP.To4()
	gateway := make(net.IP, net.IPv4len)
	copy(gateway, network.IP)
	gateway[3]++
	if !network.Contains(gateway) {
		return "", "", fmt.Errorf("subnet %q has no usable gateway", cidr)
	}
	return network.String(), gateway.String(), nil
}

func networkBridgeName(name string) string {
	// Linux interface names are limited to 15 bytes. Use a hash-derived suffix
	// so similarly named networks cannot accidentally share a bridge name.
	digest := sha256.Sum256([]byte(name))
	return "dckn-" + hex.EncodeToString(digest[:])[:8]
}

func networkID(name string) string {
	digest := sha256.Sum256([]byte(name))
	return "dckn-" + hex.EncodeToString(digest[:])[:16]
}

func saveUserNetwork(n *UserNetwork) error {
	if err := os.MkdirAll(networkStateDir(), 0700); err != nil {
		return err
	}
	return state.WriteJSON(networkStatePath(n.Name), n)
}

func LoadNetwork(name string) (*UserNetwork, error) {
	if err := validateNetworkName(name); err != nil {
		return nil, err
	}
	var n UserNetwork
	if err := state.ReadJSON(networkStatePath(name), &n); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("network %q not found", name)
		}
		return nil, err
	}
	if n.Allocated == nil {
		n.Allocated = make(map[string]bool)
	}
	return &n, nil
}

func ListNetworks() ([]*UserNetwork, error) {
	entries, err := os.ReadDir(networkStateDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var networks []*UserNetwork
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "ips.json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		n, err := LoadNetwork(name)
		if err == nil {
			networks = append(networks, n)
		}
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	return networks, nil
}

func CreateNetwork(name, subnet string) (*UserNetwork, error) {
	userNetworkMu.Lock()
	defer userNetworkMu.Unlock()
	if err := validateNetworkName(name); err != nil {
		return nil, err
	}
	if _, err := os.Stat(networkStatePath(name)); err == nil {
		return nil, fmt.Errorf("network %q already exists", name)
	}
	if subnet == "" {
		subnet = nextNetworkSubnet()
	}
	normalized, gateway, err := normalizeNetworkCIDR(subnet)
	if err != nil {
		return nil, err
	}
	if cidrOverlaps(normalized, BridgeCIDR) {
		return nil, fmt.Errorf("subnet %q overlaps the built-in bridge %s", normalized, BridgeCIDR)
	}
	for _, existing := range mustListNetworks() {
		if cidrOverlaps(normalized, existing.Subnet) {
			return nil, fmt.Errorf("subnet %q overlaps network %q (%s)", normalized, existing.Name, existing.Subnet)
		}
	}
	n := &UserNetwork{
		Name:      name,
		ID:        networkID(name),
		Driver:    "bridge",
		Subnet:    normalized,
		Gateway:   gateway,
		Bridge:    networkBridgeName(name),
		Allocated: make(map[string]bool),
		CreatedAt: time.Now(),
	}
	if err := saveUserNetwork(n); err != nil {
		return nil, fmt.Errorf("save network: %w", err)
	}
	return n, nil
}

func nextNetworkSubnet() string {
	used := make(map[string]bool)
	for _, n := range mustListNetworks() {
		used[n.Subnet] = true
	}
	for octet := 10; octet < 250; octet++ {
		candidate := fmt.Sprintf("10.0.%d.0/24", octet)
		if !used[candidate] && candidate != BridgeCIDR {
			return candidate
		}
	}
	return "10.254.0.0/24"
}

func mustListNetworks() []*UserNetwork {
	networks, _ := ListNetworks()
	return networks
}

func cidrOverlaps(left, right string) bool {
	_, leftNet, leftErr := net.ParseCIDR(left)
	_, rightNet, rightErr := net.ParseCIDR(right)
	if leftErr != nil || rightErr != nil {
		return true
	}
	return leftNet.Contains(rightNet.IP) || rightNet.Contains(leftNet.IP)
}

func EnsureUserNetwork(name string) error {
	n, err := LoadNetwork(name)
	if err != nil {
		return err
	}
	createdBridge := false
	if err := exec.Command("ip", "link", "show", n.Bridge).Run(); err != nil {
		if err := exec.Command("ip", "link", "add", n.Bridge, "type", "bridge").Run(); err != nil {
			return fmt.Errorf("create bridge %s: %w", n.Bridge, err)
		}
		createdBridge = true
		if err := exec.Command("ip", "addr", "add", n.Gateway+"/"+cidrPrefix(n.Subnet), "dev", n.Bridge).Run(); err != nil {
			_ = exec.Command("ip", "link", "delete", n.Bridge).Run()
			return fmt.Errorf("assign gateway %s: %w", n.Gateway, err)
		}
	}
	if err := exec.Command("ip", "link", "set", n.Bridge, "up").Run(); err != nil {
		if createdBridge {
			_ = exec.Command("ip", "link", "delete", n.Bridge).Run()
		}
		return fmt.Errorf("enable bridge %s: %w", n.Bridge, err)
	}
	if err := ensureNetworkFirewall(n); err != nil {
		if createdBridge {
			_ = exec.Command("ip", "link", "delete", n.Bridge).Run()
		}
		return err
	}
	return nil
}

func CIDRPrefix(cidr string) string {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return "24"
	}
	prefix, _ := network.Mask.Size()
	return fmt.Sprintf("%d", prefix)
}

func cidrPrefix(cidr string) string {
	return CIDRPrefix(cidr)
}

func ensureNetworkFirewall(n *UserNetwork) error {
	masqRule := []string{"-s", n.Subnet, "!", "-o", n.Bridge, "-j", "MASQUERADE"}
	masqCheck := append([]string{"-t", "nat", "-C", "POSTROUTING"}, masqRule...)
	masqAdded := false
	if err := exec.Command("iptables", masqCheck...).Run(); err != nil {
		if err := exec.Command("iptables", append([]string{"-t", "nat", "-A", "POSTROUTING"}, masqRule...)...).Run(); err != nil {
			return fmt.Errorf("configure network NAT: %w", err)
		}
		masqAdded = true
	}

	addedForward := make([][]string, 0, 2)
	rollback := func() {
		if masqAdded {
			_ = exec.Command("iptables", append([]string{"-t", "nat", "-D", "POSTROUTING"}, masqRule...)...).Run()
		}
		for _, rule := range addedForward {
			_ = exec.Command("iptables", append([]string{"-D"}, rule...)...).Run()
		}
	}
	for _, rule := range [][]string{{"FORWARD", "-i", n.Bridge, "-j", "ACCEPT"}, {"FORWARD", "-o", n.Bridge, "-j", "ACCEPT"}} {
		check := append([]string{"-C"}, rule...)
		if err := exec.Command("iptables", check...).Run(); err != nil {
			add := append([]string{"-A"}, rule...)
			if err := exec.Command("iptables", add...).Run(); err != nil {
				rollback()
				return fmt.Errorf("configure network forwarding: %w", err)
			}
			addedForward = append(addedForward, rule)
		}
	}
	return nil
}

func AllocateNetworkIP(name string) (string, error) {
	userNetworkMu.Lock()
	defer userNetworkMu.Unlock()
	n, err := LoadNetwork(name)
	if err != nil {
		return "", err
	}
	_, network, err := net.ParseCIDR(n.Subnet)
	if err != nil {
		return "", err
	}
	prefix, bits := network.Mask.Size()
	hosts := 1 << uint(bits-prefix)
	for host := 2; host < hosts-1; host++ {
		ip := ipv4Add(network.IP.To4(), host)
		if ip == nil {
			continue
		}
		value := ip.String()
		if !n.Allocated[value] {
			n.Allocated[value] = true
			if err := saveUserNetwork(n); err != nil {
				return "", err
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("no available IP addresses in %s", n.Subnet)
}

func ipv4Add(base net.IP, offset int) net.IP {
	if len(base) != net.IPv4len {
		return nil
	}
	value := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	value += uint32(offset)
	return net.IPv4(byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}

func ReleaseNetworkIP(name, ip string) {
	userNetworkMu.Lock()
	defer userNetworkMu.Unlock()
	n, err := LoadNetwork(name)
	if err != nil {
		return
	}
	delete(n.Allocated, ip)
	_ = saveUserNetwork(n)
}

func removeNetworkFirewall(n *UserNetwork) {
	if _, lookErr := exec.LookPath("iptables"); lookErr != nil {
		return
	}
	_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", n.Subnet, "!", "-o", n.Bridge, "-j", "MASQUERADE").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-i", n.Bridge, "-j", "ACCEPT").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-o", n.Bridge, "-j", "ACCEPT").Run()
}

func RemoveNetwork(name string) error {
	userNetworkMu.Lock()
	defer userNetworkMu.Unlock()
	n, err := LoadNetwork(name)
	if err != nil {
		return err
	}
	if len(n.Allocated) > 0 {
		return fmt.Errorf("network %q is still used by running or stopped containers", name)
	}
	removeNetworkFirewall(n)
	if _, lookErr := exec.LookPath("ip"); lookErr == nil {
		if showErr := exec.Command("ip", "link", "show", n.Bridge).Run(); showErr == nil {
			if err := exec.Command("ip", "link", "delete", n.Bridge).Run(); err != nil {
				return fmt.Errorf("remove bridge %s: %w", n.Bridge, err)
			}
		}
	}
	return os.Remove(networkStatePath(name))
}
