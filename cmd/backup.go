//go:build linux

package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"cardinal/internal/container"
	"cardinal/internal/state"
)

func Backup(args []string) {
	if len(args) < 1 {
		printBackupUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		backupCreate(args[1:])
	case "list", "ls":
		backupList(args[1:])
	case "restore":
		backupRestore(args[1:])
	case "enable":
		backupEnable(args[1:])
	case "disable":
		backupDisable(args[1:])
	case "status":
		backupStatus(args[1:])
	case "verify":
		backupVerify(args[1:])
	case "remove", "rm", "delete":
		backupRemove(args[1:])
	case "generate-key":
		backupGenerateKey(args[1:])
	default:
		fmt.Printf("unknown backup command: %s\n", args[0])
		printBackupUsage()
		os.Exit(1)
	}
}

func printBackupUsage() {
	fmt.Println(`Usage: cardinal backup COMMAND

Commands:
  create <container> [-o file.tar.gz] [-e]  Archive container writable data and metadata
  list                                      List backups in the cardinal backup directory
  restore <container> <file.tar.gz>         Restore writable data into a stopped container
  restore <container> <file.tar.gz> --rebind Restore data into a newly created container
  enable <container> [options]              Enable scheduled backups
  disable <container>                       Disable scheduled backups
  status <container>                        Show scheduled backup settings
  verify <file.tar.gz>                      Verify archive checksum
  remove <file.tar.gz>                      Delete an archive and its checksum
  generate-key                              Generate a new encryption key

Create options:
  -o <file>                                Output file path
  -e, --encrypt                            Encrypt the backup archive (requires CARDINAL_BACKUP_KEY)

Enable options:
  --interval <duration>                    Backup interval (default: 24h)
  --retention <n>                          Number of archives to keep (default: 7)
  --dir <path>                             Backup directory (default: cardinal data/backups/<container>)
  --encrypt                                Enable encryption for automatic backups`)
}

func backupDir() string { return filepath.Join(state.DataDir(), "backups") }

const (
	defaultBackupInterval  = 24 * time.Hour
	defaultBackupRetention = 7
)

func containerBackupDir(c *container.Container) string {
	if c != nil && c.BackupDir != "" {
		return c.BackupDir
	}
	if c == nil {
		return backupDir()
	}
	return filepath.Join(backupDir(), c.Name)
}

func backupEnable(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup enable <container> [--interval 24h] [--retention 7] [--dir /data/backups]")
		os.Exit(1)
	}
	fs := flag.NewFlagSet("backup enable", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	interval := fs.Duration("interval", defaultBackupInterval, "Backup interval")
	retention := fs.Int("retention", defaultBackupRetention, "Number of backups to keep")
	dir := fs.String("dir", "", "Backup directory")
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "Error: backup interval must be greater than zero")
		os.Exit(1)
	}
	if *retention < 1 || *retention > 1000 {
		fmt.Fprintln(os.Stderr, "Error: backup retention must be between 1 and 1000")
		os.Exit(1)
	}
	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	backupPath := *dir
	if backupPath == "" {
		backupPath = containerBackupDir(c)
	}
	backupPath, err = validateBackupDirectory(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid backup directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(backupPath, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating backup directory: %v\n", err)
		os.Exit(1)
	}
	c.AutoBackup = true
	c.BackupInterval = interval.String()
	c.BackupRetention = *retention
	c.BackupDir = backupPath
	c.LastBackupAt = time.Now()
	c.BackupNextAttemptAt = time.Time{}
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving backup settings: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Automatic backups enabled for %s: every %s, keeping %d copies\n", c.Name, c.BackupInterval, c.BackupRetention)
	ensureBootstrap()
}

func backupDisable(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup disable <container>")
		os.Exit(1)
	}
	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	c.AutoBackup = false
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving backup settings: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Automatic backups disabled for %s\n", c.Name)
}

func backupStatus(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup status <container>")
		os.Exit(1)
	}
	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Container: %s\n", c.Name)
	fmt.Printf("  Enabled: %t\n", c.AutoBackup)
	if !c.AutoBackup {
		return
	}
	fmt.Printf("  Interval: %s\n", c.BackupInterval)
	fmt.Printf("  Retention: %d\n", c.BackupRetention)
	fmt.Printf("  Directory: %s\n", containerBackupDir(c))
	if c.LastBackupAt.IsZero() {
		fmt.Println("  Last successful backup: never")
	} else {
		fmt.Printf("  Last successful backup: %s\n", c.LastBackupAt.Format(time.RFC3339))
	}
	if !c.BackupNextAttemptAt.IsZero() {
		fmt.Printf("  Next retry after: %s\n", c.BackupNextAttemptAt.Format(time.RFC3339))
	}
}

func backupCreate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup create <container> [-o file.tar.gz] [-e]")
		os.Exit(1)
	}
	name := args[0]
	output := ""
	encrypt := false
	for i := 1; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			output = args[i+1]
			i++
		} else if args[i] == "-e" || args[i] == "--encrypt" {
			encrypt = true
		}
	}
	c, err := container.Load(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if c.Status == container.Running {
		fmt.Fprintln(os.Stderr, "Error: stop the container before creating a consistent backup")
		os.Exit(1)
	}
	if output == "" {
		if err := os.MkdirAll(backupDir(), 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating backup directory: %v\n", err)
			os.Exit(1)
		}
		ext := ".tar.gz"
		if encrypt {
			ext = ".tar.gz.enc"
		}
		output = filepath.Join(backupDir(), fmt.Sprintf("%s-%s%s", c.Name, time.Now().Format("20060102-150405"), ext))
	}
	parent, err := validateBackupDirectory(filepath.Dir(output))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid backup output directory: %v\n", err)
		os.Exit(1)
	}
	output = filepath.Join(parent, filepath.Base(output))
	if err := os.MkdirAll(parent, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating backup parent: %v\n", err)
		os.Exit(1)
	}
	if err := createContainerBackup(c, output); err != nil {
		_ = os.Remove(output)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := writeBackupChecksum(output); err != nil {
		_ = os.Remove(output)
		fmt.Fprintf(os.Stderr, "Error writing checksum: %v\n", err)
		os.Exit(1)
	}
	// Encrypt backup if requested
	if encrypt {
		enc, err := container.NewBackupEncryptorFromEnv()
		if err != nil {
			_ = os.Remove(output)
			_ = os.Remove(backupChecksumPath(output))
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := enc.EncryptFile(output); err != nil {
			_ = os.Remove(output)
			_ = os.Remove(backupChecksumPath(output))
			fmt.Fprintf(os.Stderr, "Error encrypting backup: %v\n", err)
			os.Exit(1)
		}
		// Recalculate checksum for encrypted file
		_ = os.Remove(backupChecksumPath(output))
		if err := writeBackupChecksum(output); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not write encrypted checksum: %v\n", err)
		}
		fmt.Printf("Backup encrypted with AES-256-GCM\n")
	}
	fmt.Printf("Created backup: %s (%d bytes)\n", output, fileSize(output))
	fmt.Printf("Checksum: %s\n", backupChecksumPath(output))
}

func backupGenerateKey(args []string) {
	key, err := container.GenerateEncryptionKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}
	hexKey := hex.EncodeToString(key)
	fmt.Printf("Generated encryption key:\n%s\n\n", hexKey)
	fmt.Println("Save this key securely. You can:")
	fmt.Println("  1. Set environment variable: export CARDINAL_BACKUP_KEY=" + hexKey)
	fmt.Println("  2. Save to file: cardinal backup generate-key > ~/.cardinal-backup-key && chmod 600 ~/.cardinal-backup-key")
}

func validateBackupDirectory(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("backup directory is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	root := filepath.Clean(state.DataDir())
	insideState := absolute == root || strings.HasPrefix(absolute, root+string(filepath.Separator))
	for _, blocked := range []string{
		string(filepath.Separator), "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64",
		"/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin", "/srv", "/sys", "/usr", "/var",
	} {
		blocked = filepath.Clean(blocked)
		if !insideState && (absolute == blocked || strings.HasPrefix(absolute, blocked+string(filepath.Separator))) {
			return "", fmt.Errorf("backup directory %q is a protected host path", path)
		}
	}
	// Reject symlinks in every existing component. This prevents a path from
	// escaping the approved state root between validation and MkdirAll.
	current := string(filepath.Separator)
	volume := filepath.VolumeName(absolute)
	if volume != "" {
		current = volume + string(filepath.Separator)
	}
	relative := strings.TrimPrefix(absolute, current)
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("backup directory %q contains symlink component %q", path, current)
		}
	}
	return absolute, nil
}

func acquireBackupLock(c *container.Container) (*os.File, error) {
	if c == nil {
		return nil, fmt.Errorf("container is nil")
	}
	backupPath, err := validateBackupDirectory(containerBackupDir(c))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(backupPath, 0700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(backupPath, ".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, nil
		}
		return nil, err
	}
	return lock, nil
}

func releaseBackupLock(lock *os.File) {
	if lock == nil {
		return
	}
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	_ = lock.Close()
}

func automaticBackupInterval(c *container.Container) time.Duration {
	if c == nil || c.BackupInterval == "" {
		return defaultBackupInterval
	}
	interval, err := time.ParseDuration(c.BackupInterval)
	if err != nil || interval <= 0 {
		return defaultBackupInterval
	}
	return interval
}

func automaticBackupRetention(c *container.Container) int {
	if c == nil || c.BackupRetention < 1 {
		return defaultBackupRetention
	}
	return c.BackupRetention
}

func automaticBackupPath(c *container.Container, at time.Time) string {
	return filepath.Join(containerBackupDir(c), fmt.Sprintf("%s-%s.tar.gz", c.Name, at.Format("20060102-150405")))
}

func pruneAutomaticBackups(c *container.Container) error {
	dir := containerBackupDir(c)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	prefix := c.Name + "-"
	type backupEntry struct {
		name string
		date time.Time
	}
	var backups []backupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		backups = append(backups, backupEntry{name: entry.Name(), date: info.ModTime()})
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].date.After(backups[j].date) })
	retention := automaticBackupRetention(c)
	if len(backups) <= retention {
		return nil
	}
	for _, entry := range backups[retention:] {
		archivePath := filepath.Join(dir, entry.name)
		if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Remove(backupChecksumPath(archivePath)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func performAutomaticBackup(c *container.Container) error {
	if c == nil || !c.AutoBackup || c.StoppedByUser || c.Status != container.Running {
		return nil
	}
	// Re-read state immediately before touching the process. A separate CLI
	// may have stopped or started the container since the supervisor listed it.
	latest, err := container.Load(c.ID)
	if err != nil {
		return fmt.Errorf("reload container state: %w", err)
	}
	if latest.Status != container.Running || latest.StoppedByUser || !latest.AutoBackup {
		return nil
	}
	c = latest
	lock, err := acquireBackupLock(c)
	if err != nil {
		return fmt.Errorf("lock backup: %w", err)
	}
	if lock == nil {
		return nil
	}
	defer releaseBackupLock(lock)
	backupPath, err := validateBackupDirectory(containerBackupDir(c))
	if err != nil {
		return fmt.Errorf("validate backup directory: %w", err)
	}
	// Use the validated path for every archive and retention operation, even
	// when an old container state contains a changed or symlinked path.
	c.BackupDir = backupPath
	if err := os.MkdirAll(backupPath, 0700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	stopped := false
	restartAfterBackup := func() {
		if !stopped {
			return
		}
		c.StoppedByUser = false
		c.Status = container.Created
		if err := c.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "cardinal supervisor: save backup restart state %s: %v\n", c.Name, err)
		}
		if err := c.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "cardinal supervisor: restart after backup %s: %v\n", c.Name, err)
		}
	}
	defer restartAfterBackup()
	if err := c.Stop(); err != nil {
		// Stop may have completed the transition but failed while persisting
		// state. In that case still restore service availability below.
		stopped = c.Status != container.Running
		return fmt.Errorf("stop for backup: %w", err)
	}
	stopped = true
	at := time.Now()
	output := automaticBackupPath(c, at)
	if err := createContainerBackup(c, output); err != nil {
		_ = os.Remove(output)
		return fmt.Errorf("create backup: %w", err)
	}
	if err := writeBackupChecksum(output); err != nil {
		_ = os.Remove(output)
		return fmt.Errorf("write backup checksum: %w", err)
	}
	if err := pruneAutomaticBackups(c); err != nil {
		return fmt.Errorf("prune backups: %w", err)
	}
	c.BackupNextAttemptAt = time.Time{}
	c.LastBackupAt = at
	if err := c.Save(); err != nil {
		return fmt.Errorf("save backup schedule: %w", err)
	}
	fmt.Printf("Automatic backup created for %s: %s\n", c.Name, output)
	return nil
}

func createContainerBackup(c *container.Container, output string) (retErr error) {
	f, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() {
		if err := f.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()
	gw := gzip.NewWriter(f)
	defer func() {
		if err := gw.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()
	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	metadata, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := writeTarBytes(tw, "container.json", metadata, 0600); err != nil {
		return err
	}
	upper, _, _ := c.OverlayDirs()
	dataMnt := filepath.Join(state.OverlayDir(), c.ID, "data")
	if mounted, err := os.Stat(dataMnt); err == nil && mounted.IsDir() {
		upper = filepath.Join(dataMnt, "upper")
	}
	if err := addTreeToTar(tw, upper, "data"); err != nil {
		return fmt.Errorf("archive writable data: %w", err)
	}
	for _, volume := range c.Volumes {
		if strings.Contains(volume.Source, "/") || strings.Contains(volume.Source, "\\") {
			// Bind mounts are part of the container's persistent data and must
			// travel with a container transfer. Keep their archive path based on
			// the container target, never on an arbitrary host pathname.
			if filepath.Clean(volume.Source) == string(filepath.Separator) {
				return fmt.Errorf("refusing to archive host root bind mount")
			}
			if err := addTreeToTar(tw, volume.Source, backupBindPrefix(volume)); err != nil {
				return fmt.Errorf("archive bind mount %s: %w", volume.Source, err)
			}
			continue
		}
		if err := addTreeToTar(tw, state.ResolveVolume(volume.Source), filepath.Join("volumes", volume.Source)); err != nil {
			return fmt.Errorf("archive volume %s: %w", volume.Source, err)
		}
	}
	return nil
}

func backupBindPrefix(volume container.VolumeMount) string {
	target := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(volume.Target)), "/")
	if target == "" || target == "." {
		target = "root"
	}
	return filepath.Join("binds", target)
}

func addTreeToTar(tw *tar.Writer, source, prefix string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		if rel == "." {
			name = filepath.ToSlash(prefix)
		}
		h, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		h.Name = name
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	})
}

func writeTarBytes(tw *tar.Writer, name string, data []byte, mode int64) error {
	h := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: time.Now()}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func backupList(args []string) {
	root := backupDir()
	if len(args) > 0 && args[0] != "" {
		root = args[0]
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		fmt.Println("No backups found")
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Printf("%-48s %10d bytes  %s\n", filepath.ToSlash(rel), info.Size(), info.ModTime().Format(time.RFC3339))
		count++
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if count == 0 {
		fmt.Println("No backups found")
	}
}

func backupVerify(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup verify <file.tar.gz>")
		os.Exit(1)
	}
	if _, err := os.Stat(backupChecksumPath(args[0])); os.IsNotExist(err) {
		fmt.Printf("Backup is valid but unverified (no checksum sidecar): %s\n", args[0])
		return
	}
	if err := verifyBackupChecksum(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Backup verification failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backup verified: %s\n", args[0])
}

func backupRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal backup remove <file.tar.gz>")
		os.Exit(1)
	}
	archivePath, err := validateBackupArchivePath(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid backup archive: %v\n", err)
		os.Exit(1)
	}
	if err := os.Remove(archivePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: backup not found: %s\n", archivePath)
		} else {
			fmt.Fprintf(os.Stderr, "Error removing backup: %v\n", err)
		}
		os.Exit(1)
	}
	if err := os.Remove(backupChecksumPath(archivePath)); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error removing backup checksum: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed backup: %s\n", archivePath)
}

func validateBackupArchivePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("archive path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	base := filepath.Base(absolute)
	if base == ".lock" || (!strings.HasSuffix(base, ".tar.gz") && !strings.HasSuffix(base, ".tar.gz.enc")) {
		return "", fmt.Errorf("expected a .tar.gz or .tar.gz.enc archive")
	}
	if _, err := validateBackupDirectory(filepath.Dir(absolute)); err != nil {
		return "", err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("archive is not a regular file")
	}
	return absolute, nil
}

func backupRestore(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cardinal backup restore <container> <file.tar.gz> [--rebind]")
		os.Exit(1)
	}
	rebind := false
	for _, arg := range args[2:] {
		if arg == "--rebind" {
			rebind = true
		}
	}
	c, err := container.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if c.Status == container.Running {
		fmt.Fprintln(os.Stderr, "Error: stop the container before restoring a backup")
		os.Exit(1)
	}
	if err := restoreContainerBackupWithOptions(c, args[1], rebind); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if rebind {
		fmt.Printf("Rebound backup data into %s\n", c.Name)
	} else {
		fmt.Printf("Restored backup into %s\n", c.Name)
	}
}

func restoreContainerBackup(c *container.Container, archivePath string) error {
	return restoreContainerBackupWithOptions(c, archivePath, false)
}

func restoreContainerBackupWithOptions(c *container.Container, archivePath string, rebind bool) error {
	if err := verifyBackupChecksum(archivePath); err != nil {
		return fmt.Errorf("verify backup checksum: %w", err)
	}
	stage, err := os.MkdirTemp(state.DataDir(), ".cardinal-restore-")
	if err != nil {
		return fmt.Errorf("create restore staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()

	metadata, err := extractBackupToStage(stage, archivePath)
	if err != nil {
		return err
	}
	var archived container.Container
	if err := json.Unmarshal(metadata, &archived); err != nil {
		return fmt.Errorf("invalid container metadata: %w", err)
	}
	if archived.ID == "" || (!rebind && archived.ID != c.ID) {
		return fmt.Errorf("backup belongs to container %q, not %q", archived.ID, c.ID)
	}

	upper, _, _ := c.OverlayDirs()
	dataMnt := filepath.Join(state.OverlayDir(), c.ID, "data")
	if mounted, err := os.Stat(dataMnt); err == nil && mounted.IsDir() {
		upper = filepath.Join(dataMnt, "upper")
	}
	if err := applyBackupTree(filepath.Join(stage, "data"), upper, false); err != nil {
		return fmt.Errorf("restore writable data: %w", err)
	}
	for _, volume := range c.Volumes {
		archiveRoot := filepath.Join("volumes", volume.Source)
		destination := state.ResolveVolume(volume.Source)
		hostVolume := false
		if strings.Contains(volume.Source, "/") || strings.Contains(volume.Source, "\\") {
			archiveRoot = backupBindPrefix(volume)
			destination = volume.Source
			hostVolume = true
		}
		source := filepath.Join(stage, archiveRoot)
		if _, err := os.Lstat(source); os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := applyBackupTree(source, destination, hostVolume); err != nil {
			return fmt.Errorf("restore volume %s: %w", volume.Source, err)
		}
	}
	return nil
}

func extractBackupToStage(stage, archivePath string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open backup: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var metadata []byte
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if h.Name == "container.json" {
			if metadata != nil {
				return nil, fmt.Errorf("duplicate container metadata")
			}
			metadata, err = io.ReadAll(io.LimitReader(tr, 1<<20))
			if err != nil {
				return nil, fmt.Errorf("read metadata: %w", err)
			}
			continue
		}
		// Backup archives contain explicit directory headers such as `data`
		// before their child entries (`data/file`). Accept only these three
		// archive namespaces, including their root directory headers. This keeps
		// traversal and host-path protections intact while allowing empty data
		// directories to be restored correctly.
		entryName := strings.TrimSuffix(filepath.ToSlash(h.Name), "/")
		allowedNamespace := func(namespace string) bool {
			return entryName == namespace || strings.HasPrefix(entryName, namespace+"/")
		}
		if !allowedNamespace("data") && !allowedNamespace("volumes") && !allowedNamespace("binds") {
			return nil, fmt.Errorf("unsafe backup entry %q", h.Name)
		}
		target, err := container.SafeBackupPath(stage, h.Name)
		if err != nil {
			return nil, err
		}
		if err := container.RejectBackupSymlinkAncestors(stage, filepath.Dir(target)); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(target); err == nil {
			return nil, fmt.Errorf("duplicate backup entry %q", h.Name)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				return nil, err
			}
			if err := os.Chmod(target, h.FileInfo().Mode().Perm()); err != nil {
				return nil, err
			}
			if err := os.Chtimes(target, h.ModTime, h.ModTime); err != nil {
				return nil, err
			}
			continue
		}
		if h.FileInfo().Mode()&os.ModeSymlink != 0 {
			if err := validateBackupLink(h.Name, h.Linkname, strings.HasPrefix(h.Name, "binds/")); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return nil, err
			}
			if err := os.Symlink(h.Linkname, target); err != nil {
				return nil, err
			}
			continue
		}
		if !h.FileInfo().Mode().IsRegular() {
			// Skip overlayfs whiteout files (.wh.*), character/block
			// devices, fifos, and sockets — these are runtime artifacts
			// that cannot be meaningfully restored.
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return nil, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err := os.Chmod(target, h.FileInfo().Mode().Perm()); err != nil {
			return nil, err
		}
		if err := os.Chtimes(target, h.ModTime, h.ModTime); err != nil {
			return nil, err
		}
	}
	if metadata == nil {
		return nil, fmt.Errorf("backup is missing container.json")
	}
	return metadata, nil
}

func validateBackupLink(name, link string, hostVolume bool) error {
	if link == "" {
		return fmt.Errorf("empty symlink target for %q", name)
	}
	if !hostVolume {
		// Absolute links are valid inside a container root filesystem, e.g.
		// /bin/busybox, and are never followed during archive extraction.
		return nil
	}
	if filepath.IsAbs(link) || strings.Contains(link, "\\") {
		return fmt.Errorf("symlink escapes host volume: %q -> %q", name, link)
	}
	clean := filepath.Clean(filepath.FromSlash(filepath.Join(filepath.Dir(name), link)))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("symlink escapes host volume: %q -> %q", name, link)
	}
	return nil
}

func applyBackupTree(source, destination string, hostVolume bool) error {
	if _, err := os.Lstat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := container.RejectBackupSymlinkAncestors(destination, destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, sourceInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := destination
		if rel != "." {
			target = filepath.Join(destination, rel)
			if err := container.RejectBackupSymlinkAncestors(destination, filepath.Dir(target)); err != nil {
				return err
			}
		}
		if sourceInfo.IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			if err := os.Chmod(target, sourceInfo.Mode().Perm()); err != nil {
				return err
			}
			return os.Chtimes(target, sourceInfo.ModTime(), sourceInfo.ModTime())
		}
		if sourceInfo.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err == nil {
				err = validateBackupLink(rel, link, hostVolume)
			}
			if err != nil {
				return err
			}
			return atomicRestoreSymlink(target, link)
		}
		if !sourceInfo.Mode().IsRegular() {
			return fmt.Errorf("unsupported staged entry %q", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(target), ".cardinal-restore-file-")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		removeTemp := true
		defer func() {
			if removeTemp {
				_ = os.Remove(tmpPath)
			}
		}()
		if _, err := io.Copy(tmp, in); err != nil {
			_ = in.Close()
			_ = tmp.Close()
			return err
		}
		if err := in.Close(); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Chmod(sourceInfo.Mode().Perm()); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Chtimes(tmpPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, target); err != nil {
			return err
		}
		removeTemp = false
		return nil
	})
}

func atomicRestoreSymlink(target, link string) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".cardinal-restore-link-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	if err := os.Symlink(link, tmpPath); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func backupChecksumPath(archivePath string) string {
	return archivePath + ".sha256"
}

func backupDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeBackupChecksum(archivePath string) error {
	digest, err := backupDigest(archivePath)
	if err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	checksumPath := backupChecksumPath(archivePath)
	tmp, err := os.CreateTemp(filepath.Dir(checksumPath), ".cardinal-checksum-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := fmt.Fprintf(tmp, "%s  %s\n", digest, filepath.Base(archivePath)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, checksumPath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func verifyBackupChecksum(archivePath string) error {
	checksumPath := backupChecksumPath(archivePath)
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Archives created before checksum sidecars were introduced remain
			// restorable. `cardinal backup verify` still reports this as unverified.
			return nil
		}
		return err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 || len(fields[0]) != sha256.Size*2 {
		return fmt.Errorf("invalid checksum file %s", checksumPath)
	}
	for _, r := range fields[0] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fmt.Errorf("invalid checksum value in %s", checksumPath)
		}
	}
	actual, err := backupDigest(archivePath)
	if err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	if !strings.EqualFold(actual, fields[0]) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", fields[0], actual)
	}
	return nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
