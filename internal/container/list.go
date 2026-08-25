//go:build linux

package container

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"cardinal/internal/state"
)

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// A zombie (defunct) process has already exited; only its kernel stub
	// remains. /proc/<pid> still exists for zombies, so a plain Stat() would
	// report them alive and stall exit detection until the parent (systemd, on
	// a host where the detached CLI is long gone) gets around to reaping it.
	// Read the state from /proc/<pid>/stat and treat Z/X as dead.
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// Format: pid (comm) state ppid ... — comm may contain spaces and parens,
	// so parse from the last closing paren.
	if i := strings.LastIndexByte(string(b), ')'); i >= 0 {
		fields := strings.Fields(string(b[i+1:]))
		if len(fields) > 0 && (fields[0] == "Z" || fields[0] == "X") {
			return false
		}
	}
	return true
}

// processCmdline returns the NUL-separated command line of a process joined
// with spaces, or "" when the process is gone or unreadable.
func processCmdline(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(b), "\x00", " ")
}

func List(all bool) ([]*Container, error) {
	entries, err := os.ReadDir(state.ContainersDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var containers []*Container
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		c, err := Load(id)
		if err != nil {
			continue
		}
		if !all && c.Status != Running {
			continue
		}
		containers = append(containers, c)
	}
	return containers, nil
}

func PrintContainers(containers []*Container) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tIMAGE\tSTATUS\tNAME\tCMD")
	for _, c := range containers {
		shortID := shortID(c.ID, 12)
		image := fmt.Sprintf("%s:%s", c.ImageName, c.ImageTag)
		cmd := strings.Join(c.Cmd, " ")
		if len(cmd) > 40 {
			cmd = cmd[:40] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			shortID, image, c.Status, c.Name, cmd)
	}
	_ = w.Flush()
}
