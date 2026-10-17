//go:build linux

package container

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoopDevicesAvailableIn(t *testing.T) {
	devDir := t.TempDir()
	if loopDevicesAvailableIn(devDir) {
		t.Fatalf("empty dev dir reported loop devices")
	}
	if err := os.WriteFile(filepath.Join(devDir, "loop3"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !loopDevicesAvailableIn(devDir) {
		t.Fatalf("pre-created loop3 was not detected")
	}
	if err := os.WriteFile(filepath.Join(devDir, "loop-control"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !loopDevicesAvailableIn(devDir) {
		t.Fatalf("loop-control was not detected")
	}
}

func TestDiskLimitDegradable(t *testing.T) {
	other := fmt.Errorf("mount disk: mkfs.ext4 failed: %w", errors.New("exit status 1"))
	if diskLimitDegradable(other) {
		t.Fatalf("non-loop failure must stay fatal")
	}

	t.Setenv("CARDINAL_STRICT_DISK_LIMIT", "")
	loopErr := errors.Join(ErrLoopUnavailable, errors.New("mount: exit status 32"))
	if !diskLimitDegradable(loopErr) {
		t.Fatalf("loop failure should degrade without strict mode")
	}

	t.Setenv("CARDINAL_STRICT_DISK_LIMIT", "1")
	if diskLimitDegradable(loopErr) {
		t.Fatalf("strict mode must fail closed")
	}
}
