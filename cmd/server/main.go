package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (optional)")
	rootCmd.PersistentFlags().StringVarP(&grpcAddr, "address", "a", ":9443", "gRPC listen address")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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

	// Build server config
	serverCfg := &server.Config{
		GRPCAddress: cfg.Server.GRPCAddress,
		Verbose:     cfg.Verbose,
	}

	// Configure TLS if enabled
	if cfg.Server.TLS.Enabled {
		serverCfg.TLS = &server.TLSConfig{
			CertFile:     cfg.Server.TLS.CertFile,
			KeyFile:      cfg.Server.TLS.KeyFile,
			ClientCAFile: cfg.Server.TLS.ClientCAFile,
		}
	}

	// Create server
	srv, err := server.New(serverCfg)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
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

	// Run server
	log.Printf("starting blazelog-server %s", config.Version)
	log.Printf("gRPC listening on %s", cfg.Server.GRPCAddress)

	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	log.Printf("server stopped")
	return nil
}
