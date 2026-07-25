package log

import (
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
		escaped := strings.ReplaceAll(msg, "\"", "\\\"")
		_, _ = fmt.Fprintf(out, "{\"time\":\"%s\",\"level\":\"%s\",\"msg\":\"%s\"}\n", timestamp, levelNames[level], escaped)
	} else {
		_, _ = fmt.Fprintf(out, "[%s] %-5s %s\n", timestamp, levelNames[level], msg)
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
