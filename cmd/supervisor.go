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
			adoptEligibleContainers(managed)
			runAutomaticBackups()
		}
	}
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
		if time.Now().Before(deadline) {
			continue
		}
		if !eligibleForSupervisor(c) {
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
