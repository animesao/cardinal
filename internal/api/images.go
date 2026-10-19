//go:build linux

package api

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cardinal/internal/image"
	"cardinal/internal/state"
)

func handleImagesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	allImages, err := image.ListImages()
	if err != nil {
		writeError(w, 500, fmt.Sprintf("list images: %v", err))
		return
	}

	result := make([]ImageSummary, 0, len(allImages))
	for _, img := range allImages {
		shortName := strings.TrimPrefix(img.Name, "library/")
		tag := img.Tag
		if tag == "" {
			tag = "latest"
		}
		result = append(result, ImageSummary{
			ID:       "sha256:" + img.Digest,
			RepoTags: []string{fmt.Sprintf("%s:%s", shortName, tag)},
			Created:  time.Now().Unix(),
			Size:     0,
		})
	}

	writeJSON(w, 200, result)
}

func handleImagesRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/images/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		writeError(w, 400, "missing image name")
		return
	}

	name := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "json":
		handleImageInspect(w, r, name)
	case "remove":
		handleImageRemove(w, r, name)
	case "push":
		handleImagePush(w, r, name)
	case "tag":
		handleImageTag(w, r, name)
	case "history":
		handleImageHistory(w, r, name)
	case "get":
		handleImageGet(w, r, name)
	case "pull":
		handleImagePull(w, r, name)
	default:
		if action == "" && r.Method == "DELETE" {
			handleImageRemove(w, r, name)
		} else if action == "" && r.Method == "GET" {
			handleImageInspect(w, r, name)
		} else {
			writeError(w, 404, fmt.Sprintf("unknown action: %s", action))
		}
	}
}

func parseImageRef(ref string) (name, tag string) {
	tag = "latest"
	if i := strings.LastIndex(ref, ":"); i > 0 {
		tag = ref[i+1:]
		ref = ref[:i]
	}
	if !strings.Contains(ref, "/") {
		ref = "library/" + ref
	}
	return ref, tag
}

func handleImageInspect(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	name, tag := parseImageRef(ref)
	img := image.LoadFromStore(name, tag)
	if img == nil {
		writeError(w, 404, fmt.Sprintf("image %s:%s not found", name, tag))
		return
	}

	shortName := strings.TrimPrefix(name, "library/")
	rootfsDir := state.ImageRootfsDir(name, tag)
	rootfsSize := dirSize(rootfsDir)

	inspect := ImageInspect{
		ID:           "sha256:" + img.Digest,
		RepoTags:     []string{fmt.Sprintf("%s:%s", shortName, tag)},
		Created:      time.Now().UTC().Format(time.RFC3339),
		Size:         rootfsSize,
		Architecture: "amd64",
		OS:           "linux",
		RootFS: &ImageRootFS{
			Type:   "layers",
			Layers: []string{},
		},
	}

	// Try to read config for more details
	cfg, err := image.ReadConfig(name, tag)
	if err == nil && cfg != nil {
		inspect.Config = &ContainerConfig{
			Image:      fmt.Sprintf("%s:%s", shortName, tag),
			Cmd:        cfg.Config.Cmd,
			Env:        cfg.Config.Env,
			WorkingDir: cfg.Config.WorkingDir,
			User:       cfg.Config.User,
		}
	}

	// Read layers from manifest
	m := image.ReadManifest(name, tag)
	if m != nil {
		for _, layer := range m.Layers {
			inspect.RootFS.Layers = append(inspect.RootFS.Layers, layer.Digest)
		}
	}

	writeJSON(w, 200, inspect)
}

func handleImageRemove(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "DELETE" {
		writeError(w, 405, "method not allowed")
		return
	}

	name, tag := parseImageRef(ref)
	if err := image.RemoveImage(name, tag); err != nil {
		writeError(w, 500, fmt.Sprintf("remove image: %v", err))
		return
	}

	writeJSON(w, 200, OKResponse{Message: "removed"})
}

func handleImagePush(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	// Try to read credentials from body
	var authMap map[string]string
	authHeader := r.Header.Get("X-Registry-Auth")
	if authHeader != "" {
		if err := json.Unmarshal([]byte(authHeader), &authMap); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid registry auth: %v", err))
			return
		}
	}

	username := authMap["username"]
	password := authMap["password"]

	if err := image.Push(ref, username, password); err != nil {
		// Текст ошибки может содержать ответ реестра, а тот — эхо
		// переданных кредов. Отдаём только сводку, без тела ответа
		// реестра.
		writeError(w, http.StatusBadGateway, fmt.Sprintf("push %s failed: %s", ref, redactRegistryError(err)))
		return
	}

	writeJSON(w, http.StatusOK, OKResponse{Message: "pushed " + ref})
}

// redactRegistryError убирает из текста ошибки возможные креды.
func redactRegistryError(err error) string {
	msg := err.Error()
	for _, marker := range []string{"password", "passwd", "token", "secret", "authorization"} {
		low := strings.ToLower(msg)
		for i := 0; ; {
			j := strings.Index(low[i:], marker)
			if j < 0 {
				break
			}
			start := i + j
			// Значение после маркера (до запятой/пробела/двоеточия).
			valStart := start + len(marker)
			for valStart < len(msg) && (msg[valStart] == '=' || msg[valStart] == ':' || msg[valStart] == ' ') {
				valStart++
			}
			valEnd := valStart
			for valEnd < len(msg) && msg[valEnd] != ',' && msg[valEnd] != ' ' && msg[valEnd] != ';' && msg[valEnd] != '"' {
				valEnd++
			}
			if valEnd > valStart {
				msg = msg[:valStart] + "[REDACTED]" + msg[valEnd:]
				low = strings.ToLower(msg)
				i = valStart + len("[REDACTED]")
				continue
			}
			i = start + len(marker)
		}
	}
	return msg
}

func handleImageTag(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	repo := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	if repo == "" {
		writeError(w, 400, "repo parameter required")
		return
	}
	if tag == "" {
		tag = "latest"
	}

	srcName, srcTag := parseImageRef(ref)
	img := image.LoadFromStore(srcName, srcTag)
	if img == nil {
		writeError(w, 404, fmt.Sprintf("image %s:%s not found", srcName, srcTag))
		return
	}

	// Copy the image metadata
	destName := repo
	if !strings.Contains(destName, "/") {
		destName = "library/" + destName
	}

	// Create new image reference (copy on disk by creating new manifest)
	img2 := &image.Image{Name: destName, Tag: tag, Digest: img.Digest}
	if err := image.SaveToStore(img2); err != nil {
		writeError(w, 500, fmt.Sprintf("tag: %v", err))
		return
	}

	writeJSON(w, 201, OKResponse{Message: "tagged"})
}

func handleImageHistory(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}

	name, tag := parseImageRef(ref)
	img := image.LoadFromStore(name, tag)
	if img == nil {
		writeError(w, 404, fmt.Sprintf("image %s:%s not found", name, tag))
		return
	}

	entries := []ImageHistoryEntry{
		{
			ID:        "sha256:" + img.Digest,
			Created:   time.Now().Unix(),
			CreatedBy: "cardinal build",
			Size:      0,
			Tags:      []string{fmt.Sprintf("%s:%s", name, tag)},
		},
	}

	writeJSON(w, 200, entries)
}

func handleImageGet(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	name, tag := parseImageRef(ref)
	img := image.LoadFromStore(name, tag)
	if img == nil {
		writeError(w, 404, fmt.Sprintf("image %s:%s not found", name, tag))
		return
	}
	imageDir := state.ImageDir(name, tag)
	info, err := os.Stat(imageDir)
	if err != nil || !info.IsDir() {
		writeError(w, 404, fmt.Sprintf("image %s:%s not found", name, tag))
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.WriteHeader(200)
	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()
	_ = filepath.Walk(imageDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(imageDir, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return nil
		}
		header.Name = rel
		if fi.Mode()&os.ModeSymlink != 0 {
			link, readErr := os.Readlink(path)
			if readErr != nil {
				return nil
			}
			header.Linkname = filepath.ToSlash(link)
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			in, openErr := os.Open(path)
			if openErr != nil {
				return nil
			}
			_, copyErr := io.Copy(tw, in)
			_ = in.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return nil
	})
}

func handleImagePull(w http.ResponseWriter, r *http.Request, ref string) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	img, err := image.Pull(ref)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("pull %s: %v", ref, err))
		return
	}

	writeJSON(w, 200, OKResponse{
		Message: fmt.Sprintf("pulled %s:%s", strings.TrimPrefix(img.Name, "library/"), img.Tag),
	})
}

// ListAllImages returns all images for system info
func ListAllImages() ([]ImageSummary, error) {
	allImages, err := image.ListImages()
	if err != nil {
		return nil, err
	}
	result := make([]ImageSummary, len(allImages))
	for i, img := range allImages {
		shortName := strings.TrimPrefix(img.Name, "library/")
		result[i] = ImageSummary{
			ID:       "sha256:" + img.Digest,
			RepoTags: []string{fmt.Sprintf("%s:%s", shortName, img.Tag)},
			Created:  time.Now().Unix(),
		}
	}
	return result, nil
}

func dirSize(path string) int64 {
	var size int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		size += info.Size()
	}
	return size
}
