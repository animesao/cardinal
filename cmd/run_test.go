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
