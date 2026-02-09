// Package main provides the BlazeLog server CLI.
package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server         ServerConfig     `yaml:"server"`
	API            APIConfig        `yaml:"api"`             // API performance/safety limits
	Metrics        MetricsConfig    `yaml:"metrics"`         // Metrics configuration
	Database       DatabaseConfig   `yaml:"database"`        // Database configuration
	ClickHouse     ClickHouseConfig `yaml:"clickhouse"`      // ClickHouse log storage configuration
	SSHConnections []SSHConnection  `yaml:"ssh_connections"` // SSH connections for remote log collection
	Auth           AuthConfig       `yaml:"auth"`            // Authentication configuration
	Verbose        bool             `yaml:"-"`               // set via CLI flag
}

// MetricsConfig contains Prometheus metrics settings.
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled"` // Enable metrics server (default: true)
	Address    string `yaml:"address"` // Metrics server address (default: :9090)
	enabledSet bool   `yaml:"-"`
}

func (m *MetricsConfig) UnmarshalYAML(value *yaml.Node) error {
	*m = MetricsConfig{}
	var aux struct {
		Enabled *bool  `yaml:"enabled"`
		Address string `yaml:"address"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	if aux.Enabled != nil {
		m.Enabled = *aux.Enabled
		m.enabledSet = true
	}
	m.Address = aux.Address
	return nil
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	JWTSecretEnv     string   `yaml:"jwt_secret_env"`      // Env var name for JWT secret (default: BLAZELOG_JWT_SECRET)
	CSRFSecretEnv    string   `yaml:"csrf_secret_env"`     // Env var name for CSRF secret (optional, for web UI)
	TrustedOrigins   []string `yaml:"trusted_origins"`     // Trusted origins for CSRF (default: localhost:8080, 127.0.0.1:8080)
	TrustedProxies   []string `yaml:"trusted_proxies"`     // Trusted proxy IPs/CIDRs for X-Forwarded-For (empty = don't trust proxy headers)
	UseSecureCookies bool     `yaml:"use_secure_cookies"`  // Use Secure flag for cookies (enable in production with HTTPS)
	AccessTokenTTL   string   `yaml:"access_token_ttl"`    // Access token TTL (default: 15m)
	RefreshTokenTTL  string   `yaml:"refresh_token_ttl"`   // Refresh token TTL (default: 168h / 7 days)
	RateLimitPerIP   int      `yaml:"rate_limit_per_ip"`   // Login rate limit per IP (default: 5/15m)
	RateLimitPerUser int      `yaml:"rate_limit_per_user"` // API rate limit per user (default: 100/min)
	LockoutThreshold int      `yaml:"lockout_threshold"`   // Failed attempts before lockout (default: 5)
	LockoutDuration  string   `yaml:"lockout_duration"`    // Lockout duration (default: 30m)
}

// ClickHouseConfig contains ClickHouse settings.
type ClickHouseConfig struct {
	Enabled          bool           `yaml:"enabled"`            // Enable ClickHouse log storage
	Addresses        []string       `yaml:"addresses"`          // ClickHouse server addresses (host:port)
	Database         string         `yaml:"database"`           // Database name (default: blazelog)
	Username         string         `yaml:"username"`           // Username for authentication
	Password         string         `yaml:"password"`           // Password (use password_env for security)
	PasswordEnv      string         `yaml:"password_env"`       // Environment variable name for password
	MaxOpenConns     int            `yaml:"max_open_conns"`     // Max open connections (default: 5)
	BatchSize        int            `yaml:"batch_size"`         // Batch size for inserts (default: 1000)
	FlushInterval    string         `yaml:"flush_interval"`     // Flush interval (default: 5s)
	MaxBufferSize    int            `yaml:"max_buffer_size"`    // Max buffer size before dropping (default: 100000)
	RetentionDays    int            `yaml:"retention_days"`     // Log retention in days (default: 30)
	RetentionByLevel map[string]int `yaml:"retention_by_level"` // Per-level retention days (e.g., error: 90, debug: 7)
}

// DatabaseConfig contains database settings.
type DatabaseConfig struct {
	Path string `yaml:"path"` // SQLite database file path (default: ./data/blazelog.db)
}

// ServerConfig contains server settings.
type ServerConfig struct {
	GRPCAddress   string        `yaml:"grpc_address"`   // gRPC listen address (default: :9443)
	HTTPAddress   string        `yaml:"http_address"`   // HTTP listen address (default: :8080)
	AllowInsecure bool          `yaml:"allow_insecure"` // Explicitly allow non-TLS operation (development only)
	TLS           TLSConfig     `yaml:"tls"`            // TLS configuration for mTLS
	HTTPTLS       HTTPTLSConfig `yaml:"http_tls"`       // TLS configuration for HTTP API
}

// TLSConfig contains TLS settings for the server.
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`        // Enable mTLS
	CertFile     string `yaml:"cert_file"`      // Server certificate file
	KeyFile      string `yaml:"key_file"`       // Server private key file
	ClientCAFile string `yaml:"client_ca_file"` // CA certificate for verifying client certs
}

// HTTPTLSConfig contains TLS settings for the HTTP API.
type HTTPTLSConfig struct {
	Enabled  bool   `yaml:"enabled"`   // Enable HTTPS
	CertFile string `yaml:"cert_file"` // Server certificate file
	KeyFile  string `yaml:"key_file"`  // Server private key file
}

// APIConfig contains API query and streaming safety limits.
type APIConfig struct {
	MaxQueryRange      string `yaml:"max_query_range"`      // Max allowed query range (default: 24h)
	QueryTimeout       string `yaml:"query_timeout"`        // Per-request storage timeout (default: 10s)
	StreamMaxDuration  string `yaml:"stream_max_duration"`  // SSE stream max duration (default: 30m)
	StreamPollInterval string `yaml:"stream_poll_interval"` // SSE polling interval (default: 1s)
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
	if c.API.MaxQueryRange == "" {
		c.API.MaxQueryRange = "24h"
	}
	if c.API.QueryTimeout == "" {
		c.API.QueryTimeout = "10s"
	}
	if c.API.StreamMaxDuration == "" {
		c.API.StreamMaxDuration = "30m"
	}
	if c.API.StreamPollInterval == "" {
		c.API.StreamPollInterval = "1s"
	}
	if !c.Metrics.enabledSet {
		c.Metrics.Enabled = true
	}
	// Metrics address default
	if c.Metrics.Address == "" {
		c.Metrics.Address = ":9090"
	}
	if c.Database.Path == "" {
		c.Database.Path = "./data/blazelog.db"
	}
	// ClickHouse defaults
	if len(c.ClickHouse.Addresses) == 0 {
		c.ClickHouse.Addresses = []string{"localhost:9000"}
	}
	if c.ClickHouse.Database == "" {
		c.ClickHouse.Database = "blazelog"
	}
	if c.ClickHouse.Username == "" {
		c.ClickHouse.Username = "default"
	}
	if c.ClickHouse.MaxOpenConns == 0 {
		c.ClickHouse.MaxOpenConns = 5
	}
	if c.ClickHouse.BatchSize == 0 {
		c.ClickHouse.BatchSize = 1000
	}
	if c.ClickHouse.FlushInterval == "" {
		c.ClickHouse.FlushInterval = "5s"
	}
	if c.ClickHouse.MaxBufferSize == 0 {
		c.ClickHouse.MaxBufferSize = 100000
	}
	if c.ClickHouse.RetentionDays == 0 {
		c.ClickHouse.RetentionDays = 30
	}
	// Auth defaults
	if c.Auth.JWTSecretEnv == "" {
		c.Auth.JWTSecretEnv = "BLAZELOG_JWT_SECRET"
	}
	if c.Auth.CSRFSecretEnv == "" {
		c.Auth.CSRFSecretEnv = "BLAZELOG_CSRF_SECRET"
	}
	if len(c.Auth.TrustedOrigins) == 0 {
		c.Auth.TrustedOrigins = []string{"localhost:8080", "127.0.0.1:8080"}
	}
	if c.Auth.AccessTokenTTL == "" {
		c.Auth.AccessTokenTTL = "15m"
	}
	if c.Auth.RefreshTokenTTL == "" {
		c.Auth.RefreshTokenTTL = "168h" // 7 days
	}
	if c.Auth.RateLimitPerIP == 0 {
		c.Auth.RateLimitPerIP = 5
	}
	if c.Auth.RateLimitPerUser == 0 {
		c.Auth.RateLimitPerUser = 100
	}
	if c.Auth.LockoutThreshold == 0 {
		c.Auth.LockoutThreshold = 5
	}
	if c.Auth.LockoutDuration == "" {
		c.Auth.LockoutDuration = "30m"
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
	if c.Server.HTTPTLS.Enabled {
		if c.Server.HTTPTLS.CertFile == "" {
			return fmt.Errorf("server.http_tls.cert_file is required when HTTP TLS is enabled")
		}
		if c.Server.HTTPTLS.KeyFile == "" {
			return fmt.Errorf("server.http_tls.key_file is required when HTTP TLS is enabled")
		}
	}
	if !c.Server.AllowInsecure {
		if !c.Server.TLS.Enabled {
			return fmt.Errorf("server.tls.enabled must be true unless server.allow_insecure is true")
		}
		if !c.Server.HTTPTLS.Enabled {
			return fmt.Errorf("server.http_tls.enabled must be true unless server.allow_insecure is true")
		}
	}

	maxQueryRange, err := time.ParseDuration(c.API.MaxQueryRange)
	if err != nil {
		return fmt.Errorf("api.max_query_range: %w", err)
	}
	if maxQueryRange <= 0 {
		return fmt.Errorf("api.max_query_range must be > 0")
	}
	queryTimeout, err := time.ParseDuration(c.API.QueryTimeout)
	if err != nil {
		return fmt.Errorf("api.query_timeout: %w", err)
	}
	if queryTimeout <= 0 {
		return fmt.Errorf("api.query_timeout must be > 0")
	}
	streamMaxDuration, err := time.ParseDuration(c.API.StreamMaxDuration)
	if err != nil {
		return fmt.Errorf("api.stream_max_duration: %w", err)
	}
	if streamMaxDuration <= 0 {
		return fmt.Errorf("api.stream_max_duration must be > 0")
	}
	streamPollInterval, err := time.ParseDuration(c.API.StreamPollInterval)
	if err != nil {
		return fmt.Errorf("api.stream_poll_interval: %w", err)
	}
	if streamPollInterval <= 0 {
		return fmt.Errorf("api.stream_poll_interval must be > 0")
	}
	if streamPollInterval > streamMaxDuration {
		return fmt.Errorf("api.stream_poll_interval must be <= api.stream_max_duration")
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

// WarnSecurityIssues logs warnings for insecure configuration options.
// Should be called during server startup after config is loaded.
func (c *Config) WarnSecurityIssues(logger func(format string, args ...any)) {
	// Warn about plaintext ClickHouse password
	if c.ClickHouse.Password != "" && c.ClickHouse.PasswordEnv == "" {
		logger("SECURITY WARNING: clickhouse.password is set in plaintext in config file. Use clickhouse.password_env instead.")
	}

	// Warn about plaintext SSH passwords
	for i, conn := range c.SSHConnections {
		if conn.Password != "" {
			logger("SECURITY WARNING: ssh_connections[%d].password is set in plaintext. Use key-based authentication instead.", i)
		}
		if conn.KeyPassphrase != "" {
			logger("SECURITY WARNING: ssh_connections[%d].key_passphrase is set in plaintext. Consider using an encrypted key file.", i)
		}
	}

	// Warn if TLS is disabled and insecure mode is explicitly allowed.
	if c.Server.AllowInsecure && !c.Server.TLS.Enabled {
		logger("SECURITY WARNING: TLS is disabled for gRPC. Agent-server communication is not encrypted.")
	}
	if c.Server.AllowInsecure && !c.Server.HTTPTLS.Enabled {
		logger("SECURITY WARNING: HTTP TLS is disabled for the API. Enable HTTPS or terminate TLS at a trusted proxy.")
	}

	// Warn if secure cookies are disabled
	if !c.Auth.UseSecureCookies {
		logger("SECURITY WARNING: use_secure_cookies is disabled. Enable for production with HTTPS.")
	}
}
