//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/log"
	"dck/internal/state"
)

func (c *Container) Remove(force bool) error {
	// Never trust the persisted status alone: stale state can say "stopped"
	// while the process tree is still alive (recorded-PID races, pre-1.23
	// states). Detect live processes directly before deleting anything.
	if c.hasLiveProcesses() {
		if !force {
			return fmt.Errorf("cannot remove running container %s (use -f)", c.ID)
		}
		if err := c.Stop(); err != nil {
			log.Warn("stop container %s during remove: %v", c.ID, err)
		}
	}

	// console-serve can outlive the container state; never leak it.
	c.killConsoleServe()
	c.cleanupRootlessPorts()
	c.cleanupNetwork()
	cleanupContainerCgroup(c.ID, c.CgroupPath)

	upper, _, merged := c.OverlayDirs()
	unmountOverlay(merged)
	TeardownDiskLimit(state.OverlayDir(), c.ID)
	if err := os.RemoveAll(filepath.Dir(upper)); err != nil {
		return fmt.Errorf("remove container overlay: %w", err)
	}
	os.Remove(c.LogFile())
	os.Remove(state.ConsolePath(c.ID))
	if err := c.DeleteState(); err != nil {
		return fmt.Errorf("delete container state: %w", err)
	}
	EmitEvent(EventDestroy, c)

	return nil
}

// hasLiveProcesses reports whether any recorded process (init, unshare,
// console-serve) or anything accounted to the container cgroup is still alive.
func (c *Container) hasLiveProcesses() bool {
	if pidAlive(c.PID) || (c.UnsharePID > 0 && pidAlive(c.UnsharePID)) || pidAlive(c.ConsoleServePID) {
		return true
	}
	if c.CgroupPath != "" {
		if b, err := os.ReadFile(filepath.Join(c.CgroupPath, "cgroup.procs")); err == nil && strings.TrimSpace(string(b)) != "" {
			return true
		}
	}
	return false
}
