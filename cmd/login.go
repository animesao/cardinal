//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"cardinal/internal/image"
)

func Login(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	username := fs.String("u", "", "Registry username")
	password := fs.String("p", "", "Registry password")
	passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing login options: %v\n", err)
		os.Exit(1)
	}

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: cardinal login <registry> [-u username] [-p password]")
		os.Exit(1)
	}

	registry := freeArgs[0]
	user := *username
	pass := *password

	if user == "" {
		fmt.Print("Username: ")
		if _, err := fmt.Scanln(&user); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading username: %v\n", err)
			os.Exit(1)
		}
	}
	if pass == "" && !*passwordStdin {
		fmt.Print("Password: ")
		if _, err := fmt.Scanln(&pass); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
	}

	if err := image.Login(registry, user, pass); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Logout(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal logout <registry>")
		os.Exit(1)
	}

	for _, registry := range args {
		if err := image.Logout(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
