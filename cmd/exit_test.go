//go:build linux

package cmd

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

// stubExit replaces exitFunc with a recorder and returns the recorder plus
// a restore function. Commands under test then capture exit codes instead
// of terminating the test binary.
func stubExit(t *testing.T) (*int, func()) {
	t.Helper()
	got := -1
	old := exitFunc
	exitFunc = func(code int) { got = code }
	return &got, func() { exitFunc = old }
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	_ = w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestMustParse_ValidFlags(t *testing.T) {
	got, restore := stubExit(t)
	defer restore()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	all := fs.Bool("all", false, "")
	out := captureStderr(t, func() { mustParse(fs, []string{"--all"}, "test") })
	if *got != -1 {
		t.Fatalf("exit code = %d; want no exit", *got)
	}
	if !*all {
		t.Fatalf("--all was not parsed")
	}
	if out != "" {
		t.Fatalf("unexpected stderr output: %q", out)
	}
}

func TestMustParse_BadFlagExitsOne(t *testing.T) {
	got, restore := stubExit(t)
	defer restore()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("all", false, "")
	out := captureStderr(t, func() { mustParse(fs, []string{"--bogus"}, "test") })
	if *got != 1 {
		t.Fatalf("exit code = %d; want 1", *got)
	}
	if !strings.Contains(out, "Error parsing test options") {
		t.Fatalf("stderr = %q; want Error parsing test options", out)
	}
}

func TestMustParse_HelpExitsZeroWithUsage(t *testing.T) {
	got, restore := stubExit(t)
	defer restore()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("all", false, "stop everything")
	out := captureStderr(t, func() { mustParse(fs, []string{"--help"}, "test") })
	if *got != 0 {
		t.Fatalf("exit code = %d; want 0", *got)
	}
	if !strings.Contains(out, "Usage of test") {
		t.Fatalf("stderr = %q; want flag usage", out)
	}
}

func TestFailf_PrintsAndExitsOne(t *testing.T) {
	got, restore := stubExit(t)
	defer restore()
	out := captureStderr(t, func() { failf("boom %d", 42) })
	if *got != 1 {
		t.Fatalf("exit code = %d; want 1", *got)
	}
	if !strings.Contains(out, "Error: boom 42") {
		t.Fatalf("stderr = %q; want Error: boom 42", out)
	}
}

func TestStripGlobalFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, []string{}},
		{"json stripped", []string{"--json", "web"}, []string{"web"}},
		{"quiet stripped", []string{"-f", "--quiet"}, []string{"-f"}},
		{"log-level separate", []string{"--log-level", "debug", "web"}, []string{"web"}},
		{"log-level equals", []string{"--log-level=warn", "web"}, []string{"web"}},
		{"log-level unknown value kept prefix-stripped", []string{"--log-level=verbose", "web"}, []string{"web"}},
		{"command flags kept", []string{"--all", "--tail", "5"}, []string{"--all", "--tail", "5"}},
		{"similar prefix kept", []string{"--json-file", "x"}, []string{"--json-file", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stripGlobalFlags(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("stripGlobalFlags(%q) = %q; want %q", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("stripGlobalFlags(%q) = %q; want %q", tc.in, got, tc.want)
				}
			}
		})
	}
}

func TestHasLongHelpArgument(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want bool
	}{
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"help"}, true},
		{[]string{"web", "--help"}, true},
		{[]string{"web"}, false},
		{[]string{"--hel"}, false},
		{[]string{"--json"}, false},
		{nil, false},
	} {
		if got := hasLongHelpArgument(tc.in); got != tc.want {
			t.Errorf("hasLongHelpArgument(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
