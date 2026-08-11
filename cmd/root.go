//go:build linux

package cmd

import (
	"fmt"
	"os"
)

var repoURL = "https://raw.githubusercontent.com/animesao/dck"
var releaseURL = "https://github.com/animesao/dck"
var blueprintRepoURL = "https://raw.githubusercontent.com/dck-organization/dck-blueprints"

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "pull":
		Pull(args)
	case "run":
		Run(args)
	case "ps":
		Ps(args)
	case "inspect":
		Inspect(args)
	case "backup":
		Backup(args)
	case "port":
		Port(args)
	case "network":
		Network(args)
	case "stop":
		Stop(args)
	case "start":
		StartCmd(args)
	case "restart":
		Restart(args)
	case "rm":
		Rm(args)
	case "logs":
		Logs(args)
	case "fs":
		Fs(args)
	case "exec":
		Exec(args)
	case "console":
		Console(args)
	case "console-serve":
		ConsoleServe(args)
	case "attach":
		Attach(args)
	case "init":
		initContainer(args)
	case "stats":
		Stats(args)
	case "volume":
		Volume(args)
	case "images":
		Images(args)
	case "verify":
		Verify(args)
	case "rmi":
		Rmi(args)
	case "update":
		Update(args)
	case "up":
		Up(args)
	case "down":
		Down(args)
	case "bootstrap":
		Bootstrap(args)
	case "supervisor":
		Supervisor(args)
	case "--help", "-h", "help":
		printUsage()
	case "cp":
		Cp(args)
	case "top":
		Top(args)
	case "commit":
		Commit(args)
	case "rename":
		Rename(args)
	case "search":
		Search(args)
	case "set":
		Set(args)
	case "info":
		Info(args)
	case "serve":
		Serve(args)
	case "system":
		System(args)
	case "build":
		Build(args)
	case "push":
		Push(args)
	case "login":
		Login(args)
	case "logout":
		Logout(args)
	case "events":
		Events(args)
	case "export":
		Export(args)
	case "import":
		Import(args)
	case "cluster":
		Cluster(args)
	case "service":
		Service(args)
	case "fn":
		Fn(args)
	case "blueprint":
		Blueprint(args)
	case "version", "--version", "-v":
		fmt.Println("dck version", version)
		fmt.Printf("Run 'dck update --check' to check for newer versions.\n")
	default:
		fmt.Printf("unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`dck - simple container runtime

Usage:
  Image:
    dck pull [--platform] <image>[:tag]    Pull image from registry
    dck push <image>[:tag]                 Push image to registry
    dck images                             List local images
    dck verify <image>[:tag]               Verify local config and layer digests
    dck search <term>                     Search images on Docker Hub
    dck rmi <image>[:tag]                  Remove image
    dck commit <c> <img>[:tag]             Create image from container
    dck build -t name:tag [opts] .         Build image from Dockerfile
    dck export <image> -o f.tar.gz         Save image to file
    dck import <file.tar.gz>               Load image from file

  Container:
    dck run [opts] <image> [cmd]           Create and run container
    dck start <container>                  Start stopped container
    dck stop <container>                   Stop running container
    dck restart <container>                Restart container
    dck rm [-f] <container>                Remove container
    dck rename <c> <new-name>              Rename container
    dck set <c> [opts]                     Change container params
    dck ps [-a]                            List containers
    dck inspect [--sensitive] <container> Show container state as JSON
    dck backup create <container>          Create a container data backup
    dck backup list                        List backups
    dck backup restore <c> <file>          Restore a stopped container backup
    dck backup enable <c> --interval 24h   Enable scheduled backups
    dck backup disable <c>                 Disable scheduled backups
    dck backup status <c>                  Show scheduled backup settings
    dck logs [-f] [--tail <n>] [--previous|--all] <c> Show logs
    dck stats [container]                  CPU, RAM, IO stats
    dck top <container>                    Show running processes
    dck info                               System-wide info

  Network:
    dck network create <name>              Create a user-defined bridge network
    dck network ls                         List user-defined networks
    dck network inspect <name>             Inspect a user-defined network
    dck network rm <name>                  Remove an unused user-defined network
    dck port <container>                   Show port mappings
    dck port add <c> H:C[/p]               Add port mapping
    dck port rm <c> H[/p]                  Remove port mapping
    dck login <registry>                   Log in to registry
    dck logout <registry>                  Log out from registry
    dck events                             Stream container events

  Filesystem:
    dck fs ls <c> [path]                   List files
    dck fs cat <c> <path>                  Show file content
    dck fs tree <c> [path]                 Directory tree
    dck fs find [c] [path] [opts]          Find files
    dck cp <src> <dst>                     Copy files host<->container

  Execution:
    dck exec <container> <cmd>             Run command in container
    dck console <container>                Web terminal in container
    dck attach <container>                 Attach to main process

  Compose:
    dck up [-f config.yml] [service]       Start containers from config
    dck up --generate                      Generate dck.toml from existing containers
    dck down [-f config.yml] [-a] [srv]    Stop/remove from config

  Volumes:
    dck volume create <name>               Create named volume
    dck volume ls                          List volumes
    dck volume rm <name>                   Remove volume
    dck volume inspect <name>              Inspect volume
    dck volume prune                       Remove unused volumes

  Cluster:
    dck cluster init [--serve]             Initialize new cluster
    dck cluster join <peer> [--serve]      Join existing cluster
    dck cluster join-token                 Show connection address for peers
    dck cluster leave                      Leave the cluster
    dck cluster info                       Show cluster overview
    dck cluster ls                         List cluster nodes
    dck cluster node ls                    List nodes with resources
    dck cluster node inspect <id>          Show detailed node info
    dck cluster serve [-p 2375]            Start API server for remote replica requests

  Services:
    dck service create ...                 Create replicated service
    dck service ls                         List services
    dck service rm <name>                  Remove service
    dck service scale <name> N             Scale service
    dck service update <name>              Rolling update

  Functions:
    dck fn deploy                          Deploy serverless function
    dck fn ls                              List functions
    dck fn rm <name>                       Remove function
    dck fn call <name>                     Invoke function

  Blueprints:
    dck blueprint list                     List available blueprints
    dck blueprint info <name>              Show blueprint details + example
    dck blueprint install <name>           Install a blueprint
    dck blueprint repo add <url>           Add blueprint repository
    dck blueprint repo list                List repositories

  System:
    dck inspect [--sensitive] <container> Show container state as JSON
    dck backup create <container>          Create a container data backup
    dck backup list                        List backups
    dck backup restore <c> <file>          Restore a stopped container backup
    dck backup enable <c> --interval 24h   Enable scheduled backups
    dck backup disable <c>                 Disable scheduled backups
    dck backup status <c>                  Show scheduled backup settings
    dck serve [-p 2375]                    Start REST API server
    dck system prune                       Clean up unused resources
    dck update [--check]                   Check for updates and self-update
    dck bootstrap [--install|--remove]     Auto-start containers on boot
    dck supervisor                         Run persistent restart supervisor
    dck version, --version, -v             Show version
    dck --help, -h, help                   Show this help

Run options:
  -d              Detach (background)                                    e.g. -d
  -n <name>       Container name                                         e.g. -n myapp
  -p H:C[/proto]  Port mapping (host:container/tcp|udp)                  e.g. -p 8080:80, -p 53:53/udp
  --ports H:C     Port mapping (alias for -p)                            e.g. --ports 8080:80
  -v S:D[:mode]   Volume mount (src:dst[:ro|rw])                          e.g. -v /data:/data:ro
  --volume S:D    Volume mount (alias for -v)                             e.g. --volume /data:/data:ro
  --vol S:D       Volume mount (alias for -v)                             e.g. --vol myvol:/data:rw
  -e K=V          Environment variable                                   e.g. -e DB_HOST=localhost
  --env-file <f>  Read environment variables from file                    e.g. --env-file .env
  -i              Interactive                                            e.g. -i
  -t              Allocate TTY                                           e.g. -t
  --rm            Remove on exit                                         e.g. --rm
  --restart       Restart policy (no|always|on-failure|unless-stopped)    e.g. --restart always
  --restart-delay  Wait before automatic restart                          e.g. --restart-delay 1m
  --restart-max-attempts  Stop crash-loop after N retries                  e.g. --restart-max-attempts 5
  --restart-window  Count retries within this duration                     e.g. --restart-window 10m
  --memory <lim>  Memory limit                                           e.g. --memory 1g, --memory 512m
  --ram <lim>     Memory limit (alias for --memory)                      e.g. --ram 2g
  --cpus <num>    CPU limit                                              e.g. --cpus 2, --cpus 0.5
  --cpu <num>     CPU limit (alias for --cpus)                           e.g. --cpu 1.5
  --disk <lim>    Disk limit                                             e.g. --disk 10G
  --workdir <dir> Working directory inside container                     e.g. --workdir /app
  -h <name>       Container hostname                                     e.g. -h myserver
  --entrypoint    Override image entrypoint                              e.g. --entrypoint /bin/bash
  --image <img>   Container image (instead of positional arg)            e.g. --image nginx:alpine
  --cmd <cmd>     Container command (instead of positional args)         e.g. --cmd "python app.py"
  --command <cmd> Container command (alias for --cmd)                    e.g. --command "java -jar server.jar"
  --cap-add       Add Linux capabilities (can repeat)                    e.g. --cap-add NET_ADMIN
  --cap-drop      Drop Linux capabilities (can repeat)                   e.g. --cap-drop ALL
  --user <uid>    Username or UID:GID                                    e.g. --user 1000:1000
  --readonly      Make rootfs read-only                                  e.g. --readonly
  --no-new-privs  Disable acquiring new privileges                       e.g. --no-new-privs
  --sysctl <k=v>  Sysctl options (can repeat)                            e.g. --sysctl net.ipv4.ip_forward=1
  --ulimit <opt>  Ulimit options (name=soft:hard)                        e.g. --ulimit nofile=1024:2048
  -l, --label     Container labels (key=val)                             e.g. -l env=prod
  --dns <ip>      DNS server (can repeat)                                e.g. --dns 8.8.8.8
  --network <m>   Network mode or user network name (bridge|none|host|name) e.g. --network appnet
  --startup <s>   Startup script (inline or @file)                       e.g. --startup @setup.sh
  --healthcheck-cmd <cmd>      Health check command                      e.g. --healthcheck-cmd "curl -f http://localhost"
  --healthcheck-interval <s>   Health check interval (seconds)           e.g. --healthcheck-interval 30
  --healthcheck-retries <n>    Health check retries                      e.g. --healthcheck-retries 5
  --healthcheck-timeout <s>    Health check timeout (seconds)            e.g. --healthcheck-timeout 10`)
}
