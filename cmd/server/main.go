package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/good-yellow-bee/blazelog/internal/server"
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

	// Build server config
	serverCfg := &server.Config{
		GRPCAddress: cfg.Server.GRPCAddress,
		Verbose:     cfg.Verbose,
	}

	// Create server
	srv := server.New(serverCfg)

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
