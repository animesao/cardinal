//go:build linux

// Package cmd hosts the cobra command tree for dck.
//
// Cobra replaces the legacy hand-rolled dispatcher in root.go (a single
// 130-line switch statement) so we get free shell completion, structured
// help, and uniform --json / --quiet / --log-level global flags. Each
// existing `func X(args []string)` in this package is wired into cobra as
// the `Run` action of its sub-command; this preserves the behaviour of
// every previously-working invocation while gaining the standard CLI UX.
//
// The package-internal `Execute` function is called from main.go.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"dck/internal/log"
)

// NewRoot constructs the cobra root command. Splitting construction from
// execution makes unit testing trivial: tests can build the tree with a
// custom IO buffer and assert on the produced output.
func NewRoot() *cobra.Command {
	// Global options surfaced on the root command so every sub-command
	// can inspect them via cmd.Root().PersistentFlags() lookups.
	var (
		logLevel string
		jsonOut  bool
		quiet    bool
	)

	rootCmd := &cobra.Command{
		Use:   "dck",
		Short: "Lightweight Linux container runtime",
		Long: `dck — Lightweight Linux container runtime.

A daemonless, OCI-compatible runtime for Linux that uses namespaces,
overlayfs, cgroups, capability dropping and seccomp filtering to isolate
untrusted workloads. Mirrors the docker CLI surface where it is useful,
and adds tooling for cluster orchestration, FaaS, blueprints and
Docker-Compose-style up/down/up commands.

GLOBAL FLAGS:
  --log-level string   Log verbosity (debug|info|warn|error) (default "info")
  --json               Emit machine-readable JSON output
  --quiet              Suppress non-essential output

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
dck run — FULL FLAG REFERENCE
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

RESOURCE LIMITS:
  --ram, --memory string     Memory limit (e.g. 512m, 8g)
  --cpu, --cpus float        CPU limit (e.g. 0.5, 2)
  --disk string              Disk limit (e.g. 1G, 512M)

NETWORKING:
  -p, --ports string         Port mapping (host:container[/protocol])
  --network string           Network mode (bridge/host/none/name)
  --dns stringSlice          DNS server (repeatable)

STORAGE:
  -v, --volume, --vol string Volume mount (src:dst[:ro|rw], repeatable)

ENVIRONMENT:
  -e string                  Env var key=val (repeatable)
  --env-file string          Path to .env file

IDENTITY & SECURITY:
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

LIFECYCLE:
  -d                         Detach (background)
  --rm                       Remove on exit
  --restart string           Policy (always|on-failure|unless-stopped)
  --restart-delay string     Delay before restart (e.g. 10s)
  --restart-max-attempts int Max restarts in window
  --restart-window string    Crash-loop window (e.g. 10m)

EXECUTION:
  -it                        Interactive TTY
  --entrypoint string        Override entrypoint
  --workdir string           Working directory
  --cmd, --command string    Command override
  --image string             Container image (alternative to positional)
  --label, -l string         Label key=val (repeatable)
  --sysctl string            Sysctl key=val (repeatable)
  --ulimit string            Ulimit name=soft:hard (repeatable)

HEALTH & STARTUP:
  --healthcheck-cmd string   Health check command
  --healthcheck-interval int Interval (seconds)
  --healthcheck-retries int  Retries
  --healthcheck-timeout int  Timeout (seconds)
  --startup string           Startup script or @filepath

SAFETY:
  --allow-dangerous-caps     Acknowledge unsafe caps (SYS_ADMIN etc)
  --allow-root               Acknowledge running as UID 0
  --audit-log                Enable audit logging
  --encrypted-backup         Encrypt backup archives

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
dck set — MODIFY RUNNING CONTAINER
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

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

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
QUICK REFERENCE
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  IMAGES        pull push images rmi build export import commit search verify
  CONTAINERS    run start stop restart rm rename ps inspect logs stats top attach
  EXECUTION     exec console cp fs
  NETWORKING    network port
  VOLUMES       volume
  COMPOSE       up down
  API SERVER    serve (on/off/status)
  REGISTRY      login logout registry
  BACKUPS       backup
  CLUSTER       cluster
  SERVICES      service fn
  DIAGNOSTICS   doctor security info events system
  LIFECYCLE     bootstrap update version

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXAMPLES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  dck run -d --ram 8g --cpu 2 -p 8080:80 --name web nginx
  dck run -it --rm alpine sh
  dck run -d -v /data:/app -e DB_HOST=localhost --network mynet myapp

  sudo dck serve on -p 2375
  dck serve status
  sudo dck serve off

  dck up -f dck.toml
  dck down -f dck.toml

  dck backup create web -o /backup/web.tar.gz -e
  dck backup enable web --interval 24h --retention 7

  dck set web --ram 16g --cpu 4 --restart on-failure
  dck logs -f --tail 50 web
  dck cp ./config.yaml web:/app/config.yaml

  dck cluster init --name prod --port 7946
  dck fn deploy --name api --port 8080 --handler /handler`,
		Version:      version,
		SilenceUsage: true,
		// Run the help when no args are provided; the legacy root used to
		// print help on `dck` with no args and exit 1, but `SilenceUsage`
		// plus the explicit Run keeps the same UX without an extra exit
		// code on the success path.
		RunE: func(cmd *cobra.Command, args []string) error {
			applyLogOptions(logLevel, jsonOut, quiet)
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyLogOptions(logLevel, jsonOut, quiet)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log verbosity (debug|info|warn|error)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output where supported")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")

	// All commands are registered via init() in this package so the order
	// does not depend on file compilation order.
	for _, c := range allCommands {
		rootCmd.AddCommand(c)
	}
	return rootCmd
}

// applyLogOptions funnels global logging flags into the log package. The
// package-level state is sufficient because dck is a single-process CLI
// (no concurrent goroutines that need to race on the logger).
func applyLogOptions(level string, jsonOut, quiet bool) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.LevelDebug)
	case "warn", "warning":
		log.SetLevel(log.LevelWarn)
	case "error", "err":
		log.SetLevel(log.LevelError)
	default:
		log.SetLevel(log.LevelInfo)
	}
	if quiet {
		log.SetLevel(log.LevelError)
	}
	log.SetJSON(jsonOut)
}

// Execute is the entry point invoked from main.go. It sets up the cobra
// command tree and runs it against the process arguments.
func Execute() {
	cmd := NewRoot()
	if err := cmd.Execute(); err != nil {
		// Every registered sub-command sets SilenceErrors=true so legacy
		// command implementations can surface errors in their own way
		// (most of them call os.Exit(1) directly after printing to
		// stderr). That setting also causes cobra NOT to print the
		// error itself, so we must replicate that printing here — without
		// this line, an unknown flag leaves the user with an empty
		// stderr and exit code 1, which is impossible to debug.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
