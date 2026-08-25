//go:build linux

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"cardinal/internal/container"
	"cardinal/internal/image"
	"cardinal/internal/state"
)

func Info(args []string) {
	containers, _ := container.List(true)
	var running, stopped int
	for _, c := range containers {
		if c.Status == container.Running {
			running++
		} else {
			stopped++
		}
	}

	images, _ := image.ListImages()

	hostname, _ := os.Hostname()
	kernel := readProc("/proc/version", 3)
	uptime := readUptime()
	cpuModel := readCPUModel()
	cpuCores := runtime.NumCPU()
	cpuPct := readCPUPercent()
	memTotal, memUsed, memPct := readMemInfo()
	diskTotal, diskUsed, diskPct := readDiskUsage("/")
	load1, load5, load15 := readLoadAvg()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %-22s %s\n", "Hostname:", hostname)
	fmt.Printf("  %-22s %s / %s / %s\n", "System:", runtime.GOOS, kernel, runtime.GOARCH)
	fmt.Printf("  %-22s %s\n", "Uptime:", uptime)
	fmt.Printf("  %-22s %s (%d cores)\n", "CPU:", cpuModel, cpuCores)
	fmt.Printf("  %-22s %.1f%%\n", "CPU Usage:", cpuPct)
	fmt.Printf("  %-22s %s / %s (%.1f%%)\n", "Memory:", formatBytes(memUsed), formatBytes(memTotal), memPct)
	fmt.Printf("  %-22s %s / %s (%.1f%%)\n", "Disk:", formatBytes(diskUsed), formatBytes(diskTotal), diskPct)
	fmt.Printf("  %-22s %.2f / %.2f / %.2f\n", "Load Average:", load1, load5, load15)
	rootlessMode := "no"
	if container.IsRootless() {
		rootlessMode = "yes ✓"
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %-22s %s\n", "Data Directory:", state.DataDir())
	fmt.Printf("  %-22s %d\n", "Running Containers:", running)
	fmt.Printf("  %-22s %d\n", "Stopped Containers:", stopped)
	fmt.Printf("  %-22s %d\n", "Images:", len(images))
	fmt.Printf("  %-22s %s\n", "Version:", version)
	fmt.Printf("  %-22s %s\n", "Rootless:", rootlessMode)
	fmt.Println(strings.Repeat("─", 50))
}

func readProc(file string, field int) string {
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	parts := strings.Fields(string(b))
	if field < len(parts) {
		return parts[field]
	}
	return ""
}

func readUptime() string {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	var secs float64
	if _, err := fmt.Sscanf(string(b), "%f", &secs); err != nil {
		return ""
	}
	d := time.Duration(secs) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func readCPUModel() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func readMemInfo() (total, used uint64, pct float64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	var memTotal, memAvail uint64
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			_, _ = fmt.Sscanf(line, "MemTotal: %d kB", &memTotal)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			_, _ = fmt.Sscanf(line, "MemAvailable: %d kB", &memAvail)
		}
	}
	memTotal *= 1024
	memAvail *= 1024
	if memTotal > 0 {
		used = memTotal - memAvail
		pct = float64(used) / float64(memTotal) * 100
	}
	return memTotal, used, pct
}

func readCPUPercent() float64 {
	type cpuTimes struct {
		user, nice, system, idle uint64
	}
	readCPU := func() cpuTimes {
		b, err := os.ReadFile("/proc/stat")
		if err != nil {
			return cpuTimes{}
		}
		var ct cpuTimes
		_, _ = fmt.Sscanf(string(b), "cpu %d %d %d %d", &ct.user, &ct.nice, &ct.system, &ct.idle)
		return ct
	}
	t1 := readCPU()
	if t1.user == 0 {
		return 0
	}
	time.Sleep(100 * time.Millisecond)
	t2 := readCPU()
	total1 := t1.user + t1.nice + t1.system + t1.idle
	total2 := t2.user + t2.nice + t2.system + t2.idle
	idle1 := t1.idle
	idle2 := t2.idle
	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta == 0 {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

func readLoadAvg() (load1, load5, load15 float64) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	_, _ = fmt.Sscanf(string(b), "%f %f %f", &load1, &load5, &load15)
	return load1, load5, load15
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
