//go:build linux

package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dck/internal/state"
)

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	AuditEventStart       AuditEventType = "start"
	AuditEventStop        AuditEventType = "stop"
	AuditEventRemove      AuditEventType = "remove"
	AuditEventExec        AuditEventType = "exec"
	AuditEventRestart     AuditEventType = "restart"
	AuditEventPause       AuditEventType = "pause"
	AuditEventUnpause     AuditEventType = "unpause"
	AuditEventBackup      AuditEventType = "backup"
	AuditEventRestore     AuditEventType = "restore"
	AuditEventCreate      AuditEventType = "create"
	AuditEventImagePull   AuditEventType = "image_pull"
	AuditEventImageRemove AuditEventType = "image_remove"
	AuditEventNetwork     AuditEventType = "network"
	AuditEventVolume      AuditEventType = "volume"
	AuditEventAuth        AuditEventType = "auth"
	AuditEventAPIRequest  AuditEventType = "api_request"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	Timestamp   time.Time       `json:"timestamp"`
	EventType   AuditEventType  `json:"event_type"`
	ContainerID string          `json:"container_id,omitempty"`
	ContainerName string        `json:"container_name,omitempty"`
	User        string          `json:"user,omitempty"`
	RemoteAddr  string          `json:"remote_addr,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
	Success     bool            `json:"success"`
	Error       string          `json:"error,omitempty"`
}

// AuditDetails contains additional details for specific event types.
type AuditDetails struct {
	Command    []string          `json:"command,omitempty"`
	Image      string            `json:"image,omitempty"`
	Ports      []PortMap         `json:"ports,omitempty"`
	Volumes    []VolumeMount     `json:"volumes,omitempty"`
	Restart    string            `json:"restart,omitempty"`
	ExitCode   int               `json:"exit_code,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Network    string            `json:"network,omitempty"`
}

// AuditLogger provides audit logging for container operations.
type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
}

var (
	globalAuditLogger *AuditLogger
	auditOnce         sync.Once
)

// InitAuditLogger initializes the global audit logger.
func InitAuditLogger() error {
	var initErr error
	auditOnce.Do(func() {
		auditDir := filepath.Join(state.DataDir(), "audit")
		if err := os.MkdirAll(auditDir, 0700); err != nil {
			initErr = fmt.Errorf("create audit directory: %w", err)
			return
		}

		// Use date-based log files
		logFile := filepath.Join(auditDir, fmt.Sprintf("audit-%s.log", time.Now().Format("2006-01-02")))
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			initErr = fmt.Errorf("open audit log: %w", err)
			return
		}

		globalAuditLogger = &AuditLogger{
			file:    f,
			encoder: json.NewEncoder(f),
		}
	})

	return initErr
}

// CloseAuditLogger closes the global audit logger.
func CloseAuditLogger() {
	if globalAuditLogger != nil {
		globalAuditLogger.mu.Lock()
		defer globalAuditLogger.mu.Unlock()
		if globalAuditLogger.file != nil {
			globalAuditLogger.file.Close()
			globalAuditLogger.file = nil
		}
	}
}

// LogAuditEvent logs an audit event.
func LogAuditEvent(eventType AuditEventType, containerID, containerName string, success bool, details *AuditDetails, err error) {
	if globalAuditLogger == nil {
		return
	}

	event := AuditEvent{
		Timestamp:     time.Now(),
		EventType:     eventType,
		ContainerID:   containerID,
		ContainerName: containerName,
		User:          getCurrentUser(),
		Success:       success,
	}

	if details != nil {
		data, marshalErr := json.Marshal(details)
		if marshalErr == nil {
			event.Details = data
		}
	}

	if err != nil {
		event.Error = err.Error()
	}

	globalAuditLogger.mu.Lock()
	defer globalAuditLogger.mu.Unlock()

	if globalAuditLogger.encoder != nil {
		_ = globalAuditLogger.encoder.Encode(event)
	}
}

// LogAPIRequest logs an API request event.
func LogAPIRequest(method, path, remoteAddr string, success bool, err error) {
	if globalAuditLogger == nil {
		return
	}

	details := &AuditDetails{
		Command: []string{method, path},
	}

	event := AuditEvent{
		Timestamp:   time.Now(),
		EventType:   AuditEventAPIRequest,
		RemoteAddr:  remoteAddr,
		Details:     mustMarshal(details),
		Success:     success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	globalAuditLogger.mu.Lock()
	defer globalAuditLogger.mu.Unlock()

	if globalAuditLogger.encoder != nil {
		_ = globalAuditLogger.encoder.Encode(event)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func getCurrentUser() string {
	// Try to get the current user
	if user := os.Getenv("SUDO_USER"); user != "" {
		return user
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return fmt.Sprintf("uid=%d", os.Getuid())
}

// GetAuditLogPath returns the path to the current audit log file.
func GetAuditLogPath() string {
	return filepath.Join(state.DataDir(), "audit", fmt.Sprintf("audit-%s.log", time.Now().Format("2006-01-02")))
}

// ReadAuditLog reads audit events from a log file.
func ReadAuditLog(path string) ([]AuditEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var events []AuditEvent
	decoder := json.NewDecoder(stringReader(string(data)))
	for decoder.More() {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			continue // Skip malformed entries
		}
		events = append(events, event)
	}

	return events, nil
}

func stringReader(s string) *stringReaderImpl {
	return &stringReaderImpl{data: s, pos: 0}
}

type stringReaderImpl struct {
	data string
	pos  int
}

func (r *stringReaderImpl) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// CleanupOldAuditLogs removes audit logs older than the specified duration.
func CleanupOldAuditLogs(maxAge time.Duration) error {
	auditDir := filepath.Join(state.DataDir(), "audit")
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(auditDir, entry.Name()))
		}
	}

	return nil
}

// LogContainerEvent is a convenience function for logging container lifecycle events.
func LogContainerEvent(eventType AuditEventType, c *Container, success bool, err error) {
	if c == nil {
		return
	}

	LogAuditEvent(eventType, c.ID, c.Name, success, nil, err)
}
