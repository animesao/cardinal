//go:build linux

package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dck/internal/state"
)

// VolumeType represents the type of volume mount
type VolumeType string

const (
	VolumeTypeBind   VolumeType = "bind"
	VolumeTypeTmpfs  VolumeType = "tmpfs"
	VolumeTypeNFS    VolumeType = "nfs"
	VolumeTypeVolume VolumeType = "volume"
)

// VolumeSpec is a parsed volume specification
type VolumeSpec struct {
	Type           VolumeType `json:"type"`
	Source         string     `json:"source"`
	Target         string     `json:"target"`
	ReadOnly       bool       `json:"read_only,omitempty"`
	Propagation    string     `json:"propagation,omitempty"`     // shared, slave, private, rshared, rslave, rprivate
	SELinuxRelabel string     `json:"selinux_relabel,omitempty"` // Z or z
	NoCopy         bool       `json:"no_copy,omitempty"`
	// NFS options
	NFOptions string `json:"nfs_options,omitempty"`
	// tmpfs options
	TmpfsSize string      `json:"tmpfs_size,omitempty"`
	TmpfsMode os.FileMode `json:"tmpfs_mode,omitempty"`
}

// Volume is a named volume stored in the state directory
type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"` // local, nfs
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  time.Time         `json:"created_at"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	// NFS options
	NFSAddress string `json:"nfs_address,omitempty"`
	NFSPath    string `json:"nfs_path,omitempty"`
}

// ParseVolumeSpec parses a volume string like "src:dst:ro,shared"
// or "tmpfs:/path:size=1G" or "nfs://server:/path:/container/path:ro"
func ParseVolumeSpec(spec string) (*VolumeSpec, error) {
	// Check for tmpfs
	if strings.HasPrefix(spec, "tmpfs:") {
		parts := strings.SplitN(spec, ":", 2)
		targetAndOpts := parts[1]
		target, opts := splitOpts(targetAndOpts)
		vs := &VolumeSpec{
			Type:   VolumeTypeTmpfs,
			Target: target,
		}
		applyOptions(vs, opts)
		return vs, nil
	}

	// Check for NFS
	if strings.HasPrefix(spec, "nfs://") || strings.HasPrefix(spec, "nfs:") {
		rest := strings.TrimPrefix(spec, "nfs://")
		rest = strings.TrimPrefix(rest, "nfs:")
		// Format: server:/path:/container/path:options
		parts := strings.SplitN(rest, ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid NFS format: %s (expected nfs://server:/export:/container/path)", spec)
		}
		serverAndPath := parts[0] + ":" + parts[1]
		targetAndOpts := parts[2]
		target, opts := splitOpts(targetAndOpts)
		vs := &VolumeSpec{
			Type:      VolumeTypeNFS,
			Source:    serverAndPath,
			Target:    target,
			NFOptions: "hard,intr,rsize=1048576,wsize=1048576",
		}
		applyOptions(vs, opts)
		return vs, nil
	}

	// Bind mount or named volume
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) == 1 {
		// Just a target — anonymous tmpfs
		return &VolumeSpec{Type: VolumeTypeTmpfs, Target: parts[0]}, nil
	}

	vs := &VolumeSpec{
		Type:   VolumeTypeBind,
		Source: parts[0],
		Target: parts[1],
	}

	if len(parts) == 3 {
		applyOptions(vs, parts[2])
	}

	// Detect named volume (no path separators in source)
	if vs.Type == VolumeTypeBind && !strings.Contains(vs.Source, "/") && !strings.Contains(vs.Source, "\\") {
		vs.Type = VolumeTypeVolume
	}

	return vs, nil
}

func splitOpts(s string) (string, string) {
	// Split on last colon? No, target is first, opts are after last colon.
	// But target can contain slash, opts cannot.
	// Simplest: first part is target (everything up to last colon if the last part looks like opts)
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return s, ""
	}
	optPart := s[idx+1:]
	targetPart := s[:idx]

	// Check if optPart looks like options (no slashes, contains commas or known keywords)
	if strings.Contains(optPart, "/") || strings.Contains(optPart, "\\") {
		return s, "" // no options, the whole string is the target
	}
	return targetPart, optPart
}

func applyOptions(vs *VolumeSpec, opts string) {
	for _, opt := range strings.Split(opts, ",") {
		opt = strings.TrimSpace(opt)
		switch {
		case opt == "ro":
			vs.ReadOnly = true
		case opt == "rw":
			vs.ReadOnly = false
		case opt == "shared", opt == "rshared":
			vs.Propagation = opt
		case opt == "slave", opt == "rslave":
			vs.Propagation = opt
		case opt == "private", opt == "rprivate":
			vs.Propagation = opt
		case opt == "Z" || opt == "z":
			vs.SELinuxRelabel = opt
		case opt == "nocopy":
			vs.NoCopy = true
		case strings.HasPrefix(opt, "size="):
			vs.TmpfsSize = opt[5:]
		case strings.HasPrefix(opt, "mode="):
			_, _ = fmt.Sscanf(opt[5:], "%o", &vs.TmpfsMode)
		case strings.HasPrefix(opt, "nfsopts="):
			vs.NFOptions = opt[8:]
		}
	}
}

// VolumeDir returns the path to a named volume's data directory
func VolumeDir(name string) string {
	return filepath.Join(state.VolumesDir(), name)
}

func validateVolumeName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\\") {
		return fmt.Errorf("invalid volume name %q", name)
	}
	return nil
}

func validateContainerTarget(target string) error {
	if target == "" || strings.Contains(target, "\\") || !filepath.IsAbs(filepath.FromSlash(target)) {
		return fmt.Errorf("container mount target must be absolute: %q", target)
	}
	for _, part := range strings.FieldsFunc(target, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return fmt.Errorf("container mount target contains parent traversal: %q", target)
		}
	}
	clean := filepath.Clean(filepath.FromSlash(target))
	if clean == string(filepath.Separator) || clean == "." {
		return fmt.Errorf("unsafe container mount target: %q", target)
	}
	rel, err := filepath.Rel(string(filepath.Separator), clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("container mount target escapes root: %q", target)
	}
	return nil
}

func validateBindSource(source string) (string, error) {
	if source == "" || !filepath.IsAbs(filepath.FromSlash(source)) {
		return "", fmt.Errorf("bind source must be an absolute host path: %q", source)
	}
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", fmt.Errorf("resolve bind source %q: %w", source, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("absolute bind source %q: %w", source, err)
	}
	for _, blocked := range []string{
		"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64",
		"/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin", "/srv",
		"/sys", "/usr", "/var", "/var/run",
	} {
		if resolved == blocked || strings.HasPrefix(resolved, blocked+string(filepath.Separator)) {
			return "", fmt.Errorf("bind source %q is a protected host path", source)
		}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat bind source %q: %w", source, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("bind source %q is not a directory", source)
	}
	return resolved, nil
}

// CreateVolume creates a new named volume
func CreateVolume(name, driver string, labels map[string]string, opts map[string]string) (*Volume, error) {
	if err := validateVolumeName(name); err != nil {
		return nil, err
	}
	volDir := VolumeDir(name)
	if _, err := os.Stat(volDir); err == nil {
		return nil, fmt.Errorf("volume %q already exists", name)
	}

	if err := os.MkdirAll(volDir, 0700); err != nil {
		return nil, fmt.Errorf("create volume directory: %w", err)
	}

	vol := &Volume{
		Name:       name,
		Driver:     driver,
		Mountpoint: volDir,
		CreatedAt:  time.Now(),
		Labels:     labels,
		Options:    opts,
	}

	if err := saveVolume(vol); err != nil {
		_ = os.RemoveAll(volDir)
		return nil, err
	}

	return vol, nil
}

// ListVolumes returns all named volumes
func ListVolumes() ([]*Volume, error) {
	entries, err := os.ReadDir(state.VolumesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var volumes []*Volume
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		vol := loadVolume(e.Name())
		if vol != nil {
			volumes = append(volumes, vol)
		}
	}
	return volumes, nil
}

// RemoveVolume removes a named volume
func RemoveVolume(name string) error {
	if err := validateVolumeName(name); err != nil {
		return err
	}
	volDir := VolumeDir(name)
	if _, err := os.Stat(volDir); os.IsNotExist(err) {
		return fmt.Errorf("volume %q not found", name)
	}

	// Check if any container is using this volume
	containers, _ := List(true)
	for _, c := range containers {
		for _, v := range c.Volumes {
			resolved := state.ResolveVolume(v.Source)
			if resolved == volDir {
				return fmt.Errorf("volume %q is in use by container %s", name, shortID(c.ID, 12))
			}
		}
	}

	return os.RemoveAll(volDir)
}

// InspectVolume returns volume details
func InspectVolume(name string) (*Volume, error) {
	vol := loadVolume(name)
	if vol == nil {
		return nil, fmt.Errorf("volume %q not found", name)
	}
	return vol, nil
}

func loadVolume(name string) *Volume {
	path := filepath.Join(VolumeDir(name), "volume.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var vol Volume
	if err := json.Unmarshal(data, &vol); err != nil {
		return nil
	}
	return &vol
}

func saveVolume(vol *Volume) error {
	path := filepath.Join(VolumeDir(vol.Name), "volume.json")
	data, err := json.Marshal(vol)
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(path, data, 0600)
}

// MountVolume mounts a volume for a container
func MountVolume(spec *VolumeSpec, containerRootfs string) error {
	if spec == nil {
		return fmt.Errorf("volume specification is nil")
	}
	if err := validateContainerTarget(spec.Target); err != nil {
		return err
	}
	root, err := filepath.Abs(containerRootfs)
	if err != nil {
		return fmt.Errorf("resolve container rootfs: %w", err)
	}
	target, err := safeContainerPath(root, spec.Target)
	if err != nil {
		return err
	}
	if err := rejectSymlinkAncestors(root, target); err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	switch spec.Type {
	case VolumeTypeTmpfs:
		return mountTmpfs(spec, target)
	case VolumeTypeNFS:
		return mountNFS(spec, target)
	case VolumeTypeBind, VolumeTypeVolume:
		return mountBind(spec, target)
	default:
		return fmt.Errorf("unknown volume type: %s", spec.Type)
	}
}

func mountTmpfs(spec *VolumeSpec, target string) error {
	opts := "size=" + spec.TmpfsSize
	if spec.TmpfsSize == "" {
		opts = "size=64M"
	}
	if spec.TmpfsMode > 0 {
		opts += fmt.Sprintf(",mode=%o", spec.TmpfsMode)
	}
	if spec.ReadOnly {
		opts += ",ro"
	}
	if err := exec.Command("mount", "-t", "tmpfs", "-o", opts, "tmpfs", target).Run(); err != nil {
		return fmt.Errorf("mount tmpfs: %w", err)
	}
	return nil
}

func mountNFS(spec *VolumeSpec, target string) error {
	// spec.Source is "server:export"
	opts := spec.NFOptions
	if opts == "" {
		opts = "hard,intr,rsize=1048576,wsize=1048576"
	}
	if spec.ReadOnly {
		opts += ",ro"
	}
	if err := exec.Command("mount", "-t", "nfs", "-o", opts, spec.Source, target).Run(); err != nil {
		return fmt.Errorf("mount nfs %s: %w", spec.Source, err)
	}
	return nil
}

func mountBind(spec *VolumeSpec, target string) error {
	source := spec.Source
	if spec.Type == VolumeTypeVolume {
		if err := validateVolumeName(spec.Source); err != nil {
			return err
		}
		source = VolumeDir(spec.Source)
		if err := os.MkdirAll(source, 0700); err != nil {
			return fmt.Errorf("create volume source: %w", err)
		}
	} else {
		var err error
		source, err = validateBindSource(source)
		if err != nil {
			return err
		}
	}

	// Copy image content into empty volumes (Docker-compatible)
	if spec.Type == VolumeTypeVolume && !spec.NoCopy {
		empty, _ := isDirEmpty(target)
		if empty {
			_ = exec.Command("cp", "-a", target+"/.", source+"/").Run()
		}
	}

	// Mount bind. Read-only bind mounts require a second remount operation;
	// applying `ro` only to the initial bind does not reliably make the mount
	// read-only on all supported Linux kernels.
	if err := exec.Command("mount", "--bind", source, target).Run(); err != nil {
		return fmt.Errorf("mount bind %s: %w", source, err)
	}

	// Apply propagation
	if spec.Propagation != "" {
		propOpt := "--make-" + spec.Propagation
		if err := exec.Command("mount", propOpt, target).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: mount %s %s: %v\n", propOpt, target, err)
		}
	}

	// Remount readonly if needed
	if spec.ReadOnly {
		if err := exec.Command("mount", "--bind", "-o", "remount,ro", source, target).Run(); err != nil {
			return fmt.Errorf("remount ro: %w", err)
		}
	}

	// SELinux relabel
	if spec.SELinuxRelabel != "" {
		if err := exec.Command("chcon", "-Rt", "svirt_sandbox_file_t", target).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: chcon %s: %v\n", target, err)
		}
	}

	return nil
}

// UmountVolume unmounts a volume
func UmountVolume(target string) {
	_ = exec.Command("umount", target).Run()
}

func rejectSymlinkAncestors(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("container mount target escapes rootfs: %q", target)
	}
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect mount target %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("mount target contains symlink: %q", current)
		}
	}
	return nil
}

func safeContainerPath(root, target string) (string, error) {
	target = filepath.FromSlash(target)
	rel := strings.TrimPrefix(target, string(filepath.Separator))
	path := filepath.Join(root, rel)
	check, err := filepath.Rel(root, path)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("container mount target escapes rootfs: %q", target)
	}
	return path, nil
}

// VolumeMountFromSpec parses a CLI/Compose volume specification and preserves
// read-only, propagation, and no-copy options in container state.
func VolumeMountFromSpec(value string) (VolumeMount, error) {
	spec, err := ParseVolumeSpec(strings.TrimSpace(value))
	if err != nil {
		return VolumeMount{}, err
	}
	return VolumeMount{
		Type:           spec.Type,
		Source:         spec.Source,
		Target:         spec.Target,
		ReadOnly:       spec.ReadOnly,
		Propagation:    spec.Propagation,
		SELinuxRelabel: spec.SELinuxRelabel,
		NoCopy:         spec.NoCopy,
	}, nil
}

// ParseVolumeString is a convenience for parsing the old-format volume string
// Returns VolumeSpec with backward compatibility
func ParseVolumeString(v string) *VolumeSpec {
	spec, err := ParseVolumeSpec(v)
	if err != nil {
		// Fallback to basic parse
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			return &VolumeSpec{
				Type:   VolumeTypeBind,
				Source: parts[0],
				Target: parts[1],
			}
		}
		return &VolumeSpec{
			Type:   VolumeTypeBind,
			Source: v,
			Target: v,
		}
	}
	return spec
}
