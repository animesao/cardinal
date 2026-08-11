//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"

	"dck/internal/state"
)

func (c *Container) Remove(force bool) error {
	c.dataMu.RLock()
	isRunning := c.Status == Running
	c.dataMu.RUnlock()
	if isRunning {
		if !force {
			return fmt.Errorf("cannot remove running container %s (use -f)", c.ID)
		}
		if err := c.Stop(); err != nil {
			return err
		}
	}

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
