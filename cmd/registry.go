//go:build linux

package cmd

import (
	"fmt"
	"os"

	"dck/internal/image"
)

// Registry handles "dck registry ..." subcommands.
//
// We deliberately reuse the existing Login/Logout plumbing from cmd/login.go
// for credential management; the registry allowlist is a new concept that
// layer on top of it. Both live under the same command family so a user
// only has to learn one name.
func Registry(args []string) {
	// Default to "allowlist list" so a bare run prints the configured
	// registries, mirroring how `dck network` behaves.
	if len(args) == 0 {
		listAllowlist()
		return
	}

	switch args[0] {
	case "allowlist", "list":
		if len(args) > 1 && args[1] == "list" {
			listAllowlist()
			return
		}
		// subcommand was already "allowlist"; fall through to the list view.
		listAllowlist()
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: dck registry add <hostname>")
			os.Exit(2)
		}
		if err := image.AddToAllowlist(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "registry add: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added %s to the registry allowlist\n", args[1])
		fmt.Println("Strict allowlist is enforced only when DCK_REGISTRY_STRICT=1 is set.")
	case "remove", "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: dck registry remove <hostname>")
			os.Exit(2)
		}
		if err := image.RemoveFromAllowlist(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "registry remove: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Removed %s from the registry allowlist\n", args[1])
	case "login":
		Login(args[1:])
	case "logout":
		Logout(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown registry subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: dck registry [allowlist|add|remove|login|logout]")
		os.Exit(2)
	}
}

func listAllowlist() {
	if err := image.InitRegistryAllowlist(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not reload allowlist: %v\n", err)
	}
	hosts := image.AllowlistSnapshot()
	if len(hosts) == 0 {
		fmt.Println("No registries in the allowlist. Use 'dck registry add <host>' to add one.")
		return
	}
	fmt.Println("Allowed registries:")
	for _, h := range hosts {
		fmt.Printf("  %s\n", h)
	}
	mode := "permissive (allowlist ignored unless DCK_REGISTRY_STRICT=1)"
	if os.Getenv("DCK_REGISTRY_STRICT") == "1" {
		mode = "STRICT (allowlist enforced)"
	}
	fmt.Printf("\nMode: %s\n", mode)
}
