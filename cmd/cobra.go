//go:build linux

// Package cmd hosts the cobra command tree for cardinal.
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

	"cardinal/internal/log"
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
		Use:   "cardinal",
		Short: "Lightweight Linux container runtime",
		Long: `cardinal — Lightweight Linux container runtime.

A daemonless, OCI-compatible runtime for Linux that uses namespaces,
overlayfs, cgroups, capability dropping and seccomp filtering to isolate
untrusted workloads. Mirrors the docker CLI surface where it is useful,
and adds tooling for cluster orchestration, FaaS, blueprints and
Docker-Compose-style up/down/up commands.

	GLOBAL FLAGS:
  --log-level string   Log verbosity (debug|info|warn|error) (default "info")
  --json               Emit machine-readable JSON output
  --quiet              Suppress non-essential output
  --host, -H string    Remote cardinal serve URL for read-only remote mode (or CARDINAL_REMOTE_HOST)
  --token string       Bearer token for the remote (or CARDINAL_TOKEN)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
IMAGE COMMANDS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal pull <image>              Pull an image (--platform linux/amd64)
  cardinal push <image>              Push an image (-u user -p pass)
  cardinal images                    List local images
  cardinal rmi <image>               Remove an image
  cardinal build -t name:tag .       Build from Dockerfile (-f, --no-cache, --build-arg)
  cardinal export -o file.tar.gz c   Export image to tar.gz
  cardinal import file.tar.gz        Import image from tar.gz
  cardinal commit <container>        Create image from container
  cardinal search <query>            Search Docker Hub
  cardinal verify <image>            Verify image manifest and digests

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CONTAINER COMMANDS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal run [opts] <image> [cmd]  Create and run a container (see below)
  cardinal start <container>         Start a stopped container
  cardinal stop <container>          Stop a running container (--all for all)
  cardinal restart <container>       Restart a container
  cardinal rm <container>            Remove a container (-f to force)
  cardinal rename <old> <new>        Rename a container
  cardinal ps                        List containers (-a for all)
  cardinal inspect <container>       Inspect container JSON (--sensitive for secrets)
  cardinal logs <container>          Show logs (-f follow, --tail N, --previous)
  cardinal stats                     Show CPU/memory/IO (--no-stream)
  cardinal top <container>           Show running processes
  cardinal attach <container>        Attach to main process

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXECUTION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal exec -it <container> sh   Execute command in container
  cardinal console <container>       Open web terminal
  cardinal cp <src> <dst>            Copy files between host and container
  cardinal fs ls <container> <path>  List files in container
  cardinal fs cat <container> <file> Read file in container
  cardinal fs tree <container> <dir> Show directory tree
  cardinal fs find <container>       Find files (--name, --grep, --type f/d)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
NETWORKING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal network create <name>     Create bridge network (--subnet)
  cardinal network ls                List networks
  cardinal network inspect <name>    Inspect network
  cardinal network rm <name>         Remove network
  cardinal port <container>          Show/modify port mappings

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
VOLUMES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal volume create <name>      Create volume (-d driver, -l label)
  cardinal volume ls                 List volumes
  cardinal volume inspect <name>     Inspect volume
  cardinal volume rm <name>          Remove volume
  cardinal volume prune              Remove unused volumes

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
COMPOSE
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal up -f cardinal.toml            Start from config file
  cardinal down -f cardinal.toml          Stop and remove (-a for all)
  cardinal up --generate             Generate cardinal.toml from containers

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
API SERVER
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal serve                     Run API server foreground (-p port)
  cardinal serve -d                  Run as daemon
  cardinal serve on -p 2375          Install systemd service (auto-start on boot)
  cardinal serve off                 Stop and remove systemd service
  cardinal serve status              Check service status
  journalctl -u cardinal-serve -f    Tail logs

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
REGISTRY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal login -u user -p pass     Log in to registry (--password-stdin)
  cardinal logout                    Log out
  cardinal registry add <host>       Add to registry allowlist
  cardinal registry rm <host>        Remove from allowlist
  cardinal registry list             List allowlist

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
BACKUPS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal backup create <name>      Create backup (-o path, -e encrypt)
  cardinal backup list               List backups
  cardinal backup restore <name>     Restore from backup
  cardinal backup enable <name>      Enable scheduled backup (--interval, --retention)
  cardinal backup disable <name>     Disable scheduled backup
  cardinal backup status <name>      Show backup status
  cardinal backup verify <file>      Verify SHA-256 checksum
  cardinal backup generate-key       Generate encryption key

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CLUSTER
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal cluster init              Initialize cluster (--name, --bind, --port)
  cardinal cluster join              Join cluster (--bind, --port, --token)
  cardinal cluster leave             Leave cluster
  cardinal cluster info              Cluster overview
  cardinal cluster ls                List nodes
  cardinal cluster serve             Start cluster API (-p, -H, --token)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
SERVICES & FAAS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal service create            Create service (--name, --replicas, -p, -e)
  cardinal service ls                List services
  cardinal service rm <name>         Remove service
  cardinal service scale <name> N    Scale to N replicas
  cardinal service update            Update service (--name, --image)
  cardinal fn deploy                 Deploy function (--name, --port, --handler)
  cardinal fn ls                     List functions
  cardinal fn rm <name>              Remove function
  cardinal fn call <name>            Call function (--data/-d)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
DIAGNOSTICS & SYSTEM
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal doctor                    Host/runtime diagnostics (--strict)
  cardinal security                  Security diagnostics
  cardinal info                      System information
  cardinal events                    Stream events (--since)
  cardinal system df                 Show disk usage by images, containers, volumes
  cardinal system prune              Remove unused containers, images and volumes
  cardinal version                   Print version
  cardinal update                    Self-update (--check for dry run)
  cardinal bootstrap                 Install systemd unit (--install/-i, --remove/-r)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
MODIFY RUNNING CONTAINER
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  cardinal set <container> [flags]   Modify parameters (see below)

` + runLongHelp + `

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
cardinal set — FLAG REFERENCE (see "cardinal set --help")
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

` + setLongHelp + `

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXAMPLES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  cardinal run -d --ram 8g --cpu 2 -p 8081:80 --name web nginx
  cardinal run -it --rm alpine sh
  cardinal run -d -v /data:/app -e DB_HOST=localhost --network mynet myapp

  sudo cardinal serve on -p 2375
  cardinal serve status
  sudo cardinal serve off

  cardinal up -f cardinal.toml
  cardinal down -f cardinal.toml

  cardinal backup create web -o /backup/web.tar.gz -e
  cardinal backup enable web --interval 24h --retention 7

  cardinal set web --ram 16g --cpu 4 --restart on-failure
  cardinal logs -f --tail 50 web
  cardinal cp ./config.yaml web:/app/config.yaml

  cardinal cluster init --name prod --port 7946
  cardinal fn deploy --name api --port 8080 --handler /handler`,
		Version:      version,
		SilenceUsage: true,
		// Disable cobra's auto-generated "Available Commands" and "Flags"
		// sections so our custom Long text is the only help output.
		DisableAutoGenTag: true,
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
	rootCmd.PersistentFlags().StringVar(&remoteHost, "host", "", "Remote `cardinal serve` URL (or CARDINAL_REMOTE_HOST)")
	rootCmd.PersistentFlags().StringVarP(&remoteHost, "H", "H", "", "Shorthand for --host")
	rootCmd.PersistentFlags().StringVar(&remoteToken, "token", "", "Bearer token for the remote (or CARDINAL_TOKEN)")

	// All commands are registered via init() in this package so the order
	// does not depend on file compilation order.
	for _, c := range allCommands {
		rootCmd.AddCommand(c)
	}
	return rootCmd
}

// applyLogOptions funnels global logging flags into the log package. The
// package-level state is sufficient because cardinal is a single-process CLI
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
