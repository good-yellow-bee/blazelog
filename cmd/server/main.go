package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api"
	"github.com/good-yellow-bee/blazelog/internal/api/health"
	"github.com/good-yellow-bee/blazelog/internal/metrics"
	"github.com/good-yellow-bee/blazelog/internal/server"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/pkg/config"
	"github.com/spf13/cobra"
)

var (
	configFile string
	grpcAddr   string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "blazelog-server",
	Short: "BlazeLog Server - Central log processing server",
	Long: `BlazeLog Server receives logs from agents, processes them,
and provides a central point for log management and analysis.`,
	RunE: runServer,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("blazelog-server %s\n", config.Version)
		fmt.Printf("  commit: %s\n", config.Commit)
		fmt.Printf("  built:  %s\n", config.BuildTime)
	},
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server health (for Docker/k8s probes)",
	RunE:  runHealthCheck,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (optional)")
	rootCmd.PersistentFlags().StringVarP(&grpcAddr, "address", "a", ":9443", "gRPC listen address")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(healthCmd)

	healthCmd.Flags().String("url", "http://localhost:8080/health/ready", "health endpoint URL")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runHealthCheck performs HTTP health check for Docker/k8s probes.
func runHealthCheck(cmd *cobra.Command, args []string) error {
	url, _ := cmd.Flags().GetString("url")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}

	fmt.Println("healthy")
	return nil
}

func runServer(cmd *cobra.Command, args []string) error {
	var cfg *Config

	// Load configuration from file if provided
	if configFile != "" {
		var err error
		cfg, err = LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	} else {
		cfg = DefaultConfig()
	}

	// Override with CLI flags
	if grpcAddr != "" {
		cfg.Server.GRPCAddress = grpcAddr
	}
	cfg.Verbose = verbose

	// Get master key from environment
	masterKey := os.Getenv("BLAZELOG_MASTER_KEY")
	if masterKey == "" {
		return fmt.Errorf("BLAZELOG_MASTER_KEY environment variable is required")
	}

	// Auto-create data directory
	dbDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Initialize storage
	store := storage.NewSQLiteStorage(cfg.Database.Path, []byte(masterKey))
	if err := store.Open(); err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Create default admin user on first run
	if err := store.EnsureAdminUser(); err != nil {
		return fmt.Errorf("ensure admin user: %w", err)
	}

	log.Printf("database initialized at %s", cfg.Database.Path)

	// Initialize ClickHouse storage (if enabled)
	var logBuffer *storage.LogBuffer
	var logStore storage.LogStorage
	if cfg.ClickHouse.Enabled {
		var chErr error
		logBuffer, logStore, chErr = initClickHouse(cfg)
		if chErr != nil {
			return fmt.Errorf("init clickhouse: %w", chErr)
		}
		defer logBuffer.Close()
		defer logStore.Close()
	}

	// Build server config
	serverCfg := &server.Config{
		GRPCAddress: cfg.Server.GRPCAddress,
		Verbose:     cfg.Verbose,
	}

	// Pass LogBuffer to server if ClickHouse enabled
	if logBuffer != nil {
		serverCfg.LogBuffer = &logBufferAdapter{logBuffer}
	}

	// Configure TLS if enabled
	if cfg.Server.TLS.Enabled {
		serverCfg.TLS = &server.TLSConfig{
			CertFile:     cfg.Server.TLS.CertFile,
			KeyFile:      cfg.Server.TLS.KeyFile,
			ClientCAFile: cfg.Server.TLS.ClientCAFile,
		}
	}

	// Create gRPC server
	srv, err := server.New(serverCfg)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	// Initialize HTTP API server
	apiServer, err := initAPIServer(cfg, store, logStore)
	if err != nil {
		return fmt.Errorf("init api server: %w", err)
	}

	// Register health checkers
	apiServer.RegisterHealthChecker(health.NewSQLiteChecker(store.DB()))
	if logStore != nil {
		apiServer.RegisterHealthChecker(health.NewClickHouseChecker(logStore))
	}

	// Initialize metrics server (if enabled)
	var metricsServer *metrics.Server
	if cfg.Metrics.Enabled {
		metrics.SetBuildInfo(config.Version, config.Commit, config.BuildTime)
		metricsServer = metrics.NewServer(cfg.Metrics.Address)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	// Run servers
	log.Printf("starting blazelog-server %s", config.Version)
	log.Printf("gRPC listening on %s", cfg.Server.GRPCAddress)

	errChan := make(chan error, 3)

	// Start gRPC server
	go func() {
		if err := srv.Run(ctx); err != nil {
			errChan <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	// Start HTTP API server
	go func() {
		if err := apiServer.Run(ctx); err != nil {
			errChan <- fmt.Errorf("api server: %w", err)
		}
	}()

	// Start metrics server (if enabled)
	if metricsServer != nil {
		go func() {
			if err := metricsServer.Start(); err != nil {
				errChan <- fmt.Errorf("metrics server: %w", err)
			}
		}()
	}

	// Wait for shutdown or error
	select {
	case <-ctx.Done():
		// Gracefully shutdown metrics server
		if metricsServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := metricsServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("metrics server shutdown error: %v", err)
			}
		}
		log.Printf("server stopped")
	case err := <-errChan:
		cancel()
		return err
	}

	return nil
}

// initAPIServer initializes the HTTP API server.
func initAPIServer(cfg *Config, store storage.Storage, logStore storage.LogStorage) (*api.Server, error) {
	// Get JWT secret
	jwtSecret := os.Getenv(cfg.Auth.JWTSecretEnv)
	if jwtSecret == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.Auth.JWTSecretEnv)
	}

	// Get CSRF secret (optional, for web UI)
	csrfSecret := ""
	if cfg.Auth.CSRFSecretEnv != "" {
		csrfSecret = os.Getenv(cfg.Auth.CSRFSecretEnv)
	}

	// Parse durations
	accessTTL, err := time.ParseDuration(cfg.Auth.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("parse access_token_ttl: %w", err)
	}
	refreshTTL, err := time.ParseDuration(cfg.Auth.RefreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("parse refresh_token_ttl: %w", err)
	}
	lockoutDuration, err := time.ParseDuration(cfg.Auth.LockoutDuration)
	if err != nil {
		return nil, fmt.Errorf("parse lockout_duration: %w", err)
	}

	apiConfig := &api.Config{
		Address:          cfg.Server.HTTPAddress,
		JWTSecret:        []byte(jwtSecret),
		CSRFSecret:       csrfSecret,
		AccessTokenTTL:   accessTTL,
		RefreshTokenTTL:  refreshTTL,
		RateLimitPerIP:   cfg.Auth.RateLimitPerIP,
		RateLimitPerUser: cfg.Auth.RateLimitPerUser,
		LockoutThreshold: cfg.Auth.LockoutThreshold,
		LockoutDuration:  lockoutDuration,
		Verbose:          cfg.Verbose,
	}

	return api.New(apiConfig, store, logStore)
}

// initClickHouse initializes ClickHouse storage and returns a LogBuffer and LogStorage.
func initClickHouse(cfg *Config) (*storage.LogBuffer, storage.LogStorage, error) {
	// Parse flush interval
	flushInterval, err := time.ParseDuration(cfg.ClickHouse.FlushInterval)
	if err != nil {
		return nil, nil, fmt.Errorf("parse flush_interval: %w", err)
	}

	// Get password from env if specified
	password := cfg.ClickHouse.Password
	if cfg.ClickHouse.PasswordEnv != "" {
		password = os.Getenv(cfg.ClickHouse.PasswordEnv)
	}

	// Create ClickHouse config
	chConfig := &storage.ClickHouseConfig{
		Addresses:     cfg.ClickHouse.Addresses,
		Database:      cfg.ClickHouse.Database,
		Username:      cfg.ClickHouse.Username,
		Password:      password,
		MaxOpenConns:  cfg.ClickHouse.MaxOpenConns,
		MaxIdleConns:  cfg.ClickHouse.MaxOpenConns,
		DialTimeout:   5 * time.Second,
		Compression:   true,
		RetentionDays: cfg.ClickHouse.RetentionDays,
	}

	// Initialize ClickHouse storage
	logStorage := storage.NewClickHouseStorage(chConfig)
	if err := logStorage.Open(); err != nil {
		return nil, nil, fmt.Errorf("open clickhouse: %w", err)
	}

	if err := logStorage.Migrate(); err != nil {
		logStorage.Close()
		return nil, nil, fmt.Errorf("migrate clickhouse: %w", err)
	}

	log.Printf("clickhouse initialized at %v (database: %s)", cfg.ClickHouse.Addresses, cfg.ClickHouse.Database)

	// Create LogBuffer
	bufferConfig := &storage.LogBufferConfig{
		BatchSize:     cfg.ClickHouse.BatchSize,
		FlushInterval: flushInterval,
		MaxSize:       cfg.ClickHouse.MaxBufferSize,
	}
	logBuffer := storage.NewLogBuffer(logStorage.Logs(), bufferConfig)

	return logBuffer, logStorage, nil
}

// logBufferAdapter adapts storage.LogBuffer to server.LogBuffer interface.
type logBufferAdapter struct {
	buffer *storage.LogBuffer
}

func (a *logBufferAdapter) AddBatch(entries []*server.LogRecord) error {
	// Convert server.LogRecord to storage.LogRecord
	records := make([]*storage.LogRecord, len(entries))
	for i, e := range entries {
		records[i] = &storage.LogRecord{
			ID:         e.ID,
			Timestamp:  e.Timestamp,
			Level:      e.Level,
			Message:    e.Message,
			Source:     e.Source,
			Type:       e.Type,
			Raw:        e.Raw,
			AgentID:    e.AgentID,
			FilePath:   e.FilePath,
			LineNumber: e.LineNumber,
			Fields:     e.Fields,
			Labels:     e.Labels,
			HTTPStatus: e.HTTPStatus,
			HTTPMethod: e.HTTPMethod,
			URI:        e.URI,
		}
	}
	return a.buffer.AddBatch(records)
}

func (a *logBufferAdapter) Close() error {
	return a.buffer.Close()
}
