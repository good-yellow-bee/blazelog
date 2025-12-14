package ssh

import (
	"testing"
	"time"
)

func TestDefaultTailerConfig(t *testing.T) {
	cfg := DefaultTailerConfig()

	if !cfg.Follow {
		t.Error("expected Follow to be true by default")
	}
	if cfg.PollInterval != 500*time.Millisecond {
		t.Errorf("expected PollInterval 500ms, got %v", cfg.PollInterval)
	}
	if !cfg.ReOpen {
		t.Error("expected ReOpen to be true by default")
	}
	if cfg.FromEnd {
		t.Error("expected FromEnd to be false by default")
	}
}

func TestTailer_NotRunning(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}
	client := NewClient(cfg)

	tailer := NewTailer(client, "/var/log/test.log", nil)

	// Lines channel should exist
	if tailer.Lines() == nil {
		t.Error("expected Lines channel to be non-nil")
	}

	// Stop should not panic when not running
	tailer.Stop()
}

func TestStreamTailer_NotRunning(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}
	client := NewClient(cfg)

	tailer := NewStreamTailer(client, "/var/log/test.log", nil)

	// Lines channel should exist
	if tailer.Lines() == nil {
		t.Error("expected Lines channel to be non-nil")
	}

	// Stop should not panic when not running
	tailer.Stop()
}

func TestLine_Fields(t *testing.T) {
	now := time.Now()
	line := Line{
		Text:     "test log line",
		FilePath: "/var/log/test.log",
		Host:     "server1:22",
		Time:     now,
		Err:      nil,
	}

	if line.Text != "test log line" {
		t.Errorf("expected Text 'test log line', got %q", line.Text)
	}
	if line.FilePath != "/var/log/test.log" {
		t.Errorf("expected FilePath '/var/log/test.log', got %q", line.FilePath)
	}
	if line.Host != "server1:22" {
		t.Errorf("expected Host 'server1:22', got %q", line.Host)
	}
	if line.Time != now {
		t.Errorf("expected Time %v, got %v", now, line.Time)
	}
	if line.Err != nil {
		t.Errorf("expected Err nil, got %v", line.Err)
	}
}

func TestBytesReader(t *testing.T) {
	data := []byte("hello world")
	reader := &bytesReader{data: data}

	// Read first chunk
	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected to read 5 bytes, got %d", n)
	}
	if string(buf) != "hello" {
		t.Errorf("expected 'hello', got %q", string(buf))
	}

	// Read rest
	buf = make([]byte, 10)
	n, err = reader.Read(buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected to read 6 bytes, got %d", n)
	}
	if string(buf[:n]) != " world" {
		t.Errorf("expected ' world', got %q", string(buf[:n]))
	}

	// Read at EOF
	n, err = reader.Read(buf)
	if err == nil {
		t.Error("expected EOF error")
	}
	if n != 0 {
		t.Errorf("expected 0 bytes at EOF, got %d", n)
	}
}

func TestTailerConfig_Custom(t *testing.T) {
	cfg := &TailerConfig{
		Follow:       false,
		PollInterval: 1 * time.Second,
		ReOpen:       false,
		FromEnd:      true,
	}

	client := NewClient(&ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	})

	tailer := NewTailer(client, "/var/log/test.log", cfg)

	if tailer.config.Follow {
		t.Error("expected Follow to be false")
	}
	if tailer.config.PollInterval != 1*time.Second {
		t.Errorf("expected PollInterval 1s, got %v", tailer.config.PollInterval)
	}
	if tailer.config.ReOpen {
		t.Error("expected ReOpen to be false")
	}
	if !tailer.config.FromEnd {
		t.Error("expected FromEnd to be true")
	}
}
