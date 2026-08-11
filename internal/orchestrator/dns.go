//go:build linux

package orchestrator

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// DNSEntry maps a service name to container IPs
type DNSEntry struct {
	Name      string    `json:"name"`
	Addresses []string  `json:"addresses"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	dnsCache   = make(map[string]*DNSEntry)
	dnsCacheMu sync.RWMutex
)

// ResolveService returns IP addresses for a service name
func ResolveService(name string) ([]string, error) {
	dnsCacheMu.RLock()
	entry, ok := dnsCache[name]
	dnsCacheMu.RUnlock()

	if ok && time.Since(entry.UpdatedAt) < 30*time.Second {
		return entry.Addresses, nil
	}

	// Refresh: query cluster
	addresses := discoverService(name)
	entry = &DNSEntry{
		Name:      name,
		Addresses: addresses,
		UpdatedAt: time.Now(),
	}

	dnsCacheMu.Lock()
	dnsCache[name] = entry
	dnsCacheMu.Unlock()

	return addresses, nil
}

// UpdateDNSEntry sets the addresses for a service
func UpdateDNSEntry(name string, addresses []string) {
	dnsCacheMu.Lock()
	dnsCache[name] = &DNSEntry{
		Name:      name,
		Addresses: addresses,
		UpdatedAt: time.Now(),
	}
	dnsCacheMu.Unlock()
}

// discoverService tries to find container IPs for a service
func discoverService(name string) []string {
	// In full implementation, query cluster nodes for replicas
	// For now, return empty (will use /etc/hosts injection instead)
	svc, err := GetService(name)
	if err != nil || svc == nil {
		return nil
	}

	replicas, err := GetServiceReplicas(name)
	if err != nil {
		return nil
	}

	addrs := make([]string, 0, len(replicas))
	for _, r := range replicas {
		// Container IP would be stored in replica state
		_ = r
		// Mock: in real impl, read from container inspect
	}

	return addrs
}

// StartDNSServer starts a minimal DNS server for service discovery
func StartDNSServer(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve dns addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen dns: %w", err)
	}
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 512)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		go handleDNSQuery(conn, remote, buf[:n])
	}
}

func handleDNSQuery(conn *net.UDPConn, remote *net.UDPAddr, data []byte) {
	// Minimal DNS response for service discovery
	// Format: query <service>.svc.cluster.local -> returns A records
	if len(data) < 12 {
		return
	}

	// Parse query name
	qname := parseDNSName(data[12:])
	if qname == "" {
		return
	}

	// Extract service name from <service>.svc.cluster.local
	serviceName := extractServiceName(qname)
	if serviceName == "" {
		return
	}

	addresses, err := ResolveService(serviceName)
	if err != nil || len(addresses) == 0 {
		return
	}

	// Build response (minimal)
	resp := buildDNSResponse(data, qname, addresses)
	if _, err := conn.WriteToUDP(resp, remote); err != nil {
		return
	}
}

func parseDNSName(data []byte) string {
	var parts []string
	i := 0
	for i < len(data) {
		length := int(data[i])
		if length == 0 {
			break
		}
		if i+length+1 > len(data) {
			return ""
		}
		parts = append(parts, string(data[i+1:i+1+length]))
		i += length + 1
	}
	return joinLabels(parts)
}

func joinLabels(labels []string) string {
	result := ""
	for i, l := range labels {
		if i > 0 {
			result += "."
		}
		result += l
	}
	return result
}

func extractServiceName(qname string) string {
	// Expect <name>.svc.cluster.local
	parts := splitDNSName(qname)
	if len(parts) >= 4 && parts[1] == "svc" && parts[2] == "cluster" && parts[3] == "local" {
		return parts[0]
	}
	return ""
}

func splitDNSName(name string) []string {
	return split(name, ".")
}

func split(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func buildDNSResponse(query []byte, qname string, addresses []string) []byte {
	resp := make([]byte, len(query)+4+len(addresses)*16)
	copy(resp, query)

	// Set response flag
	resp[2] |= 0x80
	resp[3] |= 0x00

	// Set answer count
	ancount := len(addresses)
	resp[6] = byte(ancount >> 8)
	resp[7] = byte(ancount)

	// Copy question section
	qlen := len(query)
	copy(resp[12:], query[12:qlen])

	// Add answer records
	offset := qlen
	for _, addr := range addresses {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}

		// Pointer to name
		resp[offset] = 0xc0
		resp[offset+1] = 0x0c
		offset += 2

		// Type A
		resp[offset] = 0x00
		resp[offset+1] = 0x01
		offset += 2

		// Class IN
		resp[offset] = 0x00
		resp[offset+1] = 0x01
		offset += 2

		// TTL (60 seconds)
		resp[offset] = 0x00
		resp[offset+1] = 0x00
		resp[offset+2] = 0x00
		resp[offset+3] = 60
		offset += 4

		// Data length (4 bytes for IPv4)
		resp[offset] = 0x00
		resp[offset+1] = 0x04
		offset += 2

		// IP address
		ip4 := ip.To4()
		resp[offset] = ip4[0]
		resp[offset+1] = ip4[1]
		resp[offset+2] = ip4[2]
		resp[offset+3] = ip4[3]
		offset += 4
	}

	return resp[:offset]
}

// AddDNSRecord adds service DNS (used for /etc/hosts injection)
func AddDNSRecord(containerName, serviceName, containerIP string) {
	dnsCacheMu.Lock()
	entry, ok := dnsCache[serviceName]
	if !ok {
		entry = &DNSEntry{
			Name:      serviceName,
			Addresses: []string{containerIP},
			UpdatedAt: time.Now(),
		}
		dnsCache[serviceName] = entry
	} else {
		// Add if not already present
		found := false
		for _, a := range entry.Addresses {
			if a == containerIP {
				found = true
				break
			}
		}
		if !found {
			entry.Addresses = append(entry.Addresses, containerIP)
		}
		entry.UpdatedAt = time.Now()
	}
	dnsCacheMu.Unlock()
}

// RemoveDNSRecord removes a container IP from service DNS
func RemoveDNSRecord(containerName, serviceName, containerIP string) {
	dnsCacheMu.Lock()
	if entry, ok := dnsCache[serviceName]; ok {
		updated := make([]string, 0, len(entry.Addresses))
		for _, a := range entry.Addresses {
			if a != containerIP {
				updated = append(updated, a)
			}
		}
		entry.Addresses = updated
		entry.UpdatedAt = time.Now()
	}
	dnsCacheMu.Unlock()
}
