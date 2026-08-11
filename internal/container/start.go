//go:build linux

package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dck/internal/image"
	"dck/internal/log"
	"dck/internal/network"
	"dck/internal/state"
)

func commandContext30(name string, arg ...string) *exec.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cmd := exec.CommandContext(ctx, name, arg...)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return cmd
}

func (c *Container) Start() error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	return c.startInternal()
}

func (c *Container) startInternal() error {
	c.mu.Lock()
	c.cleanupStarted = false
	c.mu.Unlock()
	stoppedContainers.Delete(c.ID)

	c.dataMu.Lock()
	if c.Status == Running {
		c.dataMu.Unlock()
		return fmt.Errorf("container %s is already running", c.ID)
	}
	c.Status = Created
	c.dataMu.Unlock()

	// A previous detached console server may outlive a failed or automatic
	// restart. Stop it before the next run truncates the shared log file.
	c.killConsoleServe()

	if err := state.EnsureDirs(); err != nil {
		return err
	}

	merged, err := c.setupFilesystem()
	if err != nil {
		return err
	}

	cmd, err := c.buildUnshareCmd(merged)
	if err != nil {
		return err
	}

	cleanupIO, err := c.setupIO(cmd)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		if cleanupIO != nil {
			cleanupIO()
		}
		return fmt.Errorf("start: %w", err)
	}
	if cleanupIO != nil {
		defer cleanupIO()
	}

	childPID := c.resolveChildPID(cmd.Process.Pid)
	c.setupContainerResources(childPID)

	if c.IP != "" {
		RegisterDNSName(c.Name, c.IP)
	}
	if c.DNS != nil || c.IP != "" {
		EnsureContainerHosts(merged, c.Name, c.IP, c.DNS)
	}

	c.dataMu.Lock()
	c.PID = childPID
	c.Status = Running
	c.StoppedByUser = false
	c.dataMu.Unlock()
	if err := c.Save(); err != nil {
		return err
	}
	EmitEvent(EventStart, c)

	if c.Detach {
		ctx, cancel := context.WithCancel(context.Background())
		c.cancelHealth = cancel
		monitorContainer(c, cmd, ctx)
		fmt.Println(shortID(c.ID, 12))
		return nil
	}

	return c.runForeground(cmd)
}

func (c *Container) setupFilesystem() (merged string, err error) {
	img := image.LoadFromStore(c.ImageName, c.ImageTag)
	if img == nil {
		return "", fmt.Errorf("image %s:%s not found", c.ImageName, c.ImageTag)
	}

	rootfsDir := state.ImageRootfsDir(c.ImageName, c.ImageTag)
	upper, work, mergedDir := c.OverlayDirs()
	if err := os.MkdirAll(filepath.Dir(upper), 0700); err != nil {
		return "", err
	}

	if err := SetupDiskLimit(state.OverlayDir(), c.ID, c.DiskLimit); err != nil {
		return "", fmt.Errorf("disk limit: %w", err)
	}

	dataMnt := filepath.Join(state.OverlayDir(), c.ID, "data")
	if isMounted(dataMnt) {
		upper = filepath.Join(dataMnt, "upper")
		work = filepath.Join(dataMnt, "work")
	}

	if _, err := os.Stat(mergedDir); os.IsNotExist(err) || !isOverlayMounted(mergedDir) {
		if err := SetupOverlay(rootfsDir, upper, work, mergedDir); err != nil {
			return "", fmt.Errorf("overlay: %w", err)
		}
	}

	for _, vol := range c.Volumes {
		spec := ParseVolumeString(vol.Source + ":" + vol.Target)
		if err := MountVolume(spec, mergedDir); err != nil {
			return "", fmt.Errorf("mount volume %s -> %s: %w", vol.Source, vol.Target, err)
		}
	}

	if err := c.InjectSecrets(mergedDir); err != nil {
		return "", fmt.Errorf("inject secrets: %w", err)
	}

	return mergedDir, nil
}

func (c *Container) buildUnshareCmd(merged string) (*exec.Cmd, error) {
	binPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("executable: %w", err)
	}

	unshareArgs := []string{
		"--fork", "--pid", "--mount", "--uts", "--ipc", "--kill-child",
		binPath, "init", c.ID, merged,
	}
	if c.NetworkMode != "host" {
		unshareArgs = append([]string{"--net"}, unshareArgs...)
	}
	return exec.Command("unshare", unshareArgs...), nil
}

func (c *Container) setupIO(cmd *exec.Cmd) (func(), error) {
	binPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("executable: %w", err)
	}

	switch {
	case c.Detach:
		stdinR, stdinW, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("stdin pipe: %w", err)
		}
		stdoutR, stdoutW, err := os.Pipe()
		if err != nil {
			_ = stdinR.Close()
			_ = stdinW.Close()
			return nil, fmt.Errorf("stdout pipe: %w", err)
		}

		cmd.Stdin = stdinR
		cmd.Stdout = stdoutW
		cmd.Stderr = stdoutW

		serve := exec.Command(binPath, "console-serve", c.ID)
		serve.ExtraFiles = []*os.File{stdinW, stdoutR}
		if err := serve.Start(); err != nil {
			_ = stdinR.Close()
			_ = stdinW.Close()
			_ = stdoutR.Close()
			_ = stdoutW.Close()
			return nil, fmt.Errorf("console serve: %w", err)
		}
		c.ConsoleServePID = serve.Process.Pid
		_ = stdinW.Close()
		_ = stdoutR.Close()
		return nil, nil
	case c.Interactive || c.TTY:
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return nil, nil
	default:
		logFile, err := OpenFreshLogFile(c.LogFile(), 0600)
		if err != nil {
			return nil, fmt.Errorf("log: %w", err)
		}
		cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
		cmd.Stderr = io.MultiWriter(logFile, os.Stderr)
		return func() { _ = logFile.Close() }, nil
	}
}

func (c *Container) resolveChildPID(unsharePID int) int {
	childPID := findChildPID(unsharePID)
	if childPID == 0 {
		childPID = unsharePID + 1
	}
	return childPID
}

func (c *Container) setupContainerResources(childPID int) {
	cpath, err := setupContainerCgroup(c.ID, childPID, c.MemoryLimit, c.CPUCount)
	if err != nil {
		log.Warn("Cgroup setup: %v (container will run without resource limits)", err)
	} else {
		c.CgroupPath = cpath
	}

	if c.NeedsNetwork() && runtime.GOOS == "linux" {
		if IsRootless() {
			if ip, err := SetupRootlessNetwork(childPID, c.ID); err != nil {
				log.Warn("Rootless network: %v (container will run without network)", err)
			} else {
				c.IP = ip
				for _, p := range c.Ports {
					pids, err := RootlessPortForward(p.HostPort, p.ContainerPort, p.Protocol, c.IP)
					if err != nil {
						log.Warn("  port %d -> %d: %v", p.HostPort, p.ContainerPort, err)
					} else {
						c.PortForwardPIDs = append(c.PortForwardPIDs, pids...)
					}
				}
			}
		} else if err := setupNetworking(c, childPID); err != nil {
			log.Warn("Network setup: %v (container will run without network)", err)
		}
	}
}

func (c *Container) runForeground(cmd *exec.Cmd) error {
	var healthCancel context.CancelFunc
	if c.Healthcheck != nil {
		var healthCtx context.Context
		healthCtx, healthCancel = context.WithCancel(context.Background())
		c.cancelHealth = healthCancel
		go c.runHealthcheck(healthCtx)
	}

	err := cmd.Wait()
	if healthCancel != nil {
		healthCancel()
		c.cancelHealth = nil
	}
	c.dataMu.Lock()
	c.PID = 0
	c.Status = Stopped
	c.dataMu.Unlock()
	c.cleanupNetwork()
	_ = c.Save()

	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	if shouldRestart(c.Restart, exitCode, c.StoppedByUser) {
		return c.restart()
	}
	if c.RemoveOnExit {
		cleanupContainer(c)
	}
	return err
}

type ignoreErrWriter struct{ w io.Writer }

func (w *ignoreErrWriter) Write(p []byte) (int, error) {
	n, _ := w.w.Write(p)
	return n, nil
}

func newIgnoreErrWriter(w io.Writer) *ignoreErrWriter {
	return &ignoreErrWriter{w: w}
}

func (c *Container) NeedsNetwork() bool {
	return c.NetworkMode != "none" && c.NetworkMode != "host"
}

func findChildPID(ppid int) int {
	for i := 0; i < 100; i++ {
		out, err := exec.Command("pgrep", "-P", strconv.Itoa(ppid)).Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				pid, err := strconv.Atoi(strings.TrimSpace(line))
				if err == nil && pid > 0 {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	data, err := os.ReadFile("/proc/" + strconv.Itoa(ppid) + "/task/" + strconv.Itoa(ppid) + "/children")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
				return pid
			}
		}
	}
	return 0
}

func setupNetworking(c *Container, pid int) error {
	network.EnsureNetBase()
	ip, err := network.AllocateIP()
	if err != nil {
		return err
	}
	if err := network.SetupVeth(c.ID, pid, ip); err != nil {
		network.ReleaseIP(ip)
		return err
	}
	for _, p := range c.Ports {
		if err := network.AddPortForwarding(ip, p.HostPort, p.ContainerPort, p.Protocol); err != nil {
			log.Warn("  port %d -> %d: %v", p.HostPort, p.ContainerPort, err)
		}
	}
	c.IP = ip
	return nil
}

func shouldRestart(policy string, exitCode int, stoppedByUser bool) bool {
	switch policy {
	case "always":
		return true
	case "unless-stopped":
		return !stoppedByUser
	case "on-failure":
		return exitCode != 0
	default:
		return false
	}
}

func (c *Container) restart() error {
	time.Sleep(time.Second)
	c.dataMu.Lock()
	c.Status = Created
	c.dataMu.Unlock()
	c.mu.Lock()
	c.cleanupStarted = false
	c.mu.Unlock()
	return c.startInternal()
}

func monitorContainer(c *Container, cmd *exec.Cmd, ctx context.Context) {
	if c.Healthcheck != nil {
		go c.runHealthcheck(ctx)
	}
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if c.cancelHealth != nil {
			c.cancelHealth()
			c.cancelHealth = nil
		}
		c.mu.Lock()
		if c.cleanupStarted {
			c.mu.Unlock()
			return
		}
		c.cleanupStarted = true
		c.mu.Unlock()

		stoppedByUser := c.StoppedByUser
		if !stoppedByUser {
			if _, ok := stoppedContainers.Load(c.ID); ok {
				stoppedByUser = true
			}
		}
		UnregisterDNSName(c.Name)
		c.dataMu.Lock()
		c.PID = 0
		c.Status = Stopped
		c.dataMu.Unlock()
		c.cleanupNetwork()
		c.killConsoleServe()
		_ = c.Save()

		if shouldRestart(c.Restart, exitCode, stoppedByUser) {
			go func() {
				time.Sleep(time.Second)
				c.dataMu.Lock()
				c.Status = Created
				c.dataMu.Unlock()
				c.mu.Lock()
				c.cleanupStarted = false
				c.mu.Unlock()
				_ = c.Start()
			}()
		} else if c.RemoveOnExit {
			cleanupContainer(c)
		}
	}()
}

func (c *Container) runHealthcheck(ctx context.Context) {
	hc := c.Healthcheck
	interval := time.Duration(hc.Interval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}
	retries := hc.Retries
	if retries == 0 {
		retries = 3
	}
	timeout := time.Duration(hc.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
		c.dataMu.RLock()
		isRunning := c.Status == Running
		c.dataMu.RUnlock()
		if !isRunning {
			return
		}
		err := c.execHealthcheck(hc.Cmd, timeout)
		if err != nil {
			failures++
			if failures >= retries {
				if err := commandContext30("kill", "-9", strconv.Itoa(c.PID)).Run(); err != nil {
					log.Warn("kill -9 %d: %v", c.PID, err)
				}
				return
			}
		} else {
			failures = 0
		}
	}
}

func (c *Container) execHealthcheck(cmd string, timeout time.Duration) error {
	args := []string{"-t", strconv.Itoa(c.PID), "-m", "-p", "-i", "-n", "--", "sh", "-c", cmd}
	ecmd := exec.Command("nsenter", args...)
	done := make(chan error, 1)
	go func() { done <- ecmd.Run() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = ecmd.Process.Kill()
		return fmt.Errorf("healthcheck timed out after %v", timeout)
	}
}

func (c *Container) cleanupNetwork() {
	if c.IP == "" {
		return
	}
	var ports []network.PortRule
	for _, p := range c.Ports {
		ports = append(ports, network.PortRule{HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol, ContainerIP: c.IP})
	}
	network.CleanupContainerNetwork(c.ID, c.IP, ports)
	c.IP = ""
}

func cleanupContainer(c *Container) {
	if runtime.GOOS != "linux" {
		return
	}
	if c.ConsoleServePID > 0 {
		if proc, err := os.FindProcess(c.ConsoleServePID); err == nil {
			_ = proc.Kill()
		}
		c.ConsoleServePID = 0
	}
	if c.cancelHealth != nil {
		c.cancelHealth()
		c.cancelHealth = nil
	}
	c.cleanupNetwork()
	upper, _, merged := c.OverlayDirs()
	unmountOverlay(merged)
	TeardownDiskLimit(state.OverlayDir(), c.ID)
	_ = os.Remove(state.ConsolePath(c.ID))
	_ = os.RemoveAll(filepath.Dir(upper))
	_ = os.Remove(c.LogFile())
	cleanupContainerCgroup(c.ID, c.CgroupPath)
	_ = c.DeleteState()
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
