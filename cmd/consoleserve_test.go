//go:build linux

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Typed console input must reach the container unchanged while a "> cmd"
// copy lands in the log: otherwise reconnect replays restore answers
// without the commands that produced them.
func TestConsoleInputLogger(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ctr.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logFile.Close() }()

	var mu sync.Mutex
	var forwarded bytes.Buffer
	l := &consoleInputLogger{w: &forwarded, mu: &mu, log: logFile}

	// Split mid-line on purpose: the logger must reassemble, not emit halves.
	for _, chunk := range []string{"tp", "s\nsay hel", "lo\n\n", "stop\n"} {
		if _, err := l.Write([]byte(chunk)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if got := forwarded.String(); got != "tps\nsay hello\n\nstop\n" {
		t.Fatalf("container stdin altered: %q", got)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"> tps\n", "> say hello\n", "> stop\n"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("log missing %q, got:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "\n\n") {
		t.Errorf("blank input lines leaked into log:\n%s", data)
	}
}
