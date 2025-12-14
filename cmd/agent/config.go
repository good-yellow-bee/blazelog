// Package main provides the BlazeLog agent CLI.
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Config represents the agent configuration.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Agent       AgentConfig       `yaml:"agent"`
	Reliability ReliabilityConfig `yaml:"reliability"`
	Sources     []SourceConfig    `yaml:"sources"`
	Labels      map[string]string `yaml:"labels"`
}

// ServerConfig contains server connection settings.
type ServerConfig struct {
	Address string    `yaml:"address"` // host:port
	TLS     TLSConfig `yaml:"tls"`     // TLS configuration for mTLS
}

// TLSConfig contains TLS settings for the agent.
type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`              // Enable mTLS
	CertFile           string `yaml:"cert_file"`            // Agent certificate file
	KeyFile            string `yaml:"key_file"`             // Agent private key file
	CAFile             string `yaml:"ca_file"`              // CA certificate for verifying server cert
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // Skip server cert verification (dev only)
}

// AgentConfig contains agent settings.
type AgentConfig struct {
	ID            string        `yaml:"id"`             // optional, auto-generated if empty
	Name          string        `yaml:"name"`           // human-readable name
	BatchSize     int           `yaml:"batch_size"`     // entries per batch (default: 100)
	FlushInterval time.Duration `yaml:"flush_interval"` // batch flush interval (default: 1s)
}

// ReliabilityConfig contains reliability settings.
type ReliabilityConfig struct {
	BufferDir         string        `yaml:"buffer_dir"`         // buffer directory (default: ~/.blazelog/buffer)
	BufferMaxSize     string        `yaml:"buffer_max_size"`    // max buffer size (default: 100MB)
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // heartbeat interval (default: 15s)
	ReconnectInitial  time.Duration `yaml:"reconnect_initial"`  // initial reconnect delay (default: 1s)
	ReconnectMax      time.Duration `yaml:"reconnect_max"`      // max reconnect delay (default: 30s)
}

// SourceConfig defines a log source to collect.
type SourceConfig struct {
	Name   string `yaml:"name"`   // source identifier
	Type   string `yaml:"type"`   // parser type: nginx, apache, magento, prestashop, wordpress
	Path   string `yaml:"path"`   // file path or glob pattern
	Follow bool   `yaml:"follow"` // tail mode (default: true)
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for missing config fields.
func (c *Config) setDefaults() {
	if c.Agent.ID == "" {
		c.Agent.ID = uuid.New().String()
	}
	if c.Agent.Name == "" {
		hostname, _ := os.Hostname()
		c.Agent.Name = hostname
	}
	if c.Agent.BatchSize <= 0 {
		c.Agent.BatchSize = 100
	}
	if c.Agent.FlushInterval <= 0 {
		c.Agent.FlushInterval = time.Second
	}
	for i := range c.Sources {
		if !c.Sources[i].Follow {
			c.Sources[i].Follow = true
		}
	}
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}

	// Reliability defaults
	if c.Reliability.HeartbeatInterval <= 0 {
		c.Reliability.HeartbeatInterval = 15 * time.Second
	}
	if c.Reliability.ReconnectInitial <= 0 {
		c.Reliability.ReconnectInitial = time.Second
	}
	if c.Reliability.ReconnectMax <= 0 {
		c.Reliability.ReconnectMax = 30 * time.Second
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("server.tls.cert_file is required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("server.tls.key_file is required when TLS is enabled")
		}
		if !c.Server.TLS.InsecureSkipVerify && c.Server.TLS.CAFile == "" {
			return fmt.Errorf("server.tls.ca_file is required when TLS is enabled and insecure_skip_verify is false")
		}
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}
	for i, src := range c.Sources {
		if src.Name == "" {
			return fmt.Errorf("sources[%d].name is required", i)
		}
		if src.Path == "" {
			return fmt.Errorf("sources[%d].path is required", i)
		}
		if src.Type == "" {
			return fmt.Errorf("sources[%d].type is required", i)
		}
	}
	return nil
}

// Hostname returns the system hostname.
func Hostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

// OS returns the operating system.
func OS() string {
	return runtime.GOOS
}

// Arch returns the CPU architecture.
func Arch() string {
	return runtime.GOARCH
}
