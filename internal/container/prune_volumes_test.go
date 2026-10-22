//go:build linux

package container

import (
	"os"
	"testing"
)

// TestPruneUnusedVolumesRemovesOrphanKeepsUsed checks that the prune pass
// drops volumes no container references any more and keeps the ones that are
// still mounted. Volumes outlived their containers (removing a container does
// not delete its volumes), so on a node whose containers were all removed the
// volumes kept filling the disk and `cardinal system prune` freed nothing.
func TestPruneUnusedVolumesRemovesOrphanKeepsUsed(t *testing.T) {
	orig := os.Getenv("CARDINAL_DATA_DIR")
	defer os.Setenv("CARDINAL_DATA_DIR", orig)
	os.Setenv("CARDINAL_DATA_DIR", t.TempDir())

	if _, err := CreateVolume("orphan-vol", "local", nil, nil); err != nil {
		t.Fatalf("CreateVolume(orphan-vol): %v", err)
	}
	if _, err := CreateVolume("used-vol", "local", nil, nil); err != nil {
		t.Fatalf("CreateVolume(used-vol): %v", err)
	}

	// A container still holds used-vol.
	c := &Container{
		ID:      "prunetest01",
		Status:  Stopped,
		Volumes: []VolumeMount{{Source: "used-vol", Target: "/data"}},
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save container: %v", err)
	}

	pruned := pruneUnusedVolumes()
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1 (only the orphan)", pruned)
	}
	if _, err := os.Stat(VolumeDir("orphan-vol")); !os.IsNotExist(err) {
		t.Error("orphan volume survived the prune")
	}
	if _, err := os.Stat(VolumeDir("used-vol")); err != nil {
		t.Errorf("volume in use was pruned: %v", err)
	}
}

// TestPruneUnusedVolumesWithoutContainers checks the case from the field: the
// node has no containers at all, so every volume is an orphan.
func TestPruneUnusedVolumesWithoutContainers(t *testing.T) {
	orig := os.Getenv("CARDINAL_DATA_DIR")
	defer os.Setenv("CARDINAL_DATA_DIR", orig)
	os.Setenv("CARDINAL_DATA_DIR", t.TempDir())

	for _, name := range []string{"vol-a", "vol-b", "vol-c"} {
		if _, err := CreateVolume(name, "local", nil, nil); err != nil {
			t.Fatalf("CreateVolume(%s): %v", name, err)
		}
	}

	if pruned := pruneUnusedVolumes(); pruned != 3 {
		t.Fatalf("pruned = %d, want 3", pruned)
	}
	volumes, err := ListVolumes()
	if err != nil {
		t.Fatalf("ListVolumes: %v", err)
	}
	if len(volumes) != 0 {
		t.Fatalf("volumes left after prune: %d", len(volumes))
	}
}
