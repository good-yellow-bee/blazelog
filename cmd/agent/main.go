package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/good-yellow-bee/blazelog/internal/agent"
	"github.com/good-yellow-bee/blazelog/pkg/config"
	"github.com/spf13/cobra"
)

var (
	configFile string
	serverAddr string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "blazelog-agent",
	Short: "BlazeLog Agent - Log collection agent",
	Long: `BlazeLog Agent collects logs from local files and streams them
to a BlazeLog server for processing and analysis.`,
	RunE: runAgent,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("blazelog-agent %s\n", config.Version)
		fmt.Printf("  commit: %s\n", config.Commit)
		fmt.Printf("  built:  %s\n", config.BuildTime)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "agent.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "", "server address (overrides config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override server address if provided
	if serverAddr != "" {
		cfg.Server.Address = serverAddr
	}

	// Build agent config
	sources := make([]agent.SourceConfig, len(cfg.Sources))
	for i, src := range cfg.Sources {
		sources[i] = agent.SourceConfig{
			Name:   src.Name,
			Type:   src.Type,
			Path:   src.Path,
			Follow: src.Follow,
		}
	}

	agentCfg := &agent.AgentConfig{
		ID:            cfg.Agent.ID,
		Name:          cfg.Agent.Name,
		ServerAddress: cfg.Server.Address,
		BatchSize:     cfg.Agent.BatchSize,
		FlushInterval: cfg.Agent.FlushInterval,
		Sources:       sources,
		Labels:        cfg.Labels,
		Verbose:       verbose,
	}

	// Configure TLS if enabled
	if cfg.Server.TLS.Enabled {
		agentCfg.TLS = &agent.TLSConfig{
			CertFile:           cfg.Server.TLS.CertFile,
			KeyFile:            cfg.Server.TLS.KeyFile,
			CAFile:             cfg.Server.TLS.CAFile,
			InsecureSkipVerify: cfg.Server.TLS.InsecureSkipVerify,
		}
	}

	// Create agent
	a, err := agent.New(agentCfg)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
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

	// Run agent
	log.Printf("starting blazelog-agent %s", config.Version)
	log.Printf("connecting to %s", cfg.Server.Address)
	log.Printf("collecting from %d sources", len(cfg.Sources))

	if err := a.Run(ctx); err != nil {
		return fmt.Errorf("run agent: %w", err)
	}

	log.Printf("agent stopped")
	return nil
}
