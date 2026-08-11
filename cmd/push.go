//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"

	"dck/internal/image"
)

func Push(args []string) {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	username := fs.String("u", "", "Registry username")
	password := fs.String("p", "", "Registry password")

	fs.Parse(args)

	freeArgs := fs.Args()
	if len(freeArgs) < 1 {
		fmt.Println("Usage: dck push [-u username] [-p password] <image>[:<tag>]")
		fmt.Println("  -u username  Registry username (or DOCKER_USERNAME env)")
		fmt.Println("  -p password  Registry password (or DOCKER_PASSWORD env)")
		os.Exit(1)
	}

	ref := freeArgs[0]

	user := *username
	pass := *password

	if user == "" {
		user = os.Getenv("DOCKER_USERNAME")
	}
	if pass == "" {
		pass = os.Getenv("DOCKER_PASSWORD")
	}

	if user == "" || pass == "" {
		fmt.Fprintf(os.Stderr, "Warning: no credentials provided. Push to Docker Hub may fail for private repos.\n")
	}

	if err := image.Push(ref, user, pass); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
