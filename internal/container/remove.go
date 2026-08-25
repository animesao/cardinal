//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cardinal/internal/log"
	"cardinal/internal/state"
)

// removalMarkerPath returns the tombstone path that marks a container as being
// removed, so a concurrent supervisor automatic restart can never resurrect it.
func removalMarkerPath(id string) string {
	return filepath.Join(state.ContainersDir(), id+".removing")
}

// IsBeingRemoved reports whether a removal is currently in progress for the
// container. The supervisor checks this before an automatic restart so that
// `cardinal rm` tearing a container down is never raced by the restart loop.
func IsBeingRemoved(id string) bool {
	return state.FileExists(removalMarkerPath(id))
}

func (c *Container) Remove(force bool) error {
	// Write the removal tombstone FIRST, before any check or cleanup: the
	// supervisor re-loads container state before each automatic restart, and
	// the rest of Remove (Stop grace period, resource cleanup, DeleteState)
	// takes far longer than a supervisor poll interval. Without the marker a
	// crash-looping container could be resurrected while we are still removing
	// it (exactly what was observed: cardinal rm -f printed the ID, then the
	// container reappeared in cardinal ps -a). The defer removes the marker on every
	// return path, including the !force error below.
	marker := removalMarkerPath(c.ID)
	if err := os.WriteFile(marker, []byte("removing\n"), 0600); err != nil {
		log.Warn("write removal marker %s: %v", marker, err)
	}
	defer func() { _ = os.Remove(marker) }()

	// Never trust the persisted status alone: stale state can say "stopped"
	// while the process tree is still alive (recorded-PID races, states written
	// before unshare PID tracking). Detect live processes directly before
	// deleting anything.
	if c.hasLiveProcesses() {
		if !force {
			return fmt.Errorf("cannot remove running container %s (use -f)", c.ID)
		}
		if c.Status == Running {
			if err := c.Stop(); err != nil {
				log.Warn("stop container %s during remove: %v", c.ID, err)
			}
		} else {
			// Stale state: the status says "stopped" but processes are alive.
			// Stop them directly without Stop()'s status checks.
			c.killRecordedProcesses(c.PID, c.UnsharePID)
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
	if c.processAlive() || pidAlive(c.ConsoleServePID) {
		return true
	}
	if c.CgroupPath != "" {
		if b, err := os.ReadFile(filepath.Join(c.CgroupPath, "cgroup.procs")); err == nil && strings.TrimSpace(string(b)) != "" {
			return true
		}
	}
	return false
}
