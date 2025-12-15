// Package api provides the HTTP REST API server.
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/health"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Config contains HTTP API server configuration.
type Config struct {
	Address          string
	JWTSecret        []byte
	CSRFSecret       string // For web UI CSRF protection
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	RateLimitPerIP   int
	RateLimitPerUser int
	LockoutThreshold int
	LockoutDuration  time.Duration
	Verbose          bool
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
		c.RateLimitPerIP = 5 // 5 requests per minute
	}
	if c.RateLimitPerUser == 0 {
		c.RateLimitPerUser = 100 // 100 requests per minute
	}
	if c.LockoutThreshold == 0 {
		c.LockoutThreshold = 5 // 5 failed attempts
	}
	if c.LockoutDuration == 0 {
		c.LockoutDuration = 15 * time.Minute
	}
}

// Server is the HTTP API server.
type Server struct {
	config        *Config
	storage       storage.Storage
	logStorage    storage.LogStorage
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

	s := &Server{
		config:        cfg,
		storage:       store,
		logStorage:    logStore,
		healthHandler: health.NewHandler(),
	}

	router := s.setupRouter()

	s.server = &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Run starts the HTTP server and blocks until context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		log.Printf("HTTP API listening on %s", s.config.Address)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutting down HTTP API server...")
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
