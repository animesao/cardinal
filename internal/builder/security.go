package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func safeBuildSource(contextDir, source string) (string, error) {
	if source == "" || filepath.IsAbs(filepath.FromSlash(source)) || strings.HasPrefix(source, "/") || strings.HasPrefix(source, "\\") {
		return "", fmt.Errorf("COPY source must stay inside the build context: %q", source)
	}
	for _, part := range strings.FieldsFunc(source, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("COPY source contains parent traversal: %q", source)
		}
	}
	root, err := filepath.Abs(contextDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(source))
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("COPY source escapes build context: %q", source)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve COPY source %q: %w", source, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("COPY source symlink escapes build context: %q", source)
	}
	return resolved, nil
}

func safeBuildDestination(rootfs, destination string) (string, error) {
	if destination == "" || strings.Contains(destination, "\\") {
		return "", fmt.Errorf("invalid COPY destination %q", destination)
	}
	rel := strings.TrimPrefix(destination, "/")
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", fmt.Errorf("COPY destination contains parent traversal: %q", destination)
		}
	}
	root, err := filepath.Abs(rootfs)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	check, err := filepath.Rel(root, path)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("COPY destination escapes rootfs: %q", destination)
	}
	if err := ensureDestinationWithinRoot(root, path); err != nil {
		return "", fmt.Errorf("unsafe COPY destination %q: %w", destination, err)
	}
	return path, nil
}

func ensureDestinationWithinRoot(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return resolveErr
			}
			resolved, resolveErr = filepath.Abs(resolved)
			if resolveErr != nil {
				return resolveErr
			}
			resolvedRel, resolveErr := filepath.Rel(root, resolved)
			if resolveErr != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("symlink ancestor escapes root: %q", current)
			}
		}
	}
	return nil
}

func buildEnvironment(buildEnv, imageEnv []string) []string {
	// Never inherit arbitrary host credentials/proxy settings into RUN. The
	// Dockerfile and explicit --build-arg/image ENV values remain available.
	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"TERM=xterm",
	}
	env = append(env, buildEnv...)
	env = append(env, imageEnv...)
	return env
}

func ensureBuildDirs(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}
