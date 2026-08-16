package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	err     io.Writer
	minLevel Level
	jsonMode bool
}

var Default = New(os.Stdout, os.Stderr, LevelInfo)

func New(out, errOut io.Writer, minLevel Level) *Logger {
	return &Logger{out: out, err: errOut, minLevel: minLevel}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

func (l *Logger) SetJSON(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonMode = enabled
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.minLevel {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format(time.RFC3339)

	l.mu.Lock()
	defer l.mu.Unlock()

	var out io.Writer = l.out
	if level >= LevelWarn {
		out = l.err
	}

	if l.jsonMode {
		// Proper JSON escaping for any control characters (including \n,
		// \r, \t) prevents log-injection attacks that would otherwise be
		// able to inject fake log records by putting newlines into fmt
		// format strings. The previous implementation only escaped double
		// quotes, silently breaking the assumption that one printf call
		// equals one log line; see SECURITY.md "Audit log integrity".
		payload, mErr := json.Marshal(struct {
			Time  string `json:"time"`
			Level string `json:"level"`
			Msg   string `json:"msg"`
		}{timestamp, levelNames[level], msg})
		if mErr == nil {
			_, _ = out.Write(payload)
			_, _ = out.Write([]byte("\n"))
		}
	} else {
		// Non-JSON mode still escapes newlines so the human-readable log
		// stream can't be split into multiple records by a single format
		// string. We use the Unicode line-separator character, which is
		// visually distinguishable on console while keeping records
		// one-per-line for grep/awk pipelines.
		escaped := strings.ReplaceAll(msg, "\n", "\u2028")
		escaped = strings.ReplaceAll(escaped, "\r", "\u2029")
		_, _ = fmt.Fprintf(out, "[%s] %-5s %s\n", timestamp, levelNames[level], escaped)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) { l.log(LevelDebug, format, args...) }
func (l *Logger) Info(format string, args ...interface{})  { l.log(LevelInfo, format, args...) }
func (l *Logger) Warn(format string, args ...interface{})  { l.log(LevelWarn, format, args...) }
func (l *Logger) Error(format string, args ...interface{}) { l.log(LevelError, format, args...) }

func Debug(format string, args ...interface{}) { Default.log(LevelDebug, format, args...) }
func Info(format string, args ...interface{})  { Default.log(LevelInfo, format, args...) }
func Warn(format string, args ...interface{})  { Default.log(LevelWarn, format, args...) }
func Error(format string, args ...interface{}) { Default.log(LevelError, format, args...) }

// SetLevel changes the default logger's minimum level.
func SetLevel(level Level) { Default.SetLevel(level) }

// SetJSON toggles JSON-line output on the default logger.
func SetJSON(enabled bool) { Default.SetJSON(enabled) }
