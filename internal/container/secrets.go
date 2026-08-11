//go:build linux

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dck/internal/config"
)

// ResolveSecretSpecs converts config-level secret/config refs into container mount specs
func ResolveSecretSpecs(cfg config.Config) map[string][]SecretMount {
	result := make(map[string][]SecretMount)

	for name, cc := range cfg.Container {
		var mounts []SecretMount

		for _, ref := range cc.Secrets {
			mount := SecretMount{
				Source: ref.Source,
				Target: ref.Target,
				UID:    ref.UID,
				GID:    ref.GID,
				Mode:   ref.Mode,
			}

			// Resolve file content from top-level specs
			if spec, ok := cfg.Secrets[ref.Source]; ok && spec.File != "" {
				data, err := os.ReadFile(spec.File)
				if err == nil {
					mount.Data = string(data)
				}
				if mount.Target == "" {
					mount.Target = "/run/secrets/" + ref.Source
				}
			}

			if mount.Target == "" {
				mount.Target = "/run/secrets/" + ref.Source
			}
			if mount.Mode == 0 {
				mount.Mode = 0444
			}

			mounts = append(mounts, mount)
		}

		for _, ref := range cc.Configs {
			mount := SecretMount{
				Source: ref.Source,
				Target: ref.Target,
				UID:    ref.UID,
				GID:    ref.GID,
				Mode:   ref.Mode,
			}

			if spec, ok := cfg.Configs[ref.Source]; ok && spec.File != "" {
				data, err := os.ReadFile(spec.File)
				if err == nil {
					mount.Data = string(data)
				}
				if mount.Target == "" {
					mount.Target = "/" + ref.Source
				}
			}

			if mount.Target == "" {
				mount.Target = "/" + ref.Source
			}
			if mount.Mode == 0 {
				mount.Mode = 0444
			}

			mounts = append(mounts, mount)
		}

		result[name] = mounts
	}

	return result
}

// InjectSecrets writes secret/config files into the container rootfs
func (c *Container) InjectSecrets(rootfs string) error {
	allMounts := append(c.Secrets, c.Configs...)

	for _, sm := range allMounts {
		if sm.Data == "" {
			continue
		}

		targetPath, err := safeContainerFilePath(rootfs, sm.Target)
		if err != nil {
			return fmt.Errorf("unsafe secret target %q: %w", sm.Target, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("create secret dir %s: %w", filepath.Dir(targetPath), err)
		}
		if err := rejectSecretSymlinkAncestors(rootfs, targetPath); err != nil {
			return fmt.Errorf("unsafe secret target %q: %w", sm.Target, err)
		}

		mode := os.FileMode(sm.Mode)
		if mode == 0 {
			mode = 0444
		}

		if info, statErr := os.Lstat(targetPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("write secret %s: target is a symlink", sm.Target)
		}
		if err := os.WriteFile(targetPath, []byte(sm.Data), mode); err != nil {
			return fmt.Errorf("write secret %s: %w", sm.Target, err)
		}

		// Chown if uid/gid specified
		if sm.UID != "" || sm.GID != "" {
			uid := -1
			gid := -1
			if sm.UID != "" {
				if u, err := strconv.Atoi(sm.UID); err == nil {
					uid = u
				}
			}
			if sm.GID != "" {
				if g, err := strconv.Atoi(sm.GID); err == nil {
					gid = g
				}
			}
			if uid >= 0 || gid >= 0 {
				if err := os.Chown(targetPath, uid, gid); err != nil {
					return fmt.Errorf("chown secret %s: %w", sm.Target, err)
				}
			}
		}
	}

	return nil
}

func safeContainerFilePath(rootfs, target string) (string, error) {
	if target == "" || strings.Contains(target, "\\") || !filepath.IsAbs(filepath.FromSlash(target)) {
		return "", fmt.Errorf("target must be an absolute container path")
	}
	rel := strings.TrimPrefix(filepath.FromSlash(target), string(filepath.Separator))
	if rel == "" || rel == "." {
		return "", fmt.Errorf("target must name a file")
	}
	for _, part := range strings.FieldsFunc(target, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("parent traversal")
		}
	}
	root, err := filepath.Abs(rootfs)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, rel)
	check, err := filepath.Rel(root, path)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes container rootfs")
	}
	return path, nil
}

func rejectSecretSymlinkAncestors(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes container rootfs")
	}
	current := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		if i == len(parts)-1 {
			break
		}
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink ancestor %q", current)
		}
	}
	return nil
}

// ParseComposeSecretsToMounts converts a container config's secrets/configs into VolumeMount specs
func ParseSecretsToVolumes(cc config.ContainerConfig, cfg config.Config) ([]VolumeMount, []SecretMount) {
	var volMounts []VolumeMount
	var secretMounts []SecretMount

	for _, ref := range cc.Secrets {
		target := ref.Target
		if target == "" {
			target = "/run/secrets/" + ref.Source
		}

		spec, hasSpec := cfg.Secrets[ref.Source]
		if hasSpec && spec.File != "" {
			// Bind mount the file directly
			volMounts = append(volMounts, VolumeMount{
				Source: spec.File,
				Target: target,
			})
		} else if !hasSpec && strings.HasPrefix(ref.Source, "/") {
			// Treat as absolute path
			volMounts = append(volMounts, VolumeMount{
				Source: ref.Source,
				Target: target,
			})
		} else {
			// Will be injected via InjectSecrets (data from compose top-level)
			sm := SecretMount{
				Source: ref.Source,
				Target: target,
				UID:    ref.UID,
				GID:    ref.GID,
				Mode:   ref.Mode,
			}
			if hasSpec && spec.File != "" {
				data, err := os.ReadFile(spec.File)
				if err == nil {
					sm.Data = string(data)
				}
			}
			secretMounts = append(secretMounts, sm)
		}
	}

	for _, ref := range cc.Configs {
		target := ref.Target
		if target == "" {
			target = "/" + ref.Source
		}

		spec, hasSpec := cfg.Configs[ref.Source]
		if hasSpec && spec.File != "" {
			volMounts = append(volMounts, VolumeMount{
				Source: spec.File,
				Target: target,
			})
		} else if !hasSpec && strings.HasPrefix(ref.Source, "/") {
			volMounts = append(volMounts, VolumeMount{
				Source: ref.Source,
				Target: target,
			})
		} else {
			sm := SecretMount{
				Source: ref.Source,
				Target: target,
				UID:    ref.UID,
				GID:    ref.GID,
				Mode:   ref.Mode,
			}
			if hasSpec && spec.File != "" {
				data, err := os.ReadFile(spec.File)
				if err == nil {
					sm.Data = string(data)
				}
			}
			secretMounts = append(secretMounts, sm)
		}
	}

	return volMounts, secretMounts
}

// ResolveAndSetSecrets resolves and injects secrets/configs into the container
func (c *Container) ResolveAndSetSecrets(cc config.ContainerConfig, cfg config.Config) error {
	volMounts, secretMounts := ParseSecretsToVolumes(cc, cfg)

	// Add volume mounts for file-backed secrets
	c.Volumes = append(c.Volumes, volMounts...)
	c.Secrets = secretMounts

	return nil
}
