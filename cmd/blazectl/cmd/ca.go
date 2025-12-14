package cmd

import (
	"fmt"

	"github.com/good-yellow-bee/blazelog/internal/security"
	"github.com/spf13/cobra"
)

var (
	caOutputDir string
	caValidDays int
)

// caCmd represents the ca command group
var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Certificate Authority management",
	Long:  `Commands for managing the BlazeLog Certificate Authority.`,
}

// caInitCmd represents the ca init command
var caInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Certificate Authority",
	Long: `Initialize a new Certificate Authority for BlazeLog mTLS.

Creates a CA certificate and private key that can be used to sign
server and agent certificates.

Example:
  blazelog ca init --output-dir ./certs
  blazelog ca init --output-dir /etc/blazelog/certs --valid-days 3650`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if caOutputDir == "" {
			return fmt.Errorf("--output-dir is required")
		}

		PrintVerbose("Generating CA certificate...")
		PrintVerbose("  Output directory: %s", caOutputDir)
		PrintVerbose("  Validity: %d days", caValidDays)

		if err := security.GenerateCA(caOutputDir, caValidDays); err != nil {
			return fmt.Errorf("generate CA: %w", err)
		}

		fmt.Printf("CA certificate generated successfully:\n")
		fmt.Printf("  Certificate: %s/ca.crt\n", caOutputDir)
		fmt.Printf("  Private key: %s/ca.key\n", caOutputDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(caCmd)
	caCmd.AddCommand(caInitCmd)

	caInitCmd.Flags().StringVarP(&caOutputDir, "output-dir", "o", "", "output directory for CA files (required)")
	caInitCmd.Flags().IntVarP(&caValidDays, "valid-days", "d", security.DefaultCAValidDays, "certificate validity in days")
	caInitCmd.MarkFlagRequired("output-dir")
}
