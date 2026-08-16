//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ContainerStats struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Status      Status `json:"status"`
	PID         int    `json:"pid"`

	MemoryUsage   uint64  `json:"memory_usage_bytes"`
	MemoryLimit   uint64  `json:"memory_limit_bytes"`
	MemoryMax     uint64  `json:"memory_max_bytes"`
	MemoryPercent float64 `json:"memory_percent"`

	CPUUsage   uint64  `json:"cpu_usage_usec"`
	CPUUser    uint64  `json:"cpu_user_usec"`
	CPUSystem  uint64  `json:"cpu_system_usec"`
	CPUPercent float64 `json:"cpu_percent"`
	CPUCount   float64 `json:"cpu_count"`

	PIDsCurrent uint64 `json:"pids_current"`

	IOReadBytes  uint64 `json:"io_read_bytes"`
	IOWriteBytes uint64 `json:"io_write_bytes"`

	DiskUsage uint64 `json:"disk_usage_bytes"`
	DiskLimit uint64 `json:"disk_limit_bytes"`

	Timestamp int64 `json:"timestamp"`
}

func ReadContainerStats(c *Container) (*ContainerStats, error) {
	cgPath := c.CgroupPath
	if cgPath == "" {
		cgPath = filepath.Join("/sys/fs/cgroup/dck", c.ID)
	}

	s := &ContainerStats{
		ContainerID: shortID(c.ID, 12),
		Name:        c.Name,
		Status:      c.Status,
		PID:         c.PID,
		MemoryLimit: uint64(maxInt64(c.MemoryLimit)),
		CPUCount:    c.CPUCount,
		Timestamp:   time.Now().UnixNano(),
	}

	readMemoryStats(s, cgPath)
	readCPUStats(s, cgPath)
	readPIDsStats(s, cgPath)
	readIOStats(s, cgPath)
	readDiskStats(s, c)

	if s.MemoryLimit > 0 {
		s.MemoryPercent = float64(s.MemoryUsage) / float64(s.MemoryLimit) * 100
	}

	return s, nil
}

func readMemoryStats(s *ContainerStats, cgPath string) {
	val, err := readUint64(filepath.Join(cgPath, "memory.current"))
	if err == nil {
		s.MemoryUsage = val
	}

	data, err := os.ReadFile(filepath.Join(cgPath, "memory.max"))
	if err == nil {
		trimmed := strings.TrimSpace(string(data))
		if trimmed != "max" {
			if limit, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
				s.MemoryLimit = limit
			}
		}
	}
}

func readCPUStats(s *ContainerStats, cgPath string) {
	data, err := os.ReadFile(filepath.Join(cgPath, "cpu.stat"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		switch parts[0] {
		case "usage_usec":
			s.CPUUsage = val
		case "user_usec":
			s.CPUUser = val
		case "system_usec":
			s.CPUSystem = val
		}
	}

	data, err = os.ReadFile(filepath.Join(cgPath, "cpu.max"))
	if err != nil {
		return
	}
	parts := strings.Fields(string(data))
	if len(parts) >= 2 {
		quotaStr := parts[0]
		if quotaStr != "max" {
			quota, err := strconv.ParseUint(quotaStr, 10, 64)
			if err == nil {
				period, _ := strconv.ParseUint(parts[1], 10, 64)
				if period > 0 {
					s.CPUCount = float64(quota) / float64(period)
				}
			}
		}
	}
}

func readDiskStats(s *ContainerStats, c *Container) {
	s.DiskLimit = uint64(maxInt64(c.DiskLimit))
	upper, _, _ := c.OverlayDirs()
	var total uint64
	_ = filepath.WalkDir(upper, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err == nil {
			total += uint64(info.Size())
		}
		return nil
	})
	s.DiskUsage = total
}

func maxInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func readPIDsStats(s *ContainerStats, cgPath string) {
	val, err := readUint64(filepath.Join(cgPath, "pids.current"))
	if err == nil {
		s.PIDsCurrent = val
	}
}

func readIOStats(s *ContainerStats, cgPath string) {
	data, err := os.ReadFile(filepath.Join(cgPath, "io.stat"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, part := range strings.Fields(line) {
			if strings.HasPrefix(part, "rbytes=") {
				s.IOReadBytes, _ = strconv.ParseUint(part[7:], 10, 64)
			} else if strings.HasPrefix(part, "wbytes=") {
				s.IOWriteBytes, _ = strconv.ParseUint(part[7:], 10, 64)
			}
		}
	}
}

func readUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// used for delta-CPU computation
type StatsSnapshot struct {
	CPUUsage  uint64
	Timestamp int64
}

func ComputeCPUPercent(prev, curr *StatsSnapshot) float64 {
	if prev == nil || curr == nil {
		return 0
	}
	cpuDelta := curr.CPUUsage - prev.CPUUsage
	timeDelta := curr.Timestamp - prev.Timestamp
	if timeDelta <= 0 {
		return 0
	}
	return float64(cpuDelta) / float64(timeDelta) * 100
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2fGiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2fMiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2fKiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func PrintContainerStats(s *ContainerStats, prev *StatsSnapshot, showHeader bool) {
	cpuPercent := ComputeCPUPercent(prev, &StatsSnapshot{
		CPUUsage:  s.CPUUsage,
		Timestamp: s.Timestamp,
	})

	if showHeader {
		fmt.Println("CONTAINER\tNAME\tCPU%\tMEM_USAGE\tMEM_LIMIT\tMEM%\tNET_IN\tNET_OUT\tPIDS")
	}

	netIn := formatBytes(s.IOReadBytes)
	netOut := formatBytes(s.IOWriteBytes)
	memUsage := formatBytes(s.MemoryUsage)
	memLimit := formatBytes(s.MemoryLimit)
	if s.MemoryLimit == 0 {
		memLimit = "unlim"
	}

	fmt.Printf("%s\t%s\t%.1f%%\t%s\t%s\t%.1f%%\t%s\t%s\t%d\n",
		s.ContainerID, s.Name, cpuPercent,
		memUsage, memLimit, s.MemoryPercent,
		netIn, netOut, s.PIDsCurrent,
	)
}
