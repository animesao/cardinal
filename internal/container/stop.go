//go:build linux

package container

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dck/internal/log"
	"dck/internal/state"
)

func findUnsharePID(childPID int) int {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(childPID)).Output()
	if err != nil {
		return 0
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || ppid == 0 {
		return 0
	}
	out2, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(ppid)).Output()
	if err != nil {
		return 0
	}
	if strings.TrimSpace(string(out2)) == "unshare" {
		return ppid
	}
	return 0
}

func (c *Container) Stop() error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	c.dataMu.RLock()
	status := c.Status
	pid := c.PID
	c.dataMu.RUnlock()
	if status != Running {
		// A stopped container may still have a delayed automatic restart pending.
		// Treat dck stop as an explicit cancellation instead of reporting an
		// error, so the pending restart cannot bring it back unexpectedly.
		if status == Stopped && c.Restart != "" && c.Restart != "no" {
			c.dataMu.Lock()
			c.StoppedByUser = true
			c.dataMu.Unlock()
			stoppedContainers.Store(c.ID, true)
			if err := c.Save(); err != nil {
				return fmt.Errorf("save stopped container: %w", err)
			}
			return nil
		}
		return fmt.Errorf("container %s is not running", c.ID)
	}

	c.mu.Lock()
	if c.cleanupStarted {
		c.mu.Unlock()
		return nil
	}
	c.cleanupStarted = true
	c.mu.Unlock()
	c.dataMu.Lock()
	c.StoppedByUser = true
	c.dataMu.Unlock()
	stoppedContainers.Store(c.ID, true)

	if err := c.Save(); err != nil {
		return fmt.Errorf("save stopping container: %w", err)
	}

	unsharePID := findUnsharePID(pid)
	targetPID := pid
	if unsharePID != 0 {
		targetPID = unsharePID
	}

	// Graceful shutdown: SIGTERM first, then SIGKILL after timeout.
	// If unshare was started by a previous dck run -d process,
	// --kill-child won't fire on SIGTERM so we must also signal
	// the container init directly.
	//
	// We can't use proc.Wait() — process was reparented to init, so
	// Wait() would return ECHILD. Poll with kill(pid, 0) instead.
	if err := syscall.Kill(targetPID, syscall.SIGTERM); err != nil {
		log.Warn("terminate target PID %d: %v", targetPID, err)
	}
	if waitForExit(targetPID, 5*time.Second) {
		goto cleanup
	}

	if err := syscall.Kill(targetPID, syscall.SIGKILL); err != nil {
		log.Warn("kill target PID %d: %v", targetPID, err)
	}
	waitForExit(targetPID, 2*time.Second)

cleanup:
	// Kill the container init directly (survives if unshare was killed)
	if unsharePID != 0 && pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			log.Warn("terminate container PID %d: %v", pid, err)
		}
		if waitForExit(pid, 3*time.Second) {
			goto postcleanup
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			log.Warn("kill container PID %d: %v", pid, err)
		}
		waitForExit(pid, 2*time.Second)
	}
postcleanup:

	c.cleanupRootlessPorts()
	c.killConsoleServe()
	c.cancelHealthcheck()
	c.cleanupNetwork()
	cleanupContainerCgroup(c.ID, c.CgroupPath)
	os.Remove(state.ConsolePath(c.ID))
	UnregisterDNSName(c.Name)
	c.dataMu.Lock()
	c.PID = 0
	c.Status = Stopped
	c.dataMu.Unlock()
	if err := c.Save(); err != nil {
		return fmt.Errorf("save stopped container: %w", err)
	}
	EmitEvent(EventStop, c)
	return nil
}

func (c *Container) cleanupRootlessPorts() {
	if len(c.PortForwardPIDs) > 0 {
		CleanupRootlessPorts(c.PortForwardPIDs)
		c.PortForwardPIDs = nil
	}
}

func (c *Container) killConsoleServe() {
	if c.ConsoleServePID <= 0 {
		return
	}

	pid := c.ConsoleServePID
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		log.Warn("kill console-serve PID %d: %v", pid, err)
	}
	if err := waitForExit(pid, 2*time.Second); !err {
		log.Warn("console-serve PID %d did not exit before log reset", pid)
	}
	c.ConsoleServePID = 0
}

func (c *Container) cancelHealthcheck() {
	if c.cancelHealth != nil {
		c.cancelHealth()
		c.cancelHealth = nil
	}
}

func isAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
