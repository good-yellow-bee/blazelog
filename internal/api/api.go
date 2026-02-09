// Package api provides the HTTP REST API server.
package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/health"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

// Config contains HTTP API server configuration.
type Config struct {
	Address            string
	JWTSecret          []byte
	CSRFSecret         string   // For web UI CSRF protection
	TrustedOrigins     []string // Trusted origins for CSRF (e.g., "localhost:8080")
	TrustedProxies     []string // Trusted proxy IPs/CIDRs for X-Forwarded-For
	WebUIEnabled       bool     // Enable web UI (default: true)
	UseSecureCookies   bool     // Use Secure flag for cookies (true in production with HTTPS)
	HTTPTLSEnabled     bool     // Enable HTTPS for API server
	HTTPTLSCertFile    string   // HTTPS certificate file
	HTTPTLSKeyFile     string   // HTTPS private key file
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	RateLimitPerIP     int
	RateLimitPerUser   int
	LockoutThreshold   int
	LockoutDuration    time.Duration
	MaxQueryRange      time.Duration // Max allowed logs query range
	QueryTimeout       time.Duration // Timeout for storage-backed API calls
	StreamMaxDuration  time.Duration // Max lifetime for log stream connections
	StreamPollInterval time.Duration // Poll interval for stream query loop
	Verbose            bool
}

// SetDefaults applies default values for missing configuration.
func (c *Config) SetDefaults() {
	if c.Address == "" {
		c.Address = ":8080"
	}
	if c.AccessTokenTTL == 0 {
		c.AccessTokenTTL = 15 * time.Minute
	}
	if c.RefreshTokenTTL == 0 {
		c.RefreshTokenTTL = 7 * 24 * time.Hour // 7 days
	}
	if c.RateLimitPerIP == 0 {
		c.RateLimitPerIP = 5 // 5 requests per 15 minutes
	}
	if c.RateLimitPerUser == 0 {
		c.RateLimitPerUser = 100 // 100 requests per minute
	}
	if c.LockoutThreshold == 0 {
		c.LockoutThreshold = 5 // 5 failed attempts
	}
	if c.LockoutDuration == 0 {
		c.LockoutDuration = 30 * time.Minute
	}
	if c.MaxQueryRange == 0 {
		c.MaxQueryRange = 24 * time.Hour
	}
	if c.QueryTimeout == 0 {
		c.QueryTimeout = 10 * time.Second
	}
	if c.StreamMaxDuration == 0 {
		c.StreamMaxDuration = 30 * time.Minute
	}
	if c.StreamPollInterval == 0 {
		c.StreamPollInterval = time.Second
	}
}

// Server is the HTTP API server.
type Server struct {
	config        *Config
	storage       storage.Storage
	logStorage    storage.LogStorage
	sessions      *session.Store
	server        *http.Server
	healthHandler *health.Handler
}

// New creates a new API server.
// logStore can be nil if ClickHouse is disabled.
func New(cfg *Config, store storage.Storage, logStore storage.LogStorage) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("storage is required")
	}
	if len(cfg.JWTSecret) == 0 {
		return nil, fmt.Errorf("JWT secret is required")
	}

	cfg.SetDefaults()

	// Create session store for web UI authentication (24 hour TTL)
	sessions := session.NewStore(24 * time.Hour)

	s := &Server{
		config:        cfg,
		storage:       store,
		logStorage:    logStore,
		sessions:      sessions,
		healthHandler: health.NewHandler(),
	}

	router := s.setupRouter()

	s.server = &http.Server{
		Addr:        cfg.Address,
		Handler:     router,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout is intentionally 0 (disabled) because the server
		// supports SSE streams that can last up to 30 minutes. A global
		// WriteTimeout would prematurely kill those long-lived connections.
		// Individual non-streaming handlers use http.TimeoutHandler or
		// context deadlines to bound response time.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}
	if cfg.HTTPTLSEnabled {
		s.server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
	}

	return s, nil
}

// Sessions returns the session store for web UI integration.
func (s *Server) Sessions() *session.Store {
	return s.sessions
}

// Run starts the HTTP server and blocks until context is canceled.
func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		log.Printf("HTTP API listening on %s", s.config.Address)
		var err error
		if s.config.HTTPTLSEnabled {
			err = s.server.ListenAndServeTLS(s.config.HTTPTLSCertFile, s.config.HTTPTLSKeyFile)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutting down HTTP API server...")
		s.sessions.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// Address returns the configured listen address.
func (s *Server) Address() string {
	return s.config.Address
}

// RegisterHealthChecker adds a health checker to the server.
func (s *Server) RegisterHealthChecker(c health.Checker) {
	if s.healthHandler != nil {
		s.healthHandler.RegisterChecker(c)
	}
}
