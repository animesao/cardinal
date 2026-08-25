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

	"cardinal/internal/log"
	"cardinal/internal/state"
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

// stopGracePeriod is how long the container init gets to shut down gracefully
// after SIGTERM before cardinal escalates to SIGKILL. Long enough for servers such
// as Minecraft/Paper to flush worlds and databases.
const stopGracePeriod = 10 * time.Second

func (c *Container) Stop() error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	c.dataMu.RLock()
	status := c.Status
	pid := c.PID
	unsharePID := c.UnsharePID
	c.dataMu.RUnlock()
	if status != Running {
		// Stale state can claim "stopped" while the process tree is actually
		// still alive (recorded-PID races, pre-1.23 states). Never leave those
		// processes behind.
		if pid > 0 || unsharePID > 0 || pidAlive(c.ConsoleServePID) {
			c.killRecordedProcesses(pid, unsharePID)
			c.cleanupRootlessPorts()
			c.killConsoleServe()
			c.cleanupNetwork()
			cleanupContainerCgroup(c.ID, c.CgroupPath)
		}
		// A stopped container may still have a delayed automatic restart pending.
		// Treat cardinal stop as an explicit cancellation instead of reporting an
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

	// Prefer the recorded unshare PID: killing it makes --kill-child tear down
	// the whole process tree. Fall back to locating it from the init PID.
	targetPID := unsharePID
	if targetPID <= 0 {
		targetPID = findUnsharePID(pid)
	}
	if targetPID <= 0 {
		targetPID = pid
	}
	if targetPID <= 0 {
		// No live process recorded; nothing left to signal. Finalize state only.
		log.Warn("container %s has no recorded process; finalizing state", c.ID)
		c.finalizeStopState()
		return nil
	}

	c.killRecordedProcesses(pid, targetPID)
	c.finalizeStopState()
	return nil
}

// killRecordedProcesses performs graceful shutdown of the container process
// tree. unshare forwards SIGTERM to the container init and exits immediately,
// so the graceful window is measured against the init PID itself (Paper saves
// the world, databases flush, etc.). We can't use proc.Wait() — the process was
// reparented, so Wait() would return ECHILD. Poll with kill(pid, 0) instead.
func (c *Container) killRecordedProcesses(pid, targetPID int) {
	if targetPID <= 0 {
		targetPID = pid
	}
	if targetPID <= 0 {
		return
	}
	if err := syscall.Kill(targetPID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		log.Warn("terminate target PID %d: %v", targetPID, err)
	}
	initPID := pid
	if initPID == targetPID {
		initPID = 0
	}
	if initPID > 0 {
		if err := syscall.Kill(initPID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			log.Warn("terminate container PID %d: %v", initPID, err)
		}
	}

	graceful := false
	if initPID > 0 {
		graceful = waitForExit(initPID, stopGracePeriod)
	} else {
		// No separate init recorded (legacy states): the target itself is the
		// container process, so it gets the graceful window directly.
		graceful = waitForExit(targetPID, stopGracePeriod)
	}
	if !graceful {
		if err := syscall.Kill(targetPID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			log.Warn("kill target PID %d: %v", targetPID, err)
		}
		waitForExit(targetPID, 2*time.Second)
		if initPID > 0 {
			if err := syscall.Kill(initPID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				log.Warn("kill container PID %d: %v", initPID, err)
			}
			waitForExit(initPID, 3*time.Second)
		}
	}
}

// finalizeStopState releases container resources and persists the stopped
// state after a successful stop.
func (c *Container) finalizeStopState() {
	c.cleanupRootlessPorts()
	c.killConsoleServe()
	c.cancelHealthcheck()
	c.cleanupNetwork()
	cleanupContainerCgroup(c.ID, c.CgroupPath)
	os.Remove(state.ConsolePath(c.ID))
	UnregisterDNSName(c.Name)
	c.dataMu.Lock()
	c.PID = 0
	c.UnsharePID = 0
	c.MountNamespace = 0
	c.PIDNamespace = 0
	c.NetworkNamespace = 0
	c.IPCNamespace = 0
	c.UTSNamespace = 0
	c.Status = Stopped
	c.dataMu.Unlock()
	if err := c.Save(); err != nil {
		log.Warn("save stopped container %s: %v", c.Name, err)
	}
	EmitEvent(EventStop, c)
}

func (c *Container) cleanupRootlessPorts() {
	if len(c.PortForwardPIDs) > 0 {
		CleanupRootlessPorts(c.PortForwardPIDs, c.Ports)
		c.PortForwardPIDs = nil
	}
}

func (c *Container) killConsoleServe() {
	if c.ConsoleServePID <= 0 {
		return
	}

	pid := c.ConsoleServePID
	// The stdout pipe EOFs when the container process exits, so console-serve
	// usually drains its read buffer and exits by itself. Give it a moment to
	// flush the final container output to the log before force-killing it;
	// otherwise the last lines (e.g. Minecraft's "Saving worlds") can be lost.
	if waitForExit(pid, time.Second) {
		c.ConsoleServePID = 0
		return
	}
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
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	// kill(pid, 0) also succeeds for zombie (defunct) processes, which would
	// make graceful-stop waits stall for the full timeout waiting on a corpse.
	// A zombie has already exited; treat it as not alive.
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	if i := strings.LastIndexByte(string(b), ')'); i >= 0 {
		fields := strings.Fields(string(b[i+1:]))
		if len(fields) > 0 && (fields[0] == "Z" || fields[0] == "X") {
			return false
		}
	}
	return true
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
