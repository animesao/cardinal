//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dck/internal/log"
)

const (
	cgroupRoot  = "/sys/fs/cgroup"
	dckCgroup   = "dck"
	cpuPeriod   = 100000
)

func cgroupV2Enabled() bool {
	_, err := os.Stat(filepath.Join(cgroupRoot, "cgroup.controllers"))
	return err == nil
}

func enableCgroupController(ctrl string) error {
	controllersPath := filepath.Join(cgroupRoot, "cgroup.controllers")
	data, err := os.ReadFile(controllersPath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), ctrl) {
		return fmt.Errorf("controller %s not available on this system", ctrl)
	}

	subPath := filepath.Join(cgroupRoot, "cgroup.subtree_control")
	data, err = os.ReadFile(subPath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), ctrl) {
		if err := os.WriteFile(subPath, []byte("+"+ctrl+"\n"), 0644); err != nil {
			return fmt.Errorf("enable %s controller: %w", ctrl, err)
		}
	}
	return nil
}



func setupContainerCgroup(id string, pid int, memoryLimit int64, cpuCount float64) (string, error) {
	basePath := filepath.Join(cgroupRoot, dckCgroup)
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", fmt.Errorf("cgroup base: %w", err)
	}

	// Enable controllers for dck cgroup's children (container cgroups)
	if cgroupV2Enabled() {
		enableCgroupController("memory")
		enableCgroupController("cpu")
		enableCgroupController("pids")

		// Also enable in dck's own subtree so children inherit controllers
		dckSub := filepath.Join(basePath, "cgroup.subtree_control")
		for _, ctrl := range []string{"memory", "cpu", "pids"} {
			data, _ := os.ReadFile(dckSub)
			if !strings.Contains(string(data), ctrl) {
				if err := os.WriteFile(dckSub, []byte("+"+ctrl+"\n"), 0644); err != nil {
					log.Warn("enable cgroup controller %s: %v", ctrl, err)
				}
			}
		}
	}

	cPath := filepath.Join(basePath, id)
	if err := os.MkdirAll(cPath, 0755); err != nil {
		return "", fmt.Errorf("cgroup dir: %w", err)
	}

	if memoryLimit > 0 {
		val := strconv.FormatInt(memoryLimit, 10)
		if err := os.WriteFile(filepath.Join(cPath, "memory.max"), []byte(val), 0644); err != nil {
			log.Warn("set memory.max: %v", err)
		}
	}

	if cpuCount > 0 {
		quota := int64(cpuCount * float64(cpuPeriod))
		val := fmt.Sprintf("%d %d", quota, cpuPeriod)
		if err := os.WriteFile(filepath.Join(cPath, "cpu.max"), []byte(val), 0644); err != nil {
			log.Warn("set cpu.max: %v", err)
		}
	}

	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(filepath.Join(cPath, "cgroup.procs"), []byte(pidStr), 0644); err != nil {
		log.Warn("set cgroup.procs: %v", err)
	}

	return cPath, nil
}

func cleanupContainerCgroup(id, cgroupPath string) {
	if cgroupPath == "" {
		return
	}
	if b, err := os.ReadFile(filepath.Join(cgroupPath, "cgroup.procs")); err == nil && len(b) > 0 {
		parentProcs := filepath.Join(filepath.Dir(cgroupPath), "cgroup.procs")
		os.WriteFile(parentProcs, b, 0644)
	}
	os.RemoveAll(cgroupPath)
}
