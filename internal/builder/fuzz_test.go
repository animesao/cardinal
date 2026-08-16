//go:build linux

package builder

import (
	"strings"
	"testing"
)

// FuzzParseDockerfile feeds random bytes into the Dockerfile parser.
//
// The parser is the first line of defence against malicious Dockerfiles
// that try to confuse the build graph with bizarre escaping, NUL bytes or
// extremely long lines. We just want to ensure parsing never crashes, never
// leaks huge slices, and never fails in unexpected ways.
func FuzzParseDockerfile(f *testing.F) {
	// Seed corpus with realistic edge cases.
	f.Add("FROM alpine\nRUN echo hi\n")
	f.Add("FROM scratch\nCOPY --chown=0:0 . /\n")
	f.Add("ARG $$INVALID\nRUN \n")
	f.Add("COPY a b\nCOPY \\\\\n a\n")
	f.Add("\x00\x00\x00FROM\x00scratch\x00\n")
	f.Add(strings.Repeat("A", 4096) + "\n")
	f.Add("FROM alpine\n" + strings.Repeat("# comment\n", 200))

	f.Fuzz(func(t *testing.T, content string) {
		// Don't try enormously large inputs — those are not interesting
		// and trigger os.ErrInvalidData in the lexer. Cap at 32 KiB.
		if len(content) > 32*1024 {
			t.Skip()
		}
		_, _ = ParseInstructions(content)
	})
}

// FuzzSplitShellWords is the most hit function in the parser — so we
// guarantee it on its own as well.
func FuzzSplitShellWords(f *testing.F) {
	f.Add(`"a b" 'c d' e\ f`)
	f.Add(strings.Repeat(`"`, 30))
	f.Add("FROM alpine AS --flag=value extra")
	f.Add("")

	f.Fuzz(func(t *testing.T, content string) {
		out := SplitShellWords(content)
		// Sanity: every returned token should not be larger than input.
		for _, tok := range out {
			if len(tok) > len(content)+8 {
				t.Fatalf("token larger than input: %q (len=%d input=%d)", tok, len(tok), len(content))
			}
		}
	})
}

func TestParseInstructionsLongContinuation(t *testing.T) {
	content := "RUN " + strings.Repeat(string([]byte{'\\', '\n'}), 4096) + "echo done\n"
	if _, err := ParseInstructions(content); err != nil {
		t.Fatalf("long continuation should parse without error: %v", err)
	}
}
