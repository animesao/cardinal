//go:build linux

package cmd

import "testing"

func TestParseSHA256Checksum(t *testing.T) {
	const checksum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, input := range []string{checksum, checksum + "  dck-linux-amd64\n"} {
		got, err := parseSHA256Checksum(input)
		if err != nil {
			t.Fatalf("parseSHA256Checksum(%q): %v", input, err)
		}
		if got != checksum {
			t.Fatalf("checksum = %q, want %q", got, checksum)
		}
	}
}

func TestParseSHA256ChecksumRejectsEmptyAndInvalid(t *testing.T) {
	for _, input := range []string{"", "not-a-checksum", "0123"} {
		if _, err := parseSHA256Checksum(input); err == nil {
			t.Errorf("parseSHA256Checksum(%q) unexpectedly succeeded", input)
		}
	}
}
