//go:build linux

package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogger_JSONModeProtectsAgainstLogInjection(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, &buf, LevelDebug)
	l.SetJSON(true)

	// Newline + a fake log line in one message. If the logger were to print
	// this verbatim into a JSON line, an attacker who controls `format` (CARDINAL
	// info-level messages sometimes include user-supplied values) could
	// inject arbitrary sibling records. With the fix, the result must parse
	// as a single valid JSON object.
	l.Info("user input: fake_record\n{\"level\":\"FAKE\",\"msg\":\"SPLIT\"}")

	line := strings.TrimRight(buf.String(), "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("logger.split a single record into multiple lines: %q", line)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("JSON record did not parse: %v (line=%q)", err, line)
	}
	if rec["msg"] != "user input: fake_record\n{\"level\":\"FAKE\",\"msg\":\"SPLIT\"}" {
		t.Errorf("msg field was mutated by escaping: %v", rec["msg"])
	}
}

func TestLogger_TextModeEscapesNewlines(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, &buf, LevelDebug)

	l.Info("first\nsecond")

	out := buf.String()
	// Text mode must collapse any embedded "\n" into U+2028 so that grep /
	// awk treating each line as a distinct record does not see a fake
	// sibling record from the same logger call.
	if strings.Contains(out[0:strings.Index(out, "first")+len("first")+1], "\n") {
		t.Fatalf("text mode emitted a literal newline mid-record: %q", out)
	}
	if !strings.Contains(out, "\u2028") {
		t.Fatalf("text mode should use U+2028 to join wrapped lines: %q", out)
	}
}
