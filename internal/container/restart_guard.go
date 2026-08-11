//go:build linux

package container

import "time"

const (
	defaultRestartMaxAttempts = 5
	defaultRestartWindow      = 10 * time.Minute
	stableRuntimeWindow       = time.Minute
)

func (c *Container) restartMaxAttempts() int {
	if c == nil || c.RestartMaxAttempts <= 0 {
		return defaultRestartMaxAttempts
	}
	return c.RestartMaxAttempts
}

func (c *Container) restartWindowDuration() time.Duration {
	if c != nil && c.RestartWindow != "" {
		if window, err := time.ParseDuration(c.RestartWindow); err == nil && window > 0 {
			return window
		}
	}
	return defaultRestartWindow
}

// resetRestartGuard clears a previous crash-loop block after an explicit start
// or after a process has stayed alive long enough to be considered stable.
func (c *Container) resetRestartGuard(now time.Time) {
	c.dataMu.Lock()
	c.RestartAttempts = 0
	c.RestartWindowStart = now
	c.RestartBlocked = false
	c.dataMu.Unlock()
}

// ResetRestartGuard clears a previous crash-loop block after an explicit user
// start or restart command.
func (c *Container) ResetRestartGuard() {
	c.resetRestartGuard(time.Now())
}

// allowAutomaticRestart records one restart attempt and returns false once the
// configured crash-loop budget is exhausted. State is persisted so a supervisor
// restart cannot accidentally bypass the guard.
func (c *Container) allowAutomaticRestart(now time.Time) bool {
	c.dataMu.Lock()
	if c.RestartBlocked {
		c.dataMu.Unlock()
		return false
	}
	window := c.restartWindowDuration()
	if c.RestartWindowStart.IsZero() || now.Sub(c.RestartWindowStart) >= window {
		c.RestartWindowStart = now
		c.RestartAttempts = 0
	}
	if c.RestartAttempts >= c.restartMaxAttempts() {
		c.RestartBlocked = true
		c.dataMu.Unlock()
		_ = c.Save()
		return false
	}
	c.RestartAttempts++
	c.dataMu.Unlock()
	if err := c.Save(); err != nil {
		return false
	}
	return true
}

func (c *Container) restartBlocked() bool {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return c.RestartBlocked
}

func (c *Container) markStableIfNeeded(now time.Time) {
	c.dataMu.RLock()
	started := c.LastStartedAt
	c.dataMu.RUnlock()
	if !started.IsZero() && now.Sub(started) >= stableRuntimeWindow {
		c.resetRestartGuard(now)
	}
}
