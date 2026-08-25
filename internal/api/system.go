//go:build linux

package api

import (
	"net/http"
	"os"
	"path/filepath"

	"cardinal/internal/container"
	"cardinal/internal/state"
)

func handleSystemPrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	pruneContainers := r.URL.Query().Get("containers") != "false"
	pruneImages := r.URL.Query().Get("images") != "false"

	var freedSpace int64
	var removedContainers, removedImages int

	// Collect IDs of running containers to protect their overlay dirs
	runningIDs := make(map[string]bool)
	all, _ := container.List(true)
	for _, c := range all {
		if c.Status == container.Running {
			runningIDs[c.ID] = true
		}
	}

	if pruneContainers {
		entries, _ := os.ReadDir(state.ContainersDir())
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" {
				continue
			}
			path := filepath.Join(state.ContainersDir(), e.Name())
			info, _ := os.Stat(path)
			if info != nil {
				freedSpace += info.Size()
			}
			os.Remove(path)
			removedContainers++
		}
	}

	// Remove unused overlay dirs (skip running containers)
	overlayEntries, _ := os.ReadDir(state.OverlayDir())
	for _, e := range overlayEntries {
		if !e.IsDir() {
			continue
		}
		if runningIDs[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(state.OverlayDir(), e.Name())); err != nil {
			continue
		}
	}

	if pruneImages {
		images, err := listAllImageDirs()
		if err == nil {
			for _, imgDir := range images {
				if err := os.RemoveAll(imgDir); err != nil {
					continue
				}
				removedImages++
			}
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"ContainersDeleted": removedContainers,
		"ImagesDeleted":     removedImages,
		"SpaceReclaimed":    freedSpace,
	})
}

func listAllImageDirs() ([]string, error) {
	var dirs []string
	imagesDir := state.ImagesDir()
	namespaces, err := os.ReadDir(imagesDir)
	if err != nil {
		return nil, err
	}
	for _, ns := range namespaces {
		if !ns.IsDir() {
			continue
		}
		repos, _ := os.ReadDir(filepath.Join(imagesDir, ns.Name()))
		for _, repo := range repos {
			if !repo.IsDir() {
				continue
			}
			tags, _ := os.ReadDir(filepath.Join(imagesDir, ns.Name(), repo.Name()))
			for _, tag := range tags {
				if tag.IsDir() {
					dirs = append(dirs, filepath.Join(imagesDir, ns.Name(), repo.Name(), tag.Name()))
				}
			}
		}
	}
	return dirs, nil
}
