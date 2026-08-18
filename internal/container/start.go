//go:build linux

package container

import (
	"bufio"
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
	staleIP := c.IP
	c.Status = Created
	c.dataMu.Unlock()
	if staleIP != "" {
		// A hard host crash can leave persisted allocation state behind even
		// though the process is no longer alive. Release it before allocating a
		// fresh address for this start.
		c.cleanupNetwork()
	}

	// A previous detached console server may outlive a failed or automatic
	// restart. Stop it before the next run truncates the shared log file.
	c.killConsoleServe()

	if err := state.EnsureDirs(); err != nil {
		return err
	}
	if IsRootless() && c.NetworkMode != "" && c.NetworkMode != "bridge" && c.NetworkMode != "host" && c.NetworkMode != "none" {
		return fmt.Errorf("rootless mode does not support user-defined bridge network %q", c.NetworkMode)
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

	// A bridge network must be attached before the image startup script runs.
	// Otherwise a fast-failing command can destroy the init process before the
	// host has moved the veth into its namespace, while a long-running startup
	// script can block the network setup indefinitely. The init process signals
	// readiness through inherited file descriptors and waits for the parent to
	// release it after networking is complete.
	networkHandshake := c.NeedsNetwork()
	var networkReadyR, networkReadyW, networkGoR, networkGoW *os.File
	if networkHandshake {
		networkReadyR, networkReadyW, err = os.Pipe()
		if err != nil {
			if cleanupIO != nil {
				cleanupIO()
			}
			return fmt.Errorf("network ready pipe: %w", err)
		}
		networkGoR, networkGoW, err = os.Pipe()
		if err != nil {
			_ = networkReadyR.Close()
			_ = networkReadyW.Close()
			if cleanupIO != nil {
				cleanupIO()
			}
			return fmt.Errorf("network release pipe: %w", err)
		}
		cmd.ExtraFiles = append(cmd.ExtraFiles, networkReadyW, networkGoR)
		cmd.Env = initNetworkEnvironment(os.Environ())
	}
	closeNetworkParentFiles := func() {
		for _, file := range []*os.File{networkReadyR, networkReadyW, networkGoR, networkGoW} {
			if file != nil {
				_ = file.Close()
			}
		}
	}
	defer closeNetworkParentFiles()

	if err := cmd.Start(); err != nil {
		if cleanupIO != nil {
			cleanupIO()
		}
		return fmt.Errorf("start: %w", err)
	}
	if cleanupIO != nil {
		defer cleanupIO()
	}
	if networkHandshake {
		// The child owns these ends after exec. Close the parent copies so EOF
		// and pipe readiness remain unambiguous.
		_ = networkReadyW.Close()
		networkReadyW = nil
		_ = networkGoR.Close()
		networkGoR = nil
		if err := awaitInitNetworkReady(networkReadyR); err != nil {
			c.abortStart(cmd)
			return fmt.Errorf("container init network handshake: %w", err)
		}
	}

	childPID := c.resolveChildPID(cmd.Process.Pid)
	if childPID <= 0 {
		c.abortStart(cmd)
		return fmt.Errorf("could not determine the container init process")
	}
	if err := c.setupContainerResources(childPID); err != nil {
		if networkHandshake {
			_ = releaseInitNetwork(networkGoW, false)
		}
		c.abortStart(cmd)
		return fmt.Errorf("configure container resources: %w", err)
	}
	if networkHandshake {
		if err := releaseInitNetwork(networkGoW, true); err != nil {
			c.abortStart(cmd)
			return fmt.Errorf("release container init after network setup: %w", err)
		}
	}

	if c.IP != "" {
		RegisterDNSName(c.Name, c.IP)
	}
	if c.DNS != nil || c.IP != "" {
		EnsureContainerHosts(merged, c.Name, c.IP, c.DNS)
	}

	mountNS, pidNS, networkNS, ipcNS, utsNS, err := containerNamespaceIdentities(childPID)
	if err != nil {
		// The init process may have exited while startup was in progress (for
		// example because the image command was invalid). Do not leave the
		// overlay, network, cgroup, or console helper behind in that case.
		c.abortStart(cmd)
		return fmt.Errorf("record container namespaces: %w", err)
	}
	// The container may have been removed (dck rm) while this start was in
	// flight (the supervisor's automatic restart racing a user removal). Never
	// resurrect a deleted container: tear down what we just spawned instead of
	// re-persisting its state.
	if !state.FileExists(state.ContainerPath(c.ID)) || IsBeingRemoved(c.ID) {
		c.abortStart(cmd)
		return fmt.Errorf("container %s was removed during start", c.ID)
	}
	c.dataMu.Lock()
	c.PID = childPID
	c.UnsharePID = cmd.Process.Pid
	c.MountNamespace = mountNS
	c.PIDNamespace = pidNS
	c.NetworkNamespace = networkNS
	c.IPCNamespace = ipcNS
	c.UTSNamespace = utsNS
	c.Status = Running
	c.StoppedByUser = false
	c.LastStartedAt = time.Now()
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

func initNetworkEnvironment(env []string) []string {
	filtered := make([]string, 0, len(env)+2)
	for _, entry := range env {
		if strings.HasPrefix(entry, "DCK_INIT_READY_FD=") || strings.HasPrefix(entry, "DCK_INIT_GO_FD=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "DCK_INIT_READY_FD=3", "DCK_INIT_GO_FD=4")
}

func awaitInitNetworkReady(ready *os.File) error {
	if ready == nil {
		return fmt.Errorf("network readiness pipe is unavailable")
	}
	result := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(ready).ReadString('\n')
		if err != nil {
			result <- fmt.Errorf("init exited before signaling readiness: %w", err)
			return
		}
		if strings.TrimSpace(line) != "ready" {
			result <- fmt.Errorf("unexpected init readiness message %q", strings.TrimSpace(line))
			return
		}
		result <- nil
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out waiting for init readiness")
	}
}

func releaseInitNetwork(gate *os.File, allow bool) error {
	if gate == nil {
		return fmt.Errorf("network release pipe is unavailable")
	}
	message := "abort\n"
	if allow {
		message = "go\n"
	}
	_, err := gate.WriteString(message)
	return err
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
		mountType := vol.Type
		if mountType == "" {
			mountType = VolumeTypeBind
			if !strings.Contains(vol.Source, "/") && !strings.Contains(vol.Source, "\\") {
				mountType = VolumeTypeVolume
			}
		}
		spec := &VolumeSpec{
			Type:           mountType,
			Source:         vol.Source,
			Target:         vol.Target,
			ReadOnly:       vol.ReadOnly,
			Propagation:    vol.Propagation,
			SELinuxRelabel: vol.SELinuxRelabel,
			NoCopy:         vol.NoCopy,
		}
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
		if err := rotateLogFile(c.LogFile(), maxLogFiles); err != nil {
			return nil, fmt.Errorf("rotate log: %w", err)
		}
		rotWriter, err := NewRotatingWriter(c.LogFile(), 0600)
		if err != nil {
			return nil, fmt.Errorf("log: %w", err)
		}
		cmd.Stdout = io.MultiWriter(rotWriter, os.Stdout)
		cmd.Stderr = io.MultiWriter(rotWriter, os.Stderr)
		return func() { _ = rotWriter.Close() }, nil
	}
}

// abortStart tears down everything that was set up for a container whose start
// failed after unshare was already spawned.
func (c *Container) abortStart(cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	UnregisterDNSName(c.Name)
	c.cleanupRootlessPorts()
	c.killConsoleServe()
	c.cleanupNetwork()
	upper, _, mergedDir := c.OverlayDirs()
	unmountOverlay(mergedDir)
	TeardownDiskLimit(state.OverlayDir(), c.ID)
	cleanupContainerCgroup(c.ID, c.CgroupPath)
	_ = os.Remove(state.ConsolePath(c.ID))
	_ = os.RemoveAll(filepath.Dir(upper))
}

func (c *Container) resolveChildPID(unsharePID int) int {
	if pid := findChildPID(unsharePID); pid > 0 {
		return pid
	}

	// On some kernels/util-linux combinations the process created by
	// `unshare --fork` is no longer reported in /proc/<pid>/task/<pid>/children
	// by the time the parent is inspected. This is especially common when the
	// init process immediately pivots its root or execs the image command.
	// First inspect /proc process metadata directly. This still works after the
	// init process has exec'd Java, nginx, or another application and avoids
	// depending on pgrep being installed.
	if pid := findProcChildPID(unsharePID); pid > 0 {
		return pid
	}

	// Finally locate the exact internal command line. This covers kernels where
	// the parent relationship is briefly hidden during namespace setup; never
	// guess from PID arithmetic or an unrelated process.
	return findContainerInitPID(unsharePID, c.ID)
}

func findProcChildPID(ppid int) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || pid == ppid {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		closing := strings.LastIndexByte(string(stat), ')')
		if closing < 0 {
			continue
		}
		fields := strings.Fields(string(stat[closing+1:]))
		if len(fields) < 2 {
			continue
		}
		parent, err := strconv.Atoi(fields[1])
		if err == nil && parent == ppid && pidAlive(pid) {
			return pid
		}
	}
	return 0
}

func findContainerInitPID(unsharePID int, containerID string) int {
	if containerID == "" {
		return 0
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir("/proc")
		if err == nil {
			for _, entry := range entries {
				pid, err := strconv.Atoi(entry.Name())
				if err != nil || pid <= 0 || pid == unsharePID {
					continue
				}
				cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
				if err != nil {
					continue
				}
				if isContainerInitCommandline(cmdline, containerID) && pidAlive(pid) {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0
}

func isContainerInitCommandline(cmdline []byte, containerID string) bool {
	if containerID == "" {
		return false
	}
	args := strings.Split(string(cmdline), "\x00")
	return len(args) >= 3 && args[1] == "init" && args[2] == containerID
}

func (c *Container) setupContainerResources(childPID int) error {
	cpath, err := setupContainerCgroup(c.ID, childPID, c.MemoryLimit, c.CPUCount)
	if err != nil {
		log.Warn("Cgroup setup: %v (container will run without resource limits)", err)
	} else {
		c.CgroupPath = cpath
	}

	if c.NeedsNetwork() && runtime.GOOS == "linux" {
		if IsRootless() {
			if c.NetworkMode != "" && c.NetworkMode != "bridge" && c.NetworkMode != "host" && c.NetworkMode != "none" {
				return fmt.Errorf("rootless mode does not support user-defined bridge network %q; use bridge/none/host", c.NetworkMode)
			}
			if ip, err := SetupRootlessNetwork(childPID, c.ID); err != nil {
				return fmt.Errorf("rootless network: %w", err)
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
			return fmt.Errorf("network setup: %w", err)
		}
	}
	return nil
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

	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	finalized, restart := c.finalizeStopped(exitCode)
	if !finalized {
		return err
	}
	if restart {
		if !c.allowAutomaticRestart(time.Now()) {
			log.Warn("container %s stopped restarting after crash-loop limit (%d attempts in %s)", c.Name, c.restartMaxAttempts(), c.restartWindowDuration())
			return err
		}
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
	// The kernel publishes the direct children of a process in
	// /proc/<pid>/task/<pid>/children — no pgrep dependency and no guessing.
	// unshare forks exactly one child (the container init), so the first entry
	// is the process we are looking for.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", ppid, ppid)); err == nil {
			if fields := strings.Fields(string(data)); len(fields) > 0 {
				if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 && pidAlive(pid) {
					return pid
				}
			}
		}
		// Fall back to pgrep on kernels that do not expose the children file.
		if out, err := exec.Command("pgrep", "-P", strconv.Itoa(ppid)).Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && pid > 0 {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0
}

func setupNetworking(c *Container, pid int) error {
	bridge := network.BridgeName
	gateway := network.BridgeIP
	prefix := "24"
	allocationName := ""

	if c.NetworkMode != "" && c.NetworkMode != "bridge" {
		n, err := network.LoadNetwork(c.NetworkMode)
		if err != nil {
			return err
		}
		if err := network.EnsureUserNetwork(n.Name); err != nil {
			return err
		}
		bridge = n.Bridge
		gateway = n.Gateway
		allocationName = n.Name
		prefix = network.CIDRPrefix(n.Subnet)
	}

	var ip string
	var err error
	if allocationName == "" {
		network.EnsureNetBase()
		ip, err = network.AllocateIP()
	} else {
		ip, err = network.AllocateNetworkIP(allocationName)
	}
	if err != nil {
		return err
	}
	if err := network.SetupVethOnBridge(c.ID, pid, ip, bridge, gateway, prefix); err != nil {
		if allocationName == "" {
			network.ReleaseIP(ip)
		} else {
			network.ReleaseNetworkIP(allocationName, ip)
		}
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
	if stoppedByUser {
		return false
	}
	switch policy {
	case "always":
		return true
	case "unless-stopped":
		return true
	case "on-failure":
		return exitCode != 0
	default:
		return false
	}
}

func (c *Container) restart() error {
	time.Sleep(c.restartDelay())
	if !c.canRestartAfterDelay() {
		return nil
	}
	c.dataMu.Lock()
	c.Status = Created
	c.dataMu.Unlock()
	c.mu.Lock()
	c.cleanupStarted = false
	c.mu.Unlock()
	return c.startInternal()
}

// finalizeStopped marks a container as stopped after its main process exited,
// releases every container resource (network, console-serve, cgroup, DNS), and
// persists the state. It returns whether the caller actually performed the
// finalization (false when another path already owns the cleanup) and whether
// the configured restart policy wants an automatic restart.
func (c *Container) finalizeStopped(exitCode int) (finalized, restart bool) {
	c.mu.Lock()
	if c.cleanupStarted {
		c.mu.Unlock()
		return false, false
	}
	c.cleanupStarted = true
	c.mu.Unlock()

	stoppedByUser := c.stoppedByUser()
	if !stoppedByUser {
		if _, ok := stoppedContainers.Load(c.ID); ok {
			stoppedByUser = true
		}
	}
	UnregisterDNSName(c.Name)
	c.dataMu.Lock()
	c.PID = 0
	c.UnsharePID = 0
	c.MountNamespace = 0
	c.PIDNamespace = 0
	c.NetworkNamespace = 0
	c.IPCNamespace = 0
	c.UTSNamespace = 0
	c.Status = Stopped
	c.LastExitAt = time.Now()
	c.dataMu.Unlock()
	c.cleanupNetwork()
	c.killConsoleServe()
	cleanupContainerCgroup(c.ID, c.CgroupPath)
	if saveErr := c.Save(); saveErr != nil {
		log.Warn("save stopped container %s: %v", c.Name, saveErr)
	}
	c.markStableIfNeeded(time.Now())
	return true, shouldRestart(c.Restart, exitCode, stoppedByUser)
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

		finalized, restart := c.finalizeStopped(exitCode)
		if !finalized {
			return
		}
		if restart && (!c.Detach || c.Restart == "on-failure") {
			if !c.allowAutomaticRestart(time.Now()) {
				log.Warn("container %s stopped restarting after crash-loop limit (%d attempts in %s)", c.Name, c.restartMaxAttempts(), c.restartWindowDuration())
				return
			}
			go func() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(c.restartDelay()):
				}
				if !c.canRestartAfterDelay() {
					return
				}
				c.dataMu.Lock()
				c.Status = Created
				c.dataMu.Unlock()
				c.mu.Lock()
				c.cleanupStarted = false
				c.mu.Unlock()
				if err := c.Start(); err != nil {
					log.Warn("restart container %s: %v", c.Name, err)
				}
			}()
		} else if c.RemoveOnExit {
			cleanupContainer(c)
		}
	}()
}

// HandleMainProcessExit is invoked by the supervisor when an orphaned container
// main process (unshare) exits after the original `dck run -d` process is gone.
// It finalizes state and resources exactly like the in-process monitor would.
// It reports whether finalization actually ran, so callers can avoid repeating
// the work (e.g. when the state file vanished between Load and finalize).
func HandleMainProcessExit(id string) bool {
	c, err := Load(id)
	if err != nil {
		return false
	}
	return c.handleMainProcessExit()
}

func (c *Container) handleMainProcessExit() bool {
	// A concurrent Load() may have already normalized the status to "stopped";
	// finalizeStopped is idempotent and still releases network, console-serve,
	// cgroup and DNS resources, which is exactly what the supervisor must do.
	finalized, restart := c.finalizeStopped(0)
	if !finalized {
		return false
	}
	// Restart scheduling for detached containers is owned by the supervisor
	// (adoptEligibleContainers), which honors restart delays and the crash-loop
	// budget. RemoveOnExit containers are dropped here unless a restart policy
	// will bring them back (mirrors the in-process monitor behavior).
	if !restart && c.RemoveOnExit {
		cleanupContainer(c)
	}
	return true
}

func (c *Container) canRestartAfterDelay() bool {
	if _, stopped := stoppedContainers.Load(c.ID); stopped {
		return false
	}
	latest, err := Load(c.ID)
	if err != nil {
		return false
	}
	if latest.Status != Stopped {
		return false
	}
	return !latest.stoppedByUser()
}

func (c *Container) stoppedByUser() bool {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return c.StoppedByUser
}

func (c *Container) restartDelay() time.Duration {
	if c.RestartDelay != "" {
		if delay, err := time.ParseDuration(c.RestartDelay); err == nil && delay > 0 {
			return delay
		}
	}
	return time.Second
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
	if err := c.validateNamespaceTarget(); err != nil {
		return err
	}
	args := []string{"-t", strconv.Itoa(c.PID), "-m", "-p", "-i", "-n", "-r", "--", "sh", "-c", cmd}
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
	c.cleanupRootlessPorts()
	if c.IP == "" {
		return
	}
	var ports []network.PortRule
	for _, p := range c.Ports {
		ports = append(ports, network.PortRule{HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol, ContainerIP: c.IP})
	}
	if c.NetworkMode != "" && c.NetworkMode != "bridge" {
		network.ReleaseNetworkIP(c.NetworkMode, c.IP)
		network.RemoveVeth(c.ID)
		for _, p := range ports {
			network.RemovePortForwarding(c.IP, p.HostPort, p.ContainerPort, p.Protocol)
		}
	} else {
		network.CleanupContainerNetwork(c.ID, c.IP, ports)
	}
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
