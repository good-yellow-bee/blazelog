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
	Server  ServerConfig      `yaml:"server"`
	Agent   AgentConfig       `yaml:"agent"`
	Sources []SourceConfig    `yaml:"sources"`
	Labels  map[string]string `yaml:"labels"`
}

// ServerConfig contains server connection settings.
type ServerConfig struct {
	Address string `yaml:"address"` // host:port
}

// AgentConfig contains agent settings.
type AgentConfig struct {
	ID            string        `yaml:"id"`             // optional, auto-generated if empty
	Name          string        `yaml:"name"`           // human-readable name
	BatchSize     int           `yaml:"batch_size"`     // entries per batch (default: 100)
	FlushInterval time.Duration `yaml:"flush_interval"` // batch flush interval (default: 1s)
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
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
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
