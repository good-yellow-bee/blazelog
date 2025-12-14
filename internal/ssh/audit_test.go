package ssh

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testWriteCloser wraps a bytes.Buffer to implement io.WriteCloser.
type testWriteCloser struct {
	*bytes.Buffer
}

func (t *testWriteCloser) Close() error {
	return nil
}

func newTestWriteCloser() *testWriteCloser {
	return &testWriteCloser{Buffer: &bytes.Buffer{}}
}

func TestJSONAuditLogger_LogConnect(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogConnect("example.com:22", "blazelog", "bastion.com:22")

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "connect" {
		t.Errorf("event: got %q, want %q", event.Event, "connect")
	}
	if event.Host != "example.com:22" {
		t.Errorf("host: got %q, want %q", event.Host, "example.com:22")
	}
	if event.User != "blazelog" {
		t.Errorf("user: got %q, want %q", event.User, "blazelog")
	}
	if event.JumpHost != "bastion.com:22" {
		t.Errorf("jump_host: got %q, want %q", event.JumpHost, "bastion.com:22")
	}
	if event.Timestamp == "" {
		t.Error("timestamp should be set")
	}
}

func TestJSONAuditLogger_LogDisconnect(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogDisconnect("example.com:22", io.EOF)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "disconnect" {
		t.Errorf("event: got %q, want %q", event.Event, "disconnect")
	}
	if event.Error != "EOF" {
		t.Errorf("error: got %q, want %q", event.Error, "EOF")
	}
}

func TestJSONAuditLogger_LogDisconnect_NoError(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogDisconnect("example.com:22", nil)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Error != "" {
		t.Errorf("error should be empty, got %q", event.Error)
	}
}

func TestJSONAuditLogger_LogHostKeyAccepted(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogHostKeyAccepted("example.com:22", "SHA256:abc123", true)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "host_key_accepted" {
		t.Errorf("event: got %q, want %q", event.Event, "host_key_accepted")
	}
	if event.Fingerprint != "SHA256:abc123" {
		t.Errorf("fingerprint: got %q, want %q", event.Fingerprint, "SHA256:abc123")
	}
	if event.IsNew == nil || !*event.IsNew {
		t.Error("is_new should be true")
	}
}

func TestJSONAuditLogger_LogHostKeyRejected(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogHostKeyRejected("example.com:22", "SHA256:expected", "SHA256:actual")

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "host_key_rejected" {
		t.Errorf("event: got %q, want %q", event.Event, "host_key_rejected")
	}
	if event.Expected != "SHA256:expected" {
		t.Errorf("expected: got %q, want %q", event.Expected, "SHA256:expected")
	}
	if event.Actual != "SHA256:actual" {
		t.Errorf("actual: got %q, want %q", event.Actual, "SHA256:actual")
	}
}

func TestJSONAuditLogger_LogCommand(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogCommand("example.com:22", "cat /var/log/test.log", true, 150*time.Millisecond)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "command" {
		t.Errorf("event: got %q, want %q", event.Event, "command")
	}
	if event.Command != "cat /var/log/test.log" {
		t.Errorf("cmd: got %q, want %q", event.Command, "cat /var/log/test.log")
	}
	if event.Success == nil || !*event.Success {
		t.Error("success should be true")
	}
	if event.DurationMs != 150 {
		t.Errorf("duration_ms: got %d, want %d", event.DurationMs, 150)
	}
}

func TestJSONAuditLogger_LogFileOp(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogFileOp("example.com:22", "read", "/var/log/nginx/access.log", 4096, nil)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event != "file_op" {
		t.Errorf("event: got %q, want %q", event.Event, "file_op")
	}
	if event.Operation != "read" {
		t.Errorf("op: got %q, want %q", event.Operation, "read")
	}
	if event.Path != "/var/log/nginx/access.log" {
		t.Errorf("path: got %q, want %q", event.Path, "/var/log/nginx/access.log")
	}
	if event.Bytes != 4096 {
		t.Errorf("bytes: got %d, want %d", event.Bytes, 4096)
	}
}

func TestJSONAuditLogger_LogFileOp_WithError(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogFileOp("example.com:22", "read", "/var/log/missing.log", 0, os.ErrNotExist)

	var event AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Error == "" {
		t.Error("error should be set")
	}
}

func TestJSONAuditLogger_MultipleEvents(t *testing.T) {
	buf := newTestWriteCloser()
	logger := NewJSONAuditLoggerWriter(buf)

	logger.LogConnect("host1:22", "user1", "")
	logger.LogConnect("host2:22", "user2", "")
	logger.LogDisconnect("host1:22", nil)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d: unmarshal failed: %v", i, err)
		}
	}
}

func TestNewJSONAuditLogger_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "audit.log")

	logger, err := NewJSONAuditLogger(path)
	if err != nil {
		t.Fatalf("NewJSONAuditLogger failed: %v", err)
	}
	defer logger.Close()

	logger.LogConnect("test:22", "user", "")

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("audit log file should be created")
	}
}

func TestNopAuditLogger(t *testing.T) {
	logger := NopAuditLogger{}

	// Should not panic
	logger.LogConnect("host:22", "user", "")
	logger.LogDisconnect("host:22", nil)
	logger.LogHostKeyAccepted("host:22", "fp", true)
	logger.LogHostKeyRejected("host:22", "a", "b")
	logger.LogCommand("host:22", "cmd", true, time.Second)
	logger.LogFileOp("host:22", "read", "/path", 100, nil)

	if err := logger.Close(); err != nil {
		t.Errorf("Close should return nil, got %v", err)
	}
}
