//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/image"
)

func Login(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	username := fs.String("u", "", "Registry username")
	password := fs.String("p", "", "Registry password")
	passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
	fs.Parse(args)

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: dck login <registry> [-u username] [-p password]")
		os.Exit(1)
	}

	registry := freeArgs[0]
	user := *username
	pass := *password

	if user == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&user)
	}
	if pass == "" && !*passwordStdin {
		fmt.Print("Password: ")
		fmt.Scanln(&pass)
	}

	if err := image.Login(registry, user, pass); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Logout(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dck logout <registry>")
		os.Exit(1)
	}

	for _, registry := range args {
		if err := image.Logout(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
