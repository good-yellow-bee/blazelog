package ssh

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLogger defines the interface for SSH operation audit logging.
type AuditLogger interface {
	// LogConnect logs a connection attempt.
	LogConnect(host, user, jumpHost string)
	// LogDisconnect logs a disconnection.
	LogDisconnect(host string, err error)
	// LogHostKeyAccepted logs when a host key is accepted.
	LogHostKeyAccepted(host, fingerprint string, isNew bool)
	// LogHostKeyRejected logs when a host key is rejected.
	LogHostKeyRejected(host, expected, actual string)
	// LogHostKeyWarning logs when an unknown host key is accepted with warning (PolicyWarn).
	LogHostKeyWarning(host, fingerprint string, stored bool)
	// LogCommand logs a command execution.
	LogCommand(host, cmd string, success bool, duration time.Duration)
	// LogFileOp logs a file operation.
	LogFileOp(host, op, path string, bytes int64, err error)
	// Close closes the logger.
	Close() error
}

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	Timestamp   string `json:"ts"`
	Event       string `json:"event"`
	Host        string `json:"host,omitempty"`
	User        string `json:"user,omitempty"`
	JumpHost    string `json:"jump_host,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
	Command     string `json:"cmd,omitempty"`
	Success     *bool  `json:"success,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
	Operation   string `json:"op,omitempty"`
	Path        string `json:"path,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	Error       string `json:"error,omitempty"`
	IsNew       *bool  `json:"is_new,omitempty"`
	Stored      *bool  `json:"stored,omitempty"`
}

// JSONAuditLogger writes audit events as JSON lines.
type JSONAuditLogger struct {
	output io.WriteCloser
	mu     sync.Mutex
}

// NewJSONAuditLogger creates a new JSON audit logger that writes to the specified file.
// The file is created if it doesn't exist, and appended to if it does.
func NewJSONAuditLogger(path string) (*JSONAuditLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}

	return &JSONAuditLogger{output: file}, nil
}

// NewJSONAuditLoggerWriter creates a JSON audit logger with a custom writer.
// Useful for testing.
func NewJSONAuditLoggerWriter(w io.WriteCloser) *JSONAuditLogger {
	return &JSONAuditLogger{output: w}
}

func (l *JSONAuditLogger) log(event *AuditEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return // Silent fail on marshal error
	}

	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

// LogConnect logs a connection attempt.
func (l *JSONAuditLogger) LogConnect(host, user, jumpHost string) {
	l.log(&AuditEvent{
		Event:    "connect",
		Host:     host,
		User:     user,
		JumpHost: jumpHost,
	})
}

// LogDisconnect logs a disconnection.
func (l *JSONAuditLogger) LogDisconnect(host string, err error) {
	event := &AuditEvent{
		Event: "disconnect",
		Host:  host,
	}
	if err != nil {
		event.Error = err.Error()
	}
	l.log(event)
}

// LogHostKeyAccepted logs when a host key is accepted.
func (l *JSONAuditLogger) LogHostKeyAccepted(host, fingerprint string, isNew bool) {
	l.log(&AuditEvent{
		Event:       "host_key_accepted",
		Host:        host,
		Fingerprint: fingerprint,
		IsNew:       &isNew,
	})
}

// LogHostKeyRejected logs when a host key is rejected.
func (l *JSONAuditLogger) LogHostKeyRejected(host, expected, actual string) {
	l.log(&AuditEvent{
		Event:    "host_key_rejected",
		Host:     host,
		Expected: expected,
		Actual:   actual,
	})
}

// LogHostKeyWarning logs when an unknown host key is accepted with PolicyWarn.
func (l *JSONAuditLogger) LogHostKeyWarning(host, fingerprint string, stored bool) {
	l.log(&AuditEvent{
		Event:       "host_key_warning",
		Host:        host,
		Fingerprint: fingerprint,
		Stored:      &stored,
	})
}

// LogCommand logs a command execution.
func (l *JSONAuditLogger) LogCommand(host, cmd string, success bool, duration time.Duration) {
	l.log(&AuditEvent{
		Event:      "command",
		Host:       host,
		Command:    cmd,
		Success:    &success,
		DurationMs: duration.Milliseconds(),
	})
}

// LogFileOp logs a file operation.
func (l *JSONAuditLogger) LogFileOp(host, op, path string, bytes int64, err error) {
	event := &AuditEvent{
		Event:     "file_op",
		Host:      host,
		Operation: op,
		Path:      path,
		Bytes:     bytes,
	}
	if err != nil {
		event.Error = err.Error()
	}
	l.log(event)
}

// Close closes the underlying writer.
func (l *JSONAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.output.Close()
}

// NopAuditLogger is a no-op audit logger that discards all events.
type NopAuditLogger struct{}

func (NopAuditLogger) LogConnect(host, user, jumpHost string)                            {}
func (NopAuditLogger) LogDisconnect(host string, err error)                              {}
func (NopAuditLogger) LogHostKeyAccepted(host, fingerprint string, isNew bool)           {}
func (NopAuditLogger) LogHostKeyRejected(host, expected, actual string)                  {}
func (NopAuditLogger) LogHostKeyWarning(host, fingerprint string, stored bool)           {}
func (NopAuditLogger) LogCommand(host, cmd string, success bool, duration time.Duration) {}
func (NopAuditLogger) LogFileOp(host, op, path string, bytes int64, err error)           {}
func (NopAuditLogger) Close() error                                                      { return nil }
