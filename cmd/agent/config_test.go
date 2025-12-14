package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "agent.yaml")

	configContent := `
server:
  address: "localhost:9443"

agent:
  name: "test-agent"
  batch_size: 50
  flush_interval: 2s

sources:
  - name: "nginx"
    type: "nginx"
    path: "/var/log/nginx/access.log"
    follow: true

labels:
  env: "test"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Server.Address != "localhost:9443" {
		t.Errorf("Server.Address = %v, want 'localhost:9443'", cfg.Server.Address)
	}
	if cfg.Agent.Name != "test-agent" {
		t.Errorf("Agent.Name = %v, want 'test-agent'", cfg.Agent.Name)
	}
	if cfg.Agent.BatchSize != 50 {
		t.Errorf("Agent.BatchSize = %d, want 50", cfg.Agent.BatchSize)
	}
	if cfg.Agent.FlushInterval != 2*time.Second {
		t.Errorf("Agent.FlushInterval = %v, want 2s", cfg.Agent.FlushInterval)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(cfg.Sources))
	}
	if cfg.Sources[0].Name != "nginx" {
		t.Errorf("Sources[0].Name = %v, want 'nginx'", cfg.Sources[0].Name)
	}
	if cfg.Labels["env"] != "test" {
		t.Errorf("Labels[env] = %v, want 'test'", cfg.Labels["env"])
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "agent.yaml")

	// Minimal config without defaults
	configContent := `
server:
  address: "localhost:9443"

sources:
  - name: "test"
    type: "nginx"
    path: "/var/log/test.log"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Check defaults were applied
	if cfg.Agent.ID == "" {
		t.Error("Agent.ID should be auto-generated")
	}
	if cfg.Agent.BatchSize != 100 {
		t.Errorf("Agent.BatchSize = %d, want 100 (default)", cfg.Agent.BatchSize)
	}
	if cfg.Agent.FlushInterval != time.Second {
		t.Errorf("Agent.FlushInterval = %v, want 1s (default)", cfg.Agent.FlushInterval)
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "missing server address",
			config:  "sources:\n  - name: test\n    type: nginx\n    path: /tmp/test.log",
			wantErr: "server.address is required",
		},
		{
			name:    "no sources",
			config:  "server:\n  address: localhost:9443",
			wantErr: "at least one source is required",
		},
		{
			name:    "source missing name",
			config:  "server:\n  address: localhost:9443\nsources:\n  - type: nginx\n    path: /tmp/test.log",
			wantErr: "sources[0].name is required",
		},
		{
			name:    "source missing path",
			config:  "server:\n  address: localhost:9443\nsources:\n  - name: test\n    type: nginx",
			wantErr: "sources[0].path is required",
		},
		{
			name:    "source missing type",
			config:  "server:\n  address: localhost:9443\nsources:\n  - name: test\n    path: /tmp/test.log",
			wantErr: "sources[0].type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "agent.yaml")

			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := LoadConfig(configFile)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
