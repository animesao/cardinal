// Package portmap parses container port mapping specs shared by the CLI
// (-p), compose (up) and blueprints. It deliberately depends only on the
// standard library so it stays unit-testable on any platform.
//
// Accepted forms (protocol defaults to tcp):
//
//	<host>:<container>[/proto]
//	<host>:<container-range>[/proto]       host single, container range (docker
//	                                      maps each consecutive host port 1:1)
//	<host-range>:<container>[/proto]       host range → same container port
//	<host-range>:<container-range>[/proto] same length, mapped 1:1
//	<container>[/proto]                    anonymous host port (HostPort 0)
package portmap

import (
	"fmt"
	"strconv"
	"strings"
)

type Mapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

type portRange struct {
	lo int
	hi int
}

// Parse expands a single spec into one or more mappings.
func Parse(s string) ([]Mapping, error) {
	proto := "tcp"
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		proto = strings.ToLower(strings.TrimSpace(parts[1]))
		s = parts[0]
	}
	if proto == "" {
		proto = "tcp"
	}
	// The protocol reaches `iptables -p <proto>`: only real IP protocols
	// pass, so garbage can neither break the rule nor smuggle flags.
	if proto != "tcp" && proto != "udp" && proto != "sctp" && proto != "icmp" {
		return nil, fmt.Errorf("invalid protocol %q (want tcp, udp, sctp or icmp)", proto)
	}

	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		cr, err := parseRange(parts[0], "container port")
		if err != nil {
			return nil, err
		}
		if err := validateRange(cr, true); err != nil {
			return nil, err
		}
		return []Mapping{{HostPort: 0, ContainerPort: cr.lo, Protocol: proto}}, nil
	case 2:
		hr, err := parseRange(parts[0], "host port")
		if err != nil {
			return nil, err
		}
		cr, err := parseRange(parts[1], "container port")
		if err != nil {
			return nil, err
		}
		if err := validateRange(hr, false); err != nil {
			return nil, err
		}
		if err := validateRange(cr, true); err != nil {
			return nil, err
		}

		hostN := hr.hi - hr.lo + 1
		contN := cr.hi - cr.lo + 1
		if hostN > 1 && contN > 1 && hostN != contN {
			return nil, fmt.Errorf("invalid port range %q: host and container ranges must have equal size", s)
		}

		var mappings []Mapping
		switch {
		case hostN > 1 && contN > 1:
			for i := 0; i < hostN; i++ {
				mappings = append(mappings, Mapping{HostPort: hr.lo + i, ContainerPort: cr.lo + i, Protocol: proto})
			}
		case hostN > 1:
			for i := 0; i < hostN; i++ {
				mappings = append(mappings, Mapping{HostPort: hr.lo + i, ContainerPort: cr.lo, Protocol: proto})
			}
		case contN > 1:
			for i := 0; i < contN; i++ {
				mappings = append(mappings, Mapping{HostPort: hr.lo, ContainerPort: cr.lo + i, Protocol: proto})
			}
		default:
			mappings = []Mapping{{HostPort: hr.lo, ContainerPort: cr.lo, Protocol: proto}}
		}
		return mappings, nil
	default:
		return nil, fmt.Errorf("invalid port mapping %q: expected <host>:<container>[/proto]", s)
	}
}

// parseRange accepts "8000" or "8000-8010".
func parseRange(part, what string) (portRange, error) {
	part = strings.TrimSpace(part)
	dash := strings.IndexByte(part, '-')
	if dash < 0 {
		v, err := strconv.Atoi(part)
		if err != nil {
			return portRange{}, fmt.Errorf("invalid %s %q", what, part)
		}
		return portRange{lo: v, hi: v}, nil
	}
	lo, err := strconv.Atoi(part[:dash])
	if err != nil {
		return portRange{}, fmt.Errorf("invalid %s range %q", what, part)
	}
	hi, err := strconv.Atoi(part[dash+1:])
	if err != nil {
		return portRange{}, fmt.Errorf("invalid %s range %q", what, part)
	}
	if hi < lo {
		return portRange{}, fmt.Errorf("invalid %s range %q: start greater than end", what, part)
	}
	return portRange{lo: lo, hi: hi}, nil
}

func validateRange(r portRange, container bool) error {
	// Host port 0 means "pick a random free port"; container ports must be real.
	min := 1
	if !container {
		min = 0
	}
	if r.lo < min || r.hi > 65535 {
		kind := "host"
		if container {
			kind = "container"
		}
		return fmt.Errorf("%s port %d-%d out of range (%d..65535)", kind, r.lo, r.hi, min)
	}
	return nil
}
