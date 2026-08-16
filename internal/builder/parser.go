package builder

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

func ParseDockerfile(path string) ([]Instruction, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Dockerfile: %w", err)
	}

	return ParseInstructions(string(data))
}

func ParseInstructions(content string) ([]Instruction, error) {
	lines := strings.Split(content, "\n")
	var insts []Instruction
	var pending string
	var lineCont bool

	for i, raw := range lines {
		line := strings.TrimRightFunc(raw, unicode.IsSpace)

		// Handle line continuation (backslash at end of line)
		if lineCont {
			pending += " " + strings.TrimLeftFunc(line, unicode.IsSpace)
			if strings.HasSuffix(line, "\\") {
				pending = pending[:len(pending)-1]
				continue
			}
			lineCont = false
			line = pending
			pending = ""
		} else if strings.HasSuffix(line, "\\") {
			pending = line[:len(line)-1]
			lineCont = true
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		inst, err := parseInstruction(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if inst != nil {
			insts = append(insts, *inst)
		}
	}

	return insts, nil
}

func parseInstruction(line string) (*Instruction, error) {
	parts := splitInstruction(line)
	if len(parts) == 0 {
		return nil, nil
	}

	instType := InstructionType(strings.ToUpper(parts[0]))
	args := parts[1:]

	switch instType {
	case From, Run, Cmd, Entrypoint, Env, Workdir, Copy, Add,
		Expose, Label, User, Volume, Shell, Arg, StopSignal,
		Healthcheck, Maintainer, OnBuild:
		return &Instruction{Type: instType, Args: args, Raw: line}, nil
	default:
		return nil, fmt.Errorf("unknown instruction: %s", parts[0])
	}
}

// splitInstruction splits a Dockerfile instruction line into command + args,
// respecting quoted strings (single and double) and exec form JSON arrays.
func splitInstruction(line string) []string {
	line = strings.TrimSpace(line)

	// Check for exec form (JSON array): ["executable", "arg1", "arg2"]
	if strings.HasPrefix(line, "[") {
		// This is likely an argument to CMD/ENTRYPOINT/RUN in exec form
		return parseJSONArray(line)
	}

	// Split on first whitespace for instruction keyword
	idx := strings.IndexFunc(line, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	if idx == -1 {
		return []string{line}
	}

	inst := line[:idx]
	rest := strings.TrimSpace(line[idx:])

	// Check if the rest is a JSON array (e.g., CMD ["cmd", "arg"])
	if strings.HasPrefix(rest, "[") {
		return append([]string{inst}, parseJSONArray(rest)...)
	}

	// For ENV with multiple key=val pairs or KEY VAL format
	if inst == "ENV" {
		return append([]string{inst}, parseEnvArgs(rest)...)
	}

	// For LABEL with key=val pairs
	if inst == "LABEL" {
		return append([]string{inst}, parseLabelArgs(rest)...)
	}

	// For EXPOSE with multiple ports
	if inst == "EXPOSE" {
		parts := strings.Fields(rest)
		return append([]string{inst}, parts...)
	}

	// For COPY/ADD --chown flag handling
	if (inst == "COPY" || inst == "ADD") && strings.HasPrefix(rest, "--") {
		// Simple: treat flags as part of args, user will parse
		parts := SplitSpaceRespectingQuotes(rest)
		return append([]string{inst}, parts...)
	}

	// For most instructions, just split on spaces respecting quotes
	parts := SplitSpaceRespectingQuotes(rest)
	return append([]string{inst}, parts...)
}

func SplitSpaceRespectingQuotes(s string) []string {
	// Iterate byte-by-byte rather than rune-by-rune. Shell
	// tokenisation is a byte-level operation (POSIX sh splits on
	// whitespace and quote bytes; it does not interpret multi-byte
	// UTF-8 sequences as semantic units). Iterating runes would also
	// rewrite invalid UTF-8 bytes as U+FFFD (3 bytes), producing
	// tokens that are LONGER than the input that produced them. That
	// amplification is observable through FuzzSplitShellWords and a
	// motivated attacker could send a 32 KiB blob of invalid UTF-8 to
	// produce ~96 KiB of garbage in parser output. Bad. We therefore
	// skip unicode.IsSpace entirely and inline the four ASCII space /
	// tab / newline / carriage-return checks below; non-ASCII bytes
	// are preserved verbatim so tokens stay exactly as long as the
	// input contributes.
	var parts []string
	var current strings.Builder
	var inSingle, inDouble bool
	var escape bool

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			current.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' && inDouble {
			escape = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(c)
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(c)
			continue
		}
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			if !inSingle && !inDouble {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
				continue
			}
		}
		current.WriteByte(c)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func parseJSONArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return SplitSpaceRespectingQuotes(s)
	}
	s = s[1 : len(s)-1]

	var parts []string
	var current strings.Builder
	var inSingle, inDouble bool
	var escape bool

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			current.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' && inDouble {
			escape = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if c == ',' && !inSingle && !inDouble {
			val := strings.TrimSpace(current.String())
			val = strings.Trim(val, "\"'")
			if val != "" {
				parts = append(parts, val)
			}
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	val := strings.TrimSpace(current.String())
	val = strings.Trim(val, "\"'")
	if val != "" {
		parts = append(parts, val)
	}

	return parts
}


// parseEnvArgs handles "ENV KEY=VAL KEY2=VAL2" and "ENV KEY VAL" formats
func parseEnvArgs(s string) []string {
	if strings.Contains(s, "=") {
		// KEY=VAL format (may be multiple)
		parts := SplitSpaceRespectingQuotes(s)
		return parts
	}
	// KEY VAL format (single pair)
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 2 {
		return []string{parts[0], strings.TrimSpace(parts[1])}
	}
	return parts
}

// parseLabelArgs handles "LABEL key=val key2=val2"
func parseLabelArgs(s string) []string {
	return SplitSpaceRespectingQuotes(s)
}

// SplitShellWords splits s into words like a shell: respects quotes and strips them.
// Unlike SplitSpaceRespectingQuotes, it removes surrounding quote characters from tokens.
func SplitShellWords(s string) []string {
	parts := SplitSpaceRespectingQuotes(s)
	for i, p := range parts {
		if len(p) >= 2 {
			if (p[0] == '\'' && p[len(p)-1] == '\'') || (p[0] == '"' && p[len(p)-1] == '"') {
				parts[i] = p[1 : len(p)-1]
			}
		}
	}
	return parts
}

// GetExecForm attempts to parse instruction args as exec form
func GetExecForm(args []string) ([]string, bool) {
	if len(args) == 1 {
		// Could be JSON array like '["cmd", "arg"]'
		parsed := parseJSONArray(args[0])
		if len(parsed) > 0 {
			return parsed, true
		}
	}
	return nil, false
}
