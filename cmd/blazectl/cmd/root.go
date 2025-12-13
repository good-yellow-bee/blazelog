// Package cmd contains the CLI commands for blazelog.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Used for flags
	verbose bool
	output  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blazelog",
	Short: "BlazeLog - Universal Log Analyzer",
	Long: `BlazeLog is a fast, powerful, and secure universal log analyzer
built in Go with multi-platform support.

Supported log formats:
  - Nginx (access and error logs)
  - Apache (access and error logs)
  - Magento (system, exception, debug logs)
  - PrestaShop (application logs)
  - WordPress (debug.log, PHP errors)

Features:
  - Real-time log tailing and analysis
  - Pattern-based and threshold-based alerting
  - Notifications via Email, Slack, and Teams
  - Distributed log collection with secure agents
  - Web-based dashboard

Examples:
  # Parse a nginx log file
  blazelog parse nginx /var/log/nginx/access.log

  # Tail a log file and watch for errors
  blazelog tail /var/log/nginx/error.log --follow

  # Auto-detect log format
  blazelog parse auto /var/log/*.log`,
	// Run when no subcommand is specified
	Run: func(cmd *cobra.Command, args []string) {
		// Show help by default
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format (table, json, plain)")
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}

// GetOutput returns the output format.
func GetOutput() string {
	return output
}

// PrintError prints an error message and exits if fatal is true.
func PrintError(msg string, fatal bool) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	if fatal {
		os.Exit(1)
	}
}

// PrintVerbose prints a message only if verbose mode is enabled.
func PrintVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}
