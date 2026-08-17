//go:build linux

package cmd

// cobra_commands.go — central registration of every cobra sub-command.
//
// Each cobra command is a thin wrapper around the existing legacy free
// function (Run(args []string), Stop(args []string), etc.). The wrappers
// exist so the legacy functions stay 1-1 testable and so this file is the
// single, auditable place that decides which commands are exposed.
//
// To add a new command: define `func Foo(args []string)` somewhere in
// this package, then add an entry below in `allCommands`.

import (
	"fmt"

	"github.com/spf13/cobra"
)

// runFn is the legacy free-function shape used by every legacy command
// implementation. It receives the positional args (already parsed by
// cobra's flag machinery) and is expected to interact with the OS directly.
type runFn func(args []string)

// commandSpec captures one cobra command registration. Keeping it as a
// value type lets the table be inspected, linted and re-ordered easily.
type commandSpec struct {
	use   string
	short string
	run   runFn
	long  string
}

// allCommands is the registry of top-level cobra commands. Sub-commands
// (e.g. `dck backup enable`, `dck network create`) are attached below
// from a separate path; the cobra command-graph builder constructs them.
var allCommands []*cobra.Command

// register adds a top-level cobra command and appends it to the package
// registry. The wrapper preserves SilenceUsage so legacy functions can
// print their own usage blocks without cobra double-reporting them.
//
// For the `run` command we additionally disable cobra's flag parsing so
// every argument after `dck run` — including `--rm`, `--network`,
// `--memory`, `--cap-add`, `--label`, `--healthcheck-cmd` and 30+ other
// legacy flags — is forwarded verbatim to `Run(args)`, where the legacy
// stdlib `flag.NewFlagSet` already knows how to parse them. Without
// this knob, cobra would reject any run-level flag that is not in its
// own flag set, which historically means every dck invocation that we
// wrote and tested before the cobra migration. Phased migration of
// run flags into cobra is tracked as a follow-up.
func register(spec commandSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		// SilenceUsage prevents cobra from appending a usage block to
		// every error reported from the legacy function (which has its
		// own usage message printing).
		SilenceUsage: true,
		// SilenceErrors so the caller (`Execute`) decides how to surface
		// failures — this gives us uniform ExitCode behaviour for tests.
		SilenceErrors: true,
		Run: func(c *cobra.Command, args []string) {
			spec.run(args)
		},
	}
	if spec.use == "run" || spec.use == "serve" {
		// Skip cobra's unknown-flag detection. PersistentFlags such as
		// --log-level / --json / --quiet cannot be applied to `run`
		// through cobra any more; document the trade-off. The legacy
		// flag.NewFlagSet in Run() implements the same semantics
		// internally because it ignores unknown flags and the legacy
		// command ignores the global log-level flag by design.
		cmd.DisableFlagParsing = true
		// DisableFlagParsing implies cobra.OnInitialize-style hooks do
		// not fire from this subcommand, which is harmless for `run`.
		cmd.Args = cobra.ArbitraryArgs
	}
	allCommands = append(allCommands, cmd)
	return cmd
}

func init() {
	// Image operations
	register(commandSpec{"pull", "Pull an image from a registry", Pull, ""})
	register(commandSpec{"push", "Push an image to a registry", Push, ""})
	register(commandSpec{"images", "List local images", Images, ""})
	register(commandSpec{"verify", "Verify an image manifest and layer digests", Verify, ""})
	register(commandSpec{"search", "Search images on Docker Hub", Search, ""})
	register(commandSpec{"rmi", "Remove an image", Rmi, ""})
	register(commandSpec{"commit", "Create an image from a container", Commit, ""})
	register(commandSpec{"build", "Build an image from a Dockerfile", Build, ""})
	register(commandSpec{"export", "Export an image to a tar.gz archive", Export, ""})

	// Container operations
	register(commandSpec{"run", "Create and run a container", Run, `Usage: dck run [opts] <image> [cmd...]

Resource limits:
  --ram, --memory string     Memory limit (e.g. 512m, 8g)
  --cpu, --cpus float        CPU limit (e.g. 0.5, 2)
  --disk string              Disk limit (e.g. 1G, 512M)

Networking:
  -p, --ports string         Port mapping (host:container[/protocol])
  --network string           Network mode (bridge/host/none/name)
  --dns stringSlice          DNS server (repeatable)

Storage:
  -v, --volume, --vol string Volume mount (src:dst[:ro|rw], repeatable)

Environment:
  -e string                  Env var key=val (repeatable)
  --env-file string          Path to .env file

Identity and security:
  -n string                  Container name
  -h string                  Hostname
  --user string              UID:GID or username
  --cap-add stringSlice      Add capabilities (e.g. NET_ADMIN)
  --cap-drop stringSlice     Drop capabilities (e.g. ALL)
  --readonly                 Read-only rootfs
  --no-new-privs             Block privilege escalation
  --isolated                 Isolate from other containers
  --seccomp-profile string   Seccomp profile path
  --apparmor-profile string  AppArmor profile name

Lifecycle:
  -d                         Detach (background)
  --rm                       Remove on exit
  --restart string           Policy (always|on-failure|unless-stopped)
  --restart-delay string     Delay before restart (e.g. 10s)
  --restart-max-attempts int Max restarts in window
  --restart-window string    Crash-loop window (e.g. 10m)

Execution:
  -it                        Interactive TTY
  --entrypoint string        Override entrypoint
  --workdir string           Working directory
  --cmd, --command string    Command override
  --image string             Container image (alternative to positional)
  --label, -l string         Label key=val (repeatable)
  --sysctl string            Sysctl key=val (repeatable)
  --ulimit string            Ulimit name=soft:hard (repeatable)

Health and startup:
  --healthcheck-cmd string   Health check command
  --healthcheck-interval int Interval (seconds)
  --healthcheck-retries int  Retries
  --healthcheck-timeout int  Timeout (seconds)
  --startup string           Startup script or @filepath

Safety:
  --allow-dangerous-caps     Acknowledge unsafe caps (SYS_ADMIN etc)
  --allow-root               Acknowledge running as UID 0
  --audit-log                Enable audit logging
  --encrypted-backup         Encrypt backup archives

Examples:
  dck run -d --ram 8g --cpu 2 -p 8080:80 --name web nginx
  dck run -it --rm alpine sh
  dck run -d -v /data:/app -e DB_HOST=localhost myapp:latest`})
	register(commandSpec{"start", "Start a stopped container", StartCmd, ""})
	register(commandSpec{"stop", "Stop a running container", Stop, ""})
	register(commandSpec{"restart", "Restart a container", Restart, ""})
	register(commandSpec{"rm", "Remove a container", Rm, ""})
	register(commandSpec{"rename", "Rename a container", Rename, ""})
	register(commandSpec{"set", "Modify container parameters", Set, `Usage: dck set <container> [flags]

Flags:
  --ram, --memory string   Memory limit (e.g. 512m, 8g)
  --cpu, --cpus float      CPU limit (e.g. 1.5)
  --disk string            Disk limit (e.g. 1G)
  --restart string         Restart policy (no|always|on-failure|unless-stopped)
  --restart-delay string   Delay before restart
  -e string                Env var key=val (repeatable)
  --workdir string         Working directory
  --entrypoint string      Override entrypoint
  --user string            UID:GID or username
  --readonly               Read-only rootfs
  --no-new-privs           Block privilege escalation
  -h string                Hostname
  --network string         Network mode (bridge/none/host)

Example:
  dck set myweb --ram 4g --cpu 2 --restart always`})
	psCmd := register(commandSpec{"ps", "List containers", func(args []string) { Ps(args, false) }, ""})
	psCmd.Flags().BoolP("all", "a", false, "Show all containers (running + stopped)")
	psCmd.Run = func(c *cobra.Command, args []string) {
		all, _ := c.Flags().GetBool("all")
		Ps(args, all)
	}
	register(commandSpec{"inspect", "Inspect a container (JSON)", Inspect, ""})
	register(commandSpec{"logs", "Show container logs", Logs, ""})
	register(commandSpec{"stats", "Show CPU/memory/IO statistics", Stats, ""})
	register(commandSpec{"top", "Show running processes", Top, ""})
	register(commandSpec{"info", "System-wide information", Info, ""})

	// Execution
	register(commandSpec{"exec", "Execute a command in a container", Exec, ""})
	register(commandSpec{"console", "Open a web terminal in a container", Console, ""})
	register(commandSpec{"console-serve", "Run the console HTTP backend only", ConsoleServe, ""})
	register(commandSpec{"attach", "Attach to the main container process", Attach, ""})

	// Filesystem
	register(commandSpec{"fs", "Browse container filesystem (ls|cat|tree|find)", Fs, ""})
	register(commandSpec{"cp", "Copy files between host and a container", Cp, ""})

	// Network
	register(commandSpec{"port", "Show or modify port mappings", Port, ""})
	register(commandSpec{"network", "Manage user-defined bridge networks", Network, ""})
	register(commandSpec{"login", "Log in to a registry", Login, ""})
	register(commandSpec{"logout", "Log out from a registry", Logout, ""})
	register(commandSpec{"events", "Stream container events", Events, ""})

	// Backup
	register(commandSpec{"backup", "Manage container backups", Backup, ""})

	// Composition
	register(commandSpec{"up", "Bring up containers from a Compose file", Up, ""})
	register(commandSpec{"down", "Tear down containers from a Compose file", Down, ""})
	register(commandSpec{"blueprint", "Apply a pre-canned blueprint", Blueprint, ""})
	register(commandSpec{"service", "Manage long-running services", Service, ""})
	register(commandSpec{"cluster", "Cluster orchestration commands", Cluster, ""})
	register(commandSpec{"fn", "Run a serverless function", Fn, ""})

	// Diagnostics & lifecycle
	register(commandSpec{"doctor", "Read-only host/runtime diagnostics", Doctor, ""})
	register(commandSpec{"security", "Security-focused diagnostics (alias for 'doctor security')", Security, ""})
	register(commandSpec{"serve", "Run the Docker-compatible HTTP API", Serve, `Usage: dck serve [flags] [on|off|status]

Start the Docker-compatible HTTP API server.

Subcommands:
  dck serve on [--port 2375]   Install as systemd service (auto-start on boot)
  dck serve off                Stop and remove systemd service
  dck serve status             Show service status

Flags:
  -p int                  API port (default 2375)
  -H string               API host (default 127.0.0.1)
  -d                      Run as daemon (foreground by default)
  --token string          Auth token (or DCK_TOKEN env)
  --tls-cert string       TLS certificate file
  --tls-key string        TLS private key file

Examples:
  dck serve                         # foreground, port 2375
  dck serve -p 2376 -d              # daemon, port 2376
  sudo dck serve on -p 2375         # systemd service, auto-start
  sudo dck serve off                # stop and disable service
  dck serve status                  # check if running
  journalctl -u dck-serve -f        # tail logs`})
	register(commandSpec{"supervisor", "Run the container supervisor (foreground)", Supervisor, ""})
	register(commandSpec{"bootstrap", "Install the systemd supervisor unit", Bootstrap, ""})
	register(commandSpec{"update", "Self-update dck", Update, ""})
	register(commandSpec{"version", "Print version information", versionCommand, ""})

	// Volumes & images import
	register(commandSpec{"volume", "Manage named volumes", Volume, ""})
	register(commandSpec{"system", "System-wide maintenance (prune, df)", System, ""})
	register(commandSpec{"init", "Initialize a container's namespaces (internal)", initContainer, ""})

	// Registry allowlist
	register(commandSpec{"registry", "Manage registry allowlist and credentials", Registry, ""})

	// Import (the legacy command was created depending on user vector; it
	// is treated as an image operation).
	register(commandSpec{"import", "Import an image from a tar.gz archive", Import, ""})

	// "init" shadowing on shell completion — provide a hidden alias for
	// back-compat.
	for _, c := range allCommands {
		if c.Use == "init" {
			c.Aliases = append(c.Aliases, "init-container")
		}
	}

	// Sub-command examples: extend `backup` (legacy dispatch already
	// supports `backup create|list|restore|enable|disable|status|verify`)
	// and the security command.
	attachBackupSubcommands()
	attachSecuritySubcommands()
}

// versionCommand is the cobra-shaped implementation of `dck version`.
// It is intentionally a tiny function so unit tests can override it.
func versionCommand(_ []string) {
	fmt.Println("dck version", version)
	fmt.Println("Run 'dck update --check' to check for newer versions.")
}

// attachBackupSubcommands adds explicit cobra sub-commands so that
// `dck backup --help` lists the supported verbs. The legacy `Backup`
// function still dispatches on the first positional argument so the call
// graph is unchanged; this only enriches the help UI.
func attachBackupSubcommands() {
	backup := findCommand("backup")
	if backup == nil {
		return
	}
	for _, sub := range []struct {
		use, short string
	}{
		{"create", "Create a backup archive of a stopped container"},
		{"list", "List existing backups"},
		{"restore", "Restore a stopped container from a backup"},
		{"enable", "Enable scheduled backups for a container"},
		{"disable", "Disable scheduled backups for a container"},
		{"status", "Show scheduled backup status for a container"},
		{"verify", "Verify the SHA-256 checksum of a backup archive"},
	} {
		s := sub
		backup.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			Run: func(c *cobra.Command, args []string) {
				// Legacy dispatch expects `backup <verb> ...`, so we
				// re-join with the parent verb.
				Backup(append([]string{s.use}, args...))
			},
		})
	}
}

// attachSecuritySubcommands adds `dck security check` so users see the
// verb in --help. Same delegation pattern as backup.
func attachSecuritySubcommands() {
	sec := findCommand("security")
	if sec == nil {
		return
	}
	sec.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Run security-focused diagnostics (alias for `dck doctor --strict`)",
		Run: func(c *cobra.Command, args []string) {
			Security(append([]string{"check"}, args...))
		},
	})
}

func findCommand(use string) *cobra.Command {
	for _, c := range allCommands {
		if c.Name() == use {
			return c
		}
	}
	return nil
}
