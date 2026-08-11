//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dck/internal/image"
	"dck/internal/log"
	"dck/internal/state"
)

func SystemPrune() error {
	containers, err := List(true)
	if err != nil {
		return err
	}

	var removedContainers int
	for _, c := range containers {
		if c.Status != Running {
			if err := c.Remove(false); err != nil {
				log.Error("Error removing container %s: %v", shortID(c.ID, 12), err)
				continue
			}
			removedContainers++
		}
	}

	images, err := image.ListImages()
	if err != nil {
		return err
	}

	usedImages := make(map[string]bool)
	for _, c := range containers {
		key := c.ImageName + ":" + c.ImageTag
		usedImages[key] = true
	}

	var removedImages int
	for _, img := range images {
		key := img.Name + ":" + img.Tag
		if usedImages[key] {
			continue
		}
		if err := image.RemoveImage(img.Name, img.Tag); err != nil {
			log.Error("Error removing image %s:%s: %v", img.Name, img.Tag, err)
			continue
		}
		removedImages++
	}

	entries, err := os.ReadDir(state.OverlayDir())
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			id := e.Name()
			path := filepath.Join(state.ContainersDir(), id+".json")
			if !state.FileExists(path) {
				_ = os.RemoveAll(filepath.Join(state.OverlayDir(), id))
			}
		}
	}

	pruneEmptyLogs()

	if removedContainers == 0 && removedImages == 0 {
		fmt.Println("Nothing to prune")
	} else {
		if removedContainers > 0 {
			fmt.Printf("Removed %d container(s)\n", removedContainers)
		}
		if removedImages > 0 {
			fmt.Printf("Removed %d image(s)\n", removedImages)
		}
	}

	return nil
}

func pruneEmptyLogs() {
	entries, _ := os.ReadDir(state.LogsDir())
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".log")
		if !state.FileExists(filepath.Join(state.ContainersDir(), id+".json")) {
			os.Remove(filepath.Join(state.LogsDir(), e.Name()))
		}
	}
}
