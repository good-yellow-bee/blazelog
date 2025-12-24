// Package models contains the core data structures for BlazeLog.
package models

import (
	"encoding/json"
	"time"
)

// LogLevel represents the severity level of a log entry.
type LogLevel string

const (
	LevelDebug   LogLevel = "debug"
	LevelInfo    LogLevel = "info"
	LevelWarning LogLevel = "warning"
	LevelError   LogLevel = "error"
	LevelFatal   LogLevel = "fatal"
	LevelUnknown LogLevel = "unknown"
)

// LogType represents the type/source of the log.
type LogType string

const (
	LogTypeNginx      LogType = "nginx"
	LogTypeApache     LogType = "apache"
	LogTypeMagento    LogType = "magento"
	LogTypePrestaShop LogType = "prestashop"
	LogTypeWordPress  LogType = "wordpress"
	LogTypeCustom     LogType = "custom"
	LogTypeUnknown    LogType = "unknown"
)

// LogEntry represents a single parsed log entry.
type LogEntry struct {
	// Timestamp is when the log event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Level is the severity level of the log.
	Level LogLevel `json:"level"`

	// Message is the main log message content.
	Message string `json:"message"`

	// Source identifies where the log came from.
	Source string `json:"source,omitempty"`

	// Type is the log format type (nginx, apache, etc.).
	Type LogType `json:"type"`

	// Raw is the original unparsed log line.
	Raw string `json:"raw,omitempty"`

	// Fields contains additional parsed fields specific to the log type.
	// For example: status code, request path, response time, etc.
	Fields map[string]interface{} `json:"fields,omitempty"`

	// Labels are key-value pairs for categorization.
	Labels map[string]string `json:"labels,omitempty"`

	// LineNumber is the line number in the source file.
	LineNumber int64 `json:"line_number,omitempty"`

	// FilePath is the path to the source file.
	FilePath string `json:"file_path,omitempty"`
}

// NewLogEntry creates a new LogEntry with initialized maps.
func NewLogEntry() *LogEntry {
	return &LogEntry{
		Fields: make(map[string]interface{}),
		Labels: make(map[string]string),
		Level:  LevelUnknown,
		Type:   LogTypeUnknown,
	}
}

// SetField sets a field value.
func (e *LogEntry) SetField(key string, value interface{}) {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	e.Fields[key] = value
}

// GetField retrieves a field value.
func (e *LogEntry) GetField(key string) (interface{}, bool) {
	if e.Fields == nil {
		return nil, false
	}
	val, ok := e.Fields[key]
	return val, ok
}

// GetFieldString retrieves a field value as string.
func (e *LogEntry) GetFieldString(key string) string {
	val, ok := e.GetField(key)
	if !ok {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// GetFieldInt retrieves a field value as int.
func (e *LogEntry) GetFieldInt(key string) int {
	val, ok := e.GetField(key)
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// SetLabel sets a label value.
func (e *LogEntry) SetLabel(key, value string) {
	if e.Labels == nil {
		e.Labels = make(map[string]string)
	}
	e.Labels[key] = value
}

// GetLabel retrieves a label value.
func (e *LogEntry) GetLabel(key string) string {
	if e.Labels == nil {
		return ""
	}
	return e.Labels[key]
}

// JSON returns the log entry as JSON bytes.
func (e *LogEntry) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// JSONPretty returns the log entry as pretty-printed JSON bytes.
func (e *LogEntry) JSONPretty() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// String returns a string representation of the log entry.
func (e *LogEntry) String() string {
	return e.Timestamp.Format(time.RFC3339) + " [" + string(e.Level) + "] " + e.Message
}

// IsError returns true if the log level is error or fatal.
func (e *LogEntry) IsError() bool {
	return e.Level == LevelError || e.Level == LevelFatal
}

// ParseLogLevel converts a string to LogLevel.
func ParseLogLevel(s string) LogLevel {
	switch s {
	case "debug", "DEBUG", "Debug":
		return LevelDebug
	case "info", "INFO", "Info", "notice", "NOTICE", "Notice":
		return LevelInfo
	case "warning", "WARNING", "Warning", "warn", "WARN", "Warn":
		return LevelWarning
	case "error", "ERROR", "Error", "err", "ERR", "Err":
		return LevelError
	case "fatal", "FATAL", "Fatal", "critical", "CRITICAL", "Critical", "crit", "CRIT", "Crit", "emergency", "EMERGENCY", "Emergency", "emerg", "EMERG", "Emerg", "alert", "ALERT", "Alert":
		return LevelFatal
	default:
		return LevelUnknown
	}
}
