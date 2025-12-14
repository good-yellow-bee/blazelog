// Package main provides the BlazeLog server CLI.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server         ServerConfig    `yaml:"server"`
	SSHConnections []SSHConnection `yaml:"ssh_connections"` // SSH connections for remote log collection
	Verbose        bool            `yaml:"-"`               // set via CLI flag
}

// ServerConfig contains server settings.
type ServerConfig struct {
	GRPCAddress string    `yaml:"grpc_address"` // gRPC listen address (default: :9443)
	HTTPAddress string    `yaml:"http_address"` // HTTP listen address (default: :8080)
	TLS         TLSConfig `yaml:"tls"`          // TLS configuration for mTLS
}

// TLSConfig contains TLS settings for the server.
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`        // Enable mTLS
	CertFile     string `yaml:"cert_file"`      // Server certificate file
	KeyFile      string `yaml:"key_file"`       // Server private key file
	ClientCAFile string `yaml:"client_ca_file"` // CA certificate for verifying client certs
}

// SSHConnection defines a remote server connection for log collection.
type SSHConnection struct {
	Name          string      `yaml:"name"`           // Unique name for this connection
	Host          string      `yaml:"host"`           // SSH server address (host:port)
	User          string      `yaml:"user"`           // SSH username
	KeyFile       string      `yaml:"key_file"`       // Path to private key file
	KeyPassphrase string      `yaml:"key_passphrase"` // Optional passphrase for encrypted keys
	Password      string      `yaml:"password"`       // Password authentication (not recommended)
	Sources       []SSHSource `yaml:"sources"`        // Log sources on this server
}

// SSHSource defines a log source on a remote server.
type SSHSource struct {
	Path   string `yaml:"path"`   // File path or glob pattern
	Type   string `yaml:"type"`   // Parser type (nginx, apache, magento, etc.)
	Follow bool   `yaml:"follow"` // Tail the file for new content
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

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.setDefaults()
	return cfg
}

// setDefaults sets default values for missing config fields.
func (c *Config) setDefaults() {
	if c.Server.GRPCAddress == "" {
		c.Server.GRPCAddress = ":9443"
	}
	if c.Server.HTTPAddress == "" {
		c.Server.HTTPAddress = ":8080"
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Server.GRPCAddress == "" {
		return fmt.Errorf("server.grpc_address is required")
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("server.tls.cert_file is required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("server.tls.key_file is required when TLS is enabled")
		}
		if c.Server.TLS.ClientCAFile == "" {
			return fmt.Errorf("server.tls.client_ca_file is required when TLS is enabled")
		}
	}

	// Validate SSH connections
	names := make(map[string]bool)
	for i, conn := range c.SSHConnections {
		if conn.Name == "" {
			return fmt.Errorf("ssh_connections[%d].name is required", i)
		}
		if names[conn.Name] {
			return fmt.Errorf("ssh_connections[%d].name '%s' is duplicated", i, conn.Name)
		}
		names[conn.Name] = true

		if conn.Host == "" {
			return fmt.Errorf("ssh_connections[%d].host is required", i)
		}
		if conn.User == "" {
			return fmt.Errorf("ssh_connections[%d].user is required", i)
		}
		if conn.KeyFile == "" && conn.Password == "" {
			return fmt.Errorf("ssh_connections[%d] requires key_file or password", i)
		}
		if len(conn.Sources) == 0 {
			return fmt.Errorf("ssh_connections[%d].sources is required", i)
		}
		for j, src := range conn.Sources {
			if src.Path == "" {
				return fmt.Errorf("ssh_connections[%d].sources[%d].path is required", i, j)
			}
			if src.Type == "" {
				return fmt.Errorf("ssh_connections[%d].sources[%d].type is required", i, j)
			}
		}
	}

	return nil
}
