package ssh

import (
	"context"
	"testing"
	"time"
)

func TestClientConfig_Defaults(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}

	client := NewClient(cfg)

	if client.config.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", client.config.Timeout)
	}

	if client.config.KeepAliveInterval != 30*time.Second {
		t.Errorf("expected default keepalive 30s, got %v", client.config.KeepAliveInterval)
	}
}

func TestClient_NotConnected(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}

	client := NewClient(cfg)

	if client.IsConnected() {
		t.Error("expected client to not be connected initially")
	}

	// Operations should fail when not connected
	ctx := context.Background()

	_, err := client.ReadFile(ctx, "/tmp/test")
	if err == nil {
		t.Error("expected error when reading file without connection")
	}

	_, err = client.FileInfo(ctx, "/tmp/test")
	if err == nil {
		t.Error("expected error when getting file info without connection")
	}

	_, err = client.ListFiles(ctx, "/tmp/*")
	if err == nil {
		t.Error("expected error when listing files without connection")
	}

	_, err = client.StreamFile(ctx, "/tmp/test", 0)
	if err == nil {
		t.Error("expected error when streaming file without connection")
	}
}

func TestClient_NoAuthMethods(t *testing.T) {
	cfg := &ClientConfig{
		Host: "localhost:22",
		User: "test",
		// No key file or password
	}

	client := NewClient(cfg)
	ctx := context.Background()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("expected error when no auth methods configured")
	}
}

func TestClient_InvalidKeyFile(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/nonexistent/key/file",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("expected error when key file doesn't exist")
	}
}

func TestClient_CloseWithoutConnect(t *testing.T) {
	cfg := &ClientConfig{
		Host:    "localhost:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}

	client := NewClient(cfg)

	// Should not panic
	err := client.Close()
	if err != nil {
		t.Errorf("unexpected error closing unconnected client: %v", err)
	}
}

func TestRemoteFileInfo(t *testing.T) {
	info := &RemoteFileInfo{
		Path:    "/var/log/test.log",
		Size:    1024,
		ModTime: 1700000000,
		Inode:   12345,
	}

	if info.Path != "/var/log/test.log" {
		t.Errorf("expected path /var/log/test.log, got %s", info.Path)
	}
	if info.Size != 1024 {
		t.Errorf("expected size 1024, got %d", info.Size)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []string
	}{
		{
			name:     "empty",
			input:    []byte{},
			expected: nil,
		},
		{
			name:     "single line no newline",
			input:    []byte("hello"),
			expected: []string{"hello"},
		},
		{
			name:     "single line with newline",
			input:    []byte("hello\n"),
			expected: []string{"hello"},
		},
		{
			name:     "multiple lines",
			input:    []byte("line1\nline2\nline3\n"),
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "windows line endings",
			input:    []byte("line1\r\nline2\r\n"),
			expected: []string{"line1", "line2"},
		},
		{
			name:     "trailing content",
			input:    []byte("line1\nline2"),
			expected: []string{"line1", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
