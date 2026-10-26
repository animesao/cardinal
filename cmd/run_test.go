//go:build linux

package cmd

import (
	"flag"
	"testing"
)

func TestReorderRunFlagsAfterImage(t *testing.T) {
	fs := flag.NewFlagSet("run-test", flag.ContinueOnError)
	workdir := fs.String("workdir", "", "working directory")
	name := fs.String("n", "", "container name")

	args := reorderRunFlags([]string{"nginx:alpine", "--workdir", "web", "--n", "web"}, fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse reordered args: %v", err)
	}

	if *workdir != "web" {
		t.Fatalf("workdir = %q, want %q", *workdir, "web")
	}
	if *name != "web" {
		t.Fatalf("name = %q, want %q", *name, "web")
	}
	if got := fs.Args(); len(got) != 1 || got[0] != "nginx:alpine" {
		t.Fatalf("positional args = %#v, want [nginx:alpine]", got)
	}
}

// Regression for the e2e bridge-forward smoke test: container command flags
// such as -p and -h must not be reinterpreted as cardinal options once the
// container command has started.
func TestReorderRunFlagsKeepsContainerCommandVerbatim(t *testing.T) {
	fs := flag.NewFlagSet("run-test", flag.ContinueOnError)
	detach := fs.Bool("d", false, "detach")
	name := fs.String("n", "", "container name")
	port := fs.String("p", "", "port mapping")
	hostname := fs.String("h", "", "hostname")

	in := []string{"-d", "-n", "e2e-bridge", "-p", "18083:80", "alpine:latest", "httpd", "-f", "-p", "80", "-h", "/"}
	if err := fs.Parse(reorderRunFlags(in, fs)); err != nil {
		t.Fatalf("parse reordered args: %v", err)
	}

	if !*detach || *name != "e2e-bridge" || *port != "18083:80" {
		t.Fatalf("cardinal flags = detach %v, name %q, port %q", *detach, *name, *port)
	}
	if *hostname != "" {
		t.Fatalf("hostname = %q, want empty; container -h must not be consumed", *hostname)
	}

	want := []string{"alpine:latest", "httpd", "-f", "-p", "80", "-h", "/"}
	got := fs.Args()
	if len(got) != len(want) {
		t.Fatalf("positional args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("positional args = %#v, want %#v", got, want)
		}
	}
}

func TestReorderRunFlagsImageFlagThenCommand(t *testing.T) {
	fs := flag.NewFlagSet("run-test", flag.ContinueOnError)
	image := fs.String("image", "", "container image")
	name := fs.String("n", "", "container name")

	in := []string{"--image", "alpine:latest", "-n", "web", "sh", "-c", "echo hi"}
	if err := fs.Parse(reorderRunFlags(in, fs)); err != nil {
		t.Fatalf("parse reordered args: %v", err)
	}

	if *image != "alpine:latest" || *name != "web" {
		t.Fatalf("image = %q, name = %q", *image, *name)
	}
	want := []string{"sh", "-c", "echo hi"}
	got := fs.Args()
	if len(got) != len(want) {
		t.Fatalf("positional args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("positional args = %#v, want %#v", got, want)
		}
	}
}
