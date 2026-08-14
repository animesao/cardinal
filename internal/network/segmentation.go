package network

import (
	"fmt"
	"os/exec"
	"strings"

	"dck/internal/log"
)

// NetworkPolicy represents a network segmentation policy.
type NetworkPolicy struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Ingress     []NetworkRule     `json:"ingress,omitempty"`
	Egress      []NetworkRule     `json:"egress,omitempty"`
}

// NetworkRule represents a single network rule.
type NetworkRule struct {
	Protocol    string `json:"protocol,omitempty"`
	Port        int    `json:"port,omitempty"`
	Source      string `json:"source,omitempty"` // CIDR or container name
	Destination string `json:"destination,omitempty"`
	Action      string `json:"action"` // allow, deny
}

// DefaultIsolationPolicy returns a policy that isolates containers from each other.
func DefaultIsolationPolicy() *NetworkPolicy {
	return &NetworkPolicy{
		Name:        "default-isolation",
		Description: "Default policy: deny all inter-container traffic",
		Ingress: []NetworkRule{
			{Protocol: "tcp", Port: 80, Action: "allow"},
			{Protocol: "tcp", Port: 443, Action: "allow"},
		},
		Egress: []NetworkRule{
			{Protocol: "tcp", Action: "allow"},  // Allow all outbound TCP
			{Protocol: "udp", Action: "allow"},  // Allow all outbound UDP
		},
	}
}

// ApplyNetworkPolicy applies a network policy using iptables rules.
func ApplyNetworkPolicy(policy *NetworkPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	// Create a custom chain for this policy
	chainName := fmt.Sprintf("DCK-%s", strings.ToUpper(policy.Name))
	if err := createChain(chainName); err != nil {
		return fmt.Errorf("create chain %s: %w", chainName, err)
	}

	// Apply ingress rules
	for _, rule := range policy.Ingress {
		if err := applyRule(chainName, "INPUT", rule); err != nil {
			log.Warn("apply ingress rule: %v", err)
		}
	}

	// Apply egress rules
	for _, rule := range policy.Egress {
		if err := applyRule(chainName, "OUTPUT", rule); err != nil {
			log.Warn("apply egress rule: %v", err)
		}
	}

	// Add default deny rule at the end of the chain
	if err := addDefaultDeny(chainName); err != nil {
		log.Warn("add default deny: %v", err)
	}

	return nil
}

// RemoveNetworkPolicy removes a network policy.
func RemoveNetworkPolicy(policyName string) error {
	chainName := fmt.Sprintf("DCK-%s", strings.ToUpper(policyName))

	// Flush the chain
	if err := flushChain(chainName); err != nil {
		return fmt.Errorf("flush chain %s: %w", chainName, err)
	}

	// Delete the chain
	if err := deleteChain(chainName); err != nil {
		return fmt.Errorf("delete chain %s: %w", chainName, err)
	}

	return nil
}

// IsolateContainer isolates a container from all other containers.
func IsolateContainer(containerID string) error {
	return applyContainerIsolation(containerID, true)
}

// UnisolateContainer removes isolation from a container.
func UnisolateContainer(containerID string) error {
	return applyContainerIsolation(containerID, false)
}

// applyContainerIsolation applies or removes container isolation.
func applyContainerIsolation(containerID string, isolate bool) error {
	short := networkShortID(containerID)
	iface := fmt.Sprintf("ve%s", short)

	if isolate {
		// Block all traffic from this container to other containers
		rules := []string{
			"-A", "FORWARD", "-i", iface, "-o", BridgeName, "-j", "DROP",
			"-A", "FORWARD", "-i", BridgeName, "-o", iface, "-j", "DROP",
		}
		if err := exec.Command("iptables", rules...).Run(); err != nil {
			return fmt.Errorf("add isolation rules: %w", err)
		}
	} else {
		// Remove isolation rules
		rules := []string{
			"-D", "FORWARD", "-i", iface, "-o", BridgeName, "-j", "DROP",
			"-D", "FORWARD", "-i", BridgeName, "-o", iface, "-j", "DROP",
		}
		if err := exec.Command("iptables", rules...).Run(); err != nil {
			return fmt.Errorf("remove isolation rules: %w", err)
		}
	}

	return nil
}

// AllowContainerCommunication allows communication between two containers.
func AllowContainerCommunication(containerID1, containerID2 string) error {
	short1 := networkShortID(containerID1)
	short2 := networkShortID(containerID2)

	iface1 := fmt.Sprintf("ve%s", short1)
	iface2 := fmt.Sprintf("ve%s", short2)

	// Allow traffic between the two containers
	rules := []string{
		"-I", "FORWARD", "-i", iface1, "-o", iface2, "-j", "ACCEPT",
		"-I", "FORWARD", "-i", iface2, "-o", iface1, "-j", "ACCEPT",
	}

	return exec.Command("iptables", rules...).Run()
}

// DenyContainerCommunication denies communication between two containers.
func DenyContainerCommunication(containerID1, containerID2 string) error {
	short1 := networkShortID(containerID1)
	short2 := networkShortID(containerID2)

	iface1 := fmt.Sprintf("ve%s", short1)
	iface2 := fmt.Sprintf("ve%s", short2)

	// Deny traffic between the two containers
	rules := []string{
		"-I", "FORWARD", "-i", iface1, "-o", iface2, "-j", "DROP",
		"-I", "FORWARD", "-i", iface2, "-o", iface1, "-j", "DROP",
	}

	return exec.Command("iptables", rules...).Run()
}

// GetContainerFirewallRules returns the current firewall rules for a container.
func GetContainerFirewallRules(containerID string) ([]string, error) {
	short := networkShortID(containerID)
	iface := fmt.Sprintf("ve%s", short)

	out, err := exec.Command("iptables", "-L", "FORWARD", "-n", "-v", "--line-numbers").Output()
	if err != nil {
		return nil, err
	}

	var rules []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, iface) {
			rules = append(rules, line)
		}
	}

	return rules, nil
}

// createChain creates a new iptables chain.
func createChain(name string) error {
	// Check if chain exists
	if err := exec.Command("iptables", "-N", name).Run(); err != nil {
		// Chain might already exist, try to flush
		if err := flushChain(name); err != nil {
			return err
		}
	}
	return nil
}

// flushChain flushes all rules from a chain.
func flushChain(name string) error {
	return exec.Command("iptables", "-F", name).Run()
}

// deleteChain deletes an iptables chain.
func deleteChain(name string) error {
	return exec.Command("iptables", "-X", name).Run()
}

// applyRule applies a single network rule.
func applyRule(chainName, table string, rule NetworkRule) error {
	args := []string{"-A", chainName}

	// Protocol
	if rule.Protocol != "" {
		args = append(args, "-p", rule.Protocol)
	}

	// Port
	if rule.Port > 0 {
		args = append(args, "--dport", fmt.Sprintf("%d", rule.Port))
	}

	// Source
	if rule.Source != "" {
		args = append(args, "-s", rule.Source)
	}

	// Destination
	if rule.Destination != "" {
		args = append(args, "-d", rule.Destination)
	}

	// Action
	switch rule.Action {
	case "allow":
		args = append(args, "-j", "ACCEPT")
	case "deny", "drop":
		args = append(args, "-j", "DROP")
	default:
		args = append(args, "-j", "ACCEPT")
	}

	return exec.Command("iptables", args...).Run()
}

// addDefaultDeny adds a default deny rule at the end of a chain.
func addDefaultDeny(chainName string) error {
	return exec.Command("iptables", "-A", chainName, "-j", "DROP").Run()
}

// ListContainerPolicies returns all network policies applied to a container.
func ListContainerPolicies(containerID string) ([]string, error) {
	// Get all iptables rules and filter for this container
	out, err := exec.Command("iptables-save").Output()
	if err != nil {
		return nil, err
	}

	short := networkShortID(containerID)
	var policies []string

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, short) {
			policies = append(policies, line)
		}
	}

	return policies, nil
}

// SetupNetworkSegmentation sets up basic network segmentation.
func SetupNetworkSegmentation() error {
	// Create a chain for inter-container traffic
	chainName := "DCK-INTER-CONTAINER"
	if err := createChain(chainName); err != nil {
		return fmt.Errorf("create inter-container chain: %w", err)
	}

	// Add the chain to the FORWARD chain
	// Insert at the beginning to ensure it's evaluated first
	if err := exec.Command("iptables", "-I", "FORWARD", "1", "-j", chainName).Run(); err != nil {
		return fmt.Errorf("add chain to FORWARD: %w", err)
	}

	// Default: allow all inter-container traffic (can be restricted per-policy)
	if err := exec.Command("iptables", "-A", chainName, "-i", BridgeName, "-o", BridgeName, "-j", "ACCEPT").Run(); err != nil {
		return fmt.Errorf("add default allow rule: %w", err)
	}

	return nil
}

// TeardownNetworkSegmentation removes network segmentation rules.
func TeardownNetworkSegmentation() error {
	// Remove the chain from FORWARD
	_ = exec.Command("iptables", "-D", "FORWARD", "-j", "DCK-INTER-CONTAINER").Run()

	// Flush and delete the chain
	_ = flushChain("DCK-INTER-CONTAINER")
	_ = deleteChain("DCK-INTER-CONTAINER")

	return nil
}
