//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dck/internal/container"
	"dck/internal/state"
)

type diagnosticStatus string

const (
	diagnosticOK   diagnosticStatus = "OK"
	diagnosticWarn diagnosticStatus = "WARN"
	diagnosticFail diagnosticStatus = "FAIL"
)

type diagnostic struct {
	name   string
	status diagnosticStatus
	detail string
}

// Doctor performs read-only host and dck runtime checks. It never installs
// packages, changes system configuration, or starts/stops containers.
func Doctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	strict := fs.Bool("strict", false, "Treat warnings as failures")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: dck doctor [--strict]")
		os.Exit(2)
	}

	checks := collectDiagnostics()
	printDiagnostics("dck doctor", checks)
	if diagnosticsFailed(checks, *strict) {
		os.Exit(1)
	}
}

// Security runs the security-focused subset of the read-only diagnostics.
func Security(args []string) {
	if len(args) == 0 || args[0] == "check" {
		remaining := args
		if len(remaining) > 0 {
			remaining = remaining[1:]
		}
		fs := flag.NewFlagSet("security check", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		strict := fs.Bool("strict", false, "Treat warnings as failures")
		if err := fs.Parse(remaining); err != nil {
			os.Exit(2)
		}
		checks := collectDiagnostics()
		securityChecks := make([]diagnostic, 0, len(checks))
		for _, check := range checks {
			switch check.name {
			case "platform", "user", "data directory", "API exposure":
				securityChecks = append(securityChecks, check)
			default:
				if strings.HasPrefix(check.name, "rootless") {
					securityChecks = append(securityChecks, check)
				}
			}
		}
		printDiagnostics("dck security check", securityChecks)
		if diagnosticsFailed(securityChecks, *strict) {
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "Usage: dck security check [--strict]")
	os.Exit(2)
}

func collectDiagnostics() []diagnostic {
	checks := []diagnostic{
		{name: "platform", status: diagnosticOK, detail: "Linux runtime"},
		{name: "user", status: diagnosticOK, detail: fmt.Sprintf("uid=%d", os.Getuid())},
	}
	if os.Getuid() == 0 {
		checks[1] = diagnostic{name: "user", status: diagnosticWarn, detail: "running as root; prefer a dedicated rootless account"}
	}

	checks = append(checks, checkDataDirectory()...)
	checks = append(checks, checkRequiredCommands()...)
	checks = append(checks, checkKernelFeatures()...)
	checks = append(checks, checkRootless()...)
	checks = append(checks, checkAPIConfiguration()...)
	return checks
}

func checkDataDirectory() []diagnostic {
	dir := state.DataDir()
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return []diagnostic{{name: "data directory", status: diagnosticWarn, detail: fmt.Sprintf("%s does not exist yet; it will be created on first use", dir)}}
	}
	if err != nil {
		return []diagnostic{{name: "data directory", status: diagnosticFail, detail: fmt.Sprintf("cannot inspect %s: %v", dir, err)}}
	}
	if !info.IsDir() {
		return []diagnostic{{name: "data directory", status: diagnosticFail, detail: fmt.Sprintf("%s is not a directory", dir)}}
	}
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return []diagnostic{{name: "data directory", status: diagnosticFail, detail: fmt.Sprintf("%s is mode %04o; expected owner-only permissions", dir, mode)}}
	}
	return []diagnostic{{name: "data directory", status: diagnosticOK, detail: fmt.Sprintf("%s mode %04o", dir, mode)}}
}

func checkRequiredCommands() []diagnostic {
	commands := []string{"unshare", "nsenter", "mount", "umount", "ip", "pgrep"}
	checks := make([]diagnostic, 0, len(commands))
	for _, name := range commands {
		path, err := exec.LookPath(name)
		if err != nil {
			checks = append(checks, diagnostic{name: "command " + name, status: diagnosticFail, detail: "not found in PATH"})
			continue
		}
		checks = append(checks, diagnostic{name: "command " + name, status: diagnosticOK, detail: path})
	}
	if _, err := exec.LookPath("iptables"); err != nil {
		checks = append(checks, diagnostic{name: "command iptables", status: diagnosticWarn, detail: "not found; bridge port forwarding may be unavailable"})
	} else {
		checks = append(checks, diagnostic{name: "command iptables", status: diagnosticOK, detail: "available"})
	}
	return checks
}

func checkKernelFeatures() []diagnostic {
	checks := make([]diagnostic, 0, 3)
	if _, err := os.Stat("/proc/self/ns/user"); err != nil {
		checks = append(checks, diagnostic{name: "user namespaces", status: diagnosticFail, detail: err.Error()})
	} else {
		checks = append(checks, diagnostic{name: "user namespaces", status: diagnosticOK, detail: "available"})
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		checks = append(checks, diagnostic{name: "cgroups", status: diagnosticWarn, detail: "cgroups v2 controller file not found; resource limits may be unavailable"})
	} else {
		checks = append(checks, diagnostic{name: "cgroups", status: diagnosticOK, detail: "cgroups v2 detected"})
	}
	if _, err := os.Stat("/proc/filesystems"); err != nil {
		checks = append(checks, diagnostic{name: "overlayfs", status: diagnosticWarn, detail: "cannot inspect supported filesystems"})
	} else {
		data, readErr := os.ReadFile("/proc/filesystems")
		if readErr != nil || !strings.Contains(string(data), "overlay") {
			checks = append(checks, diagnostic{name: "overlayfs", status: diagnosticWarn, detail: "overlay filesystem was not advertised by the kernel"})
		} else {
			checks = append(checks, diagnostic{name: "overlayfs", status: diagnosticOK, detail: "available"})
		}
	}
	return checks
}

func checkRootless() []diagnostic {
	if !container.IsRootless() {
		return []diagnostic{{name: "rootless prerequisites", status: diagnosticWarn, detail: "not checked because dck is running as root"}}
	}
	warnings, failures := container.CheckRootlessPrereqs()
	checks := make([]diagnostic, 0, len(warnings)+len(failures)+1)
	if len(warnings) == 0 && len(failures) == 0 {
		return []diagnostic{{name: "rootless prerequisites", status: diagnosticOK, detail: "required helpers and UID/GID mappings are present"}}
	}
	for _, failure := range failures {
		checks = append(checks, diagnostic{name: "rootless prerequisite", status: diagnosticFail, detail: failure})
	}
	for _, warning := range warnings {
		checks = append(checks, diagnostic{name: "rootless prerequisite", status: diagnosticWarn, detail: warning})
	}
	return checks
}

func checkAPIConfiguration() []diagnostic {
	hostOverride := strings.TrimSpace(os.Getenv("DCK_HOST"))
	if hostOverride == "" {
		return []diagnostic{{name: "API exposure", status: diagnosticOK, detail: "localhost-only default or no DCK_HOST override"}}
	}
	host, _, err := parseHost(hostOverride)
	if err != nil {
		return []diagnostic{{name: "API exposure", status: diagnosticFail, detail: fmt.Sprintf("invalid DCK_HOST=%q: %v", hostOverride, err)}}
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return []diagnostic{{name: "API exposure", status: diagnosticOK, detail: fmt.Sprintf("localhost bind configured via DCK_HOST=%q", hostOverride)}}
	}
	if strings.TrimSpace(os.Getenv("DCK_TOKEN")) == "" {
		return []diagnostic{{name: "API exposure", status: diagnosticFail, detail: fmt.Sprintf("DCK_HOST=%q is configured without DCK_TOKEN", hostOverride)}}
	}
	return []diagnostic{{name: "API exposure", status: diagnosticOK, detail: "external host has DCK_TOKEN configured"}}
}

func printDiagnostics(title string, checks []diagnostic) {
	fmt.Println(title)
	for _, check := range checks {
		fmt.Printf("  %-26s %-4s %s\n", check.name, check.status, check.detail)
	}
}

func diagnosticsFailed(checks []diagnostic, strict bool) bool {
	for _, check := range checks {
		if check.status == diagnosticFail || (strict && check.status == diagnosticWarn) {
			return true
		}
	}
	return false
}
