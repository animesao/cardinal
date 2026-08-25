//go:build linux

package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"cardinal/internal/orchestrator"
)

func Fn(args []string) {
	if len(args) < 1 {
		printFnUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "deploy":
		fnDeploy(subargs)
	case "ls", "list":
		fnList(subargs)
	case "rm", "remove":
		fnRemove(subargs)
	case "call":
		fnCall(subargs)
	default:
		fmt.Printf("unknown function command: %s\n", subcommand)
		printFnUsage()
		os.Exit(1)
	}
}

func printFnUsage() {
	fmt.Println(`Usage: cardinal fn COMMAND

Manage serverless functions

Commands:
  deploy          Deploy a function
  ls              List functions
  rm              Remove a function
  call            Invoke a function`)
}

func fnDeploy(args []string) {
	fs := flag.NewFlagSet("fn deploy", flag.ExitOnError)
	name := fs.String("name", "", "Function name")
	port := fs.Int("port", 8080, "Container port")
	handler := fs.String("handler", "/handler", "Handler path")
	timeout := fs.Int("timeout", 30, "Execution timeout (s)")
	idle := fs.Int("idle", 300, "Idle timeout before scale-to-zero (s)")
	memory := fs.String("memory", "", "Memory limit")
	cpus := fs.Float64("cpus", 0, "CPU limit")
	warm := fs.Int("warm", 0, "Warm replicas to keep")

	var envVars stringSlice
	fs.Var(&envVars, "e", "Environment variables")
	fs.Var(&envVars, "env", "Environment variables")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing fn deploy options: %v\n", err)
		os.Exit(1)
	}

	image := ""
	freeArgs := fs.Args()
	if len(freeArgs) > 0 {
		image = freeArgs[0]
	}

	if *name == "" || image == "" {
		fmt.Println("Usage: cardinal fn deploy --name <name> [--port N] [--timeout N] [--idle N] <image>")
		os.Exit(1)
	}

	env := make(map[string]string)
	for _, e := range envVars {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 2 {
			env[kv[0]] = kv[1]
		}
	}

	opts := orchestrator.FnOpts{
		Handler:     *handler,
		Env:         env,
		Timeout:     *timeout,
		IdleTimeout: *idle,
		Memory:      *memory,
		CPUs:        *cpus,
		Replicas:    *warm,
	}

	fn, err := orchestrator.DeployFunction(context.Background(), *name, image, *port, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deployed function %s\n", fn.Name)
	fmt.Printf("  Image: %s\n", fn.Image)
	fmt.Printf("  Port: %d\n", fn.Port)
	fmt.Printf("  Timeout: %ds\n", fn.Timeout)
	fmt.Printf("  Idle timeout: %ds\n", fn.IdleTimeout)
	fmt.Printf("  Warm replicas: %d\n", fn.Replicas)
}

func fnList(args []string) {
	fns, err := orchestrator.ListFunctions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(fns) == 0 {
		fmt.Println("No functions deployed")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tIMAGE\tPORT\tTIMEOUT\tIDLE\tWARM\tINVOKES"); err != nil {
		return
	}
	for _, f := range fns {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%ds\t%ds\t%d\t%d\n",
			f.Name, f.Image, f.Port,
			f.Timeout, f.IdleTimeout,
			f.Replicas, f.InvokeCount); err != nil {
			return
		}
	}
	_ = w.Flush()
}

func fnRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal fn rm <name> [<name>...]")
		os.Exit(1)
	}

	for _, name := range args {
		if err := orchestrator.RemoveFunction(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing function %q: %v\n", name, err)
		} else {
			fmt.Printf("Removed function %s\n", name)
		}
	}
}

func fnCall(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal fn call <name> [--data <payload>]")
		os.Exit(1)
	}

	name := args[0]
	payload := []byte{}

	// Check for --data
	for i := 1; i < len(args); i++ {
		if args[i] == "--data" || args[i] == "-d" {
			if i+1 < len(args) {
				payload = []byte(args[i+1])
			}
		}
	}

	// If stdin has data, read it
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		stdinData, _ := io.ReadAll(os.Stdin)
		if len(stdinData) > 0 {
			payload = stdinData
		}
	}

	result, err := orchestrator.InvokeFunction(context.Background(), name, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Try to pretty-print JSON response
	var pretty interface{}
	if err := json.Unmarshal(result, &pretty); err == nil {
		formatted, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(formatted))
	} else {
		fmt.Print(string(result))
	}
}
