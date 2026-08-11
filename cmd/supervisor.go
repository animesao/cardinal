//go:build linux

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"dck/internal/container"
	"dck/internal/network"
)

const supervisorPollInterval = time.Second

var (
	backupInFlight sync.Map
	backupWorkers  sync.WaitGroup
)

// Supervisor keeps detached restart-policy containers alive independently of a
// short-lived `dck run -d` CLI process. The container monitor owns crash
// restart delays; this process only adopts containers that are not yet managed.
func Supervisor(args []string) {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: dck supervisor")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	defer backupWorkers.Wait()

	// Become a subreaper so orphaned container processes — whose `dck run -d`
	// parent already exited — are reparented here instead of to init, and their
	// exits can be observed and reaped below.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "dck supervisor: set child subreaper: %v\n", err)
	}
	network.EnsureNetBase()
	managed := make(map[string]time.Time)
	adoptEligibleContainers(managed)

	ticker := time.NewTicker(supervisorPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reapExitedContainers()
			adoptEligibleContainers(managed)
			runAutomaticBackups()
		}
	}
}

// reapExitedContainers reaps orphaned child processes (detached container
// unshare processes whose CLI parent already exited) and finalizes container
// state and resources for each one.
func reapExitedContainers() {
	for {
		var ws unix.WaitStatus
		pid, err := unix.Wait4(-1, &ws, unix.WNOHANG, nil)
		if err == unix.ECHILD || pid <= 0 {
			return
		}
		handleExitedContainerProcess(pid)
	}
}

func handleExitedContainerProcess(pid int) {
	all, err := container.List(true)
	if err != nil {
		return
	}
	for _, c := range all {
		if c.UnsharePID == pid {
			container.HandleMainProcessExit(c.ID)
			return
		}
	}
	// Other orphaned helpers (console-serve, port forwarders) need no action;
	// they are cleaned up when their owning container is stopped or removed.
}

func adoptEligibleContainers(managed map[string]time.Time) {
	all, err := container.List(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dck supervisor: list containers: %v\n", err)
		return
	}
	for _, c := range all {
		if c == nil {
			continue
		}
		if c.StoppedByUser || c.Restart == "no" || c.Restart == "" {
			delete(managed, c.ID)
			continue
		}
		if c.Status == container.Running {
			if !c.Detach {
				continue
			}
			// A running container is healthy from the supervisor's point of view;
			// clear any stale restart deadline from an earlier failed start.
			managed[c.ID] = time.Time{}
			continue
		}
		deadline, alreadyManaged := managed[c.ID]
		if !alreadyManaged {
			deadline = time.Now()
			if !c.LastExitAt.IsZero() {
				deadline = c.LastExitAt.Add(supervisorRestartDelay(c))
			}
			managed[c.ID] = deadline
		}
		// A container that was running and then crashed has a zero deadline
		// recorded while it was healthy; recompute it from LastExitAt so the
		// configured --restart-delay is honored instead of restarting at once.
		if deadline.IsZero() && !c.LastExitAt.IsZero() {
			deadline = c.LastExitAt.Add(supervisorRestartDelay(c))
			managed[c.ID] = deadline
		}
		if time.Now().Before(deadline) {
			continue
		}
		if !eligibleForSupervisor(c) {
			continue
		}
		// Honor the crash-loop budget: each automatic start consumes one attempt
		// and the container is blocked once the window budget is exhausted.
		if !container.AllowAutomaticRestart(c) {
			continue
		}
		managed[c.ID] = time.Time{}
		if err := c.Start(); err != nil {
			managed[c.ID] = time.Now().Add(supervisorRestartDelay(c))
			fmt.Fprintf(os.Stderr, "dck supervisor: start %s: %v\n", c.Name, err)
		}
	}
}

func runAutomaticBackups() {
	all, err := container.List(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dck supervisor: list backup containers: %v\n", err)
		return
	}
	now := time.Now()
	for _, c := range all {
		if c == nil || !c.AutoBackup || c.StoppedByUser || c.Status != container.Running {
			continue
		}
		interval := automaticBackupInterval(c)
		if !c.BackupNextAttemptAt.IsZero() && now.Before(c.BackupNextAttemptAt) {
			continue
		}
		if !c.LastBackupAt.IsZero() && now.Before(c.LastBackupAt.Add(interval)) {
			continue
		}
		if _, loaded := backupInFlight.LoadOrStore(c.ID, struct{}{}); loaded {
			continue
		}
		backupWorkers.Add(1)
		go func(c *container.Container, now time.Time, interval time.Duration) {
			defer backupWorkers.Done()
			defer backupInFlight.Delete(c.ID)
			// performAutomaticBackup records LastBackupAt only after the archive
			// and retention pass succeed. Failed attempts get a persisted backoff.
			if err := performAutomaticBackup(c); err != nil {
				c.BackupNextAttemptAt = now.Add(minBackupRetryDelay(interval))
				if saveErr := c.Save(); saveErr != nil {
					fmt.Fprintf(os.Stderr, "dck supervisor: save backup retry %s: %v\n", c.Name, saveErr)
				}
				fmt.Fprintf(os.Stderr, "dck supervisor: backup %s: %v\n", c.Name, err)
			}
		}(c, now, interval)
	}
}

func minBackupRetryDelay(interval time.Duration) time.Duration {
	if interval < time.Minute {
		return interval
	}
	return time.Minute
}

func supervisorRestartDelay(c *container.Container) time.Duration {
	if c != nil && c.RestartDelay != "" {
		if delay, err := time.ParseDuration(c.RestartDelay); err == nil && delay > 0 {
			return delay
		}
	}
	return time.Second
}

func eligibleForSupervisor(c *container.Container) bool {
	if c == nil || !c.Detach || c.StoppedByUser || c.Status == container.Running || c.RestartBlocked {
		return false
	}
	switch c.Restart {
	case "always", "unless-stopped":
		return true
	default:
		return false
	}
}
