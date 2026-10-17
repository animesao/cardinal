//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"cardinal/internal/image"
)

// readPasswordNoEcho reads one line from the terminal without echoing it
// (no new dependency: golang.org/x/sys is already vendored).
func readPasswordNoEcho(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", err
	}
	newState := *oldState
	newState.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return "", err
	}
	defer func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState) }()
	var pass string
	if _, err := fmt.Scanln(&pass); err != nil {
		fmt.Println()
		return "", err
	}
	fmt.Println()
	return pass, nil
}

func Login(args []string) {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	username := fs.String("u", "", "Registry username")
	password := fs.String("p", "", "Registry password")
	passwordStdin := fs.Bool("password-stdin", false, "Read password from stdin")
	mustParse(fs, args, "login")

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: cardinal login <registry> [-u username] [-p password]")
		exitFunc(1)
	}

	registry := freeArgs[0]
	user := *username
	pass := *password
	if pass != "" {
		fmt.Fprintln(os.Stderr, "Warning: -p exposes the password in shell history and `ps` output; prefer --password-stdin or interactive input")
	}

	if user == "" {
		fmt.Print("Username: ")
		if _, err := fmt.Scanln(&user); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading username: %v\n", err)
			exitFunc(1)
		}
	}
	if pass == "" && !*passwordStdin {
		var err error
		if pass, err = readPasswordNoEcho("Password: "); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			exitFunc(1)
		}
	}

	if err := image.Login(registry, user, pass); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}

func Logout(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: cardinal logout <registry>")
		exitFunc(1)
	}

	for _, registry := range args {
		if err := image.Logout(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
