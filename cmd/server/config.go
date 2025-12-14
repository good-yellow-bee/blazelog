// Package main provides the BlazeLog server CLI.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server  ServerConfig `yaml:"server"`
	Verbose bool         `yaml:"-"` // set via CLI flag
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
	return nil
}
