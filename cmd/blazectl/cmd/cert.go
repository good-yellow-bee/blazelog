package cmd

import (
	"fmt"
	"strings"

	"github.com/good-yellow-bee/blazelog/internal/security"
	"github.com/spf13/cobra"
)

var (
	certCADir     string
	certName      string
	certOutputDir string
	certValidDays int
	certHosts     string
)

// certCmd represents the cert command group
var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Certificate management",
	Long:  `Commands for generating server and agent certificates.`,
}

// certServerCmd represents the cert server command
var certServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Generate a server certificate",
	Long: `Generate a server certificate signed by the CA.

The certificate will include localhost and any additional hosts
specified with --hosts in the Subject Alternative Names (SAN).

Example:
  blazelog cert server --ca-dir ./certs --name server --out ./certs
  blazelog cert server --ca-dir ./certs --name server --out ./certs --hosts blazelog.local,192.168.1.100`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateCertFlags(); err != nil {
			return err
		}

		hosts := parseHosts(certHosts)

		PrintVerbose("Generating server certificate...")
		PrintVerbose("  CA directory: %s", certCADir)
		PrintVerbose("  Name: %s", certName)
		PrintVerbose("  Output directory: %s", certOutputDir)
		PrintVerbose("  Validity: %d days", certValidDays)
		PrintVerbose("  Additional hosts: %v", hosts)

		if err := security.GenerateServerCert(certCADir, certName, certOutputDir, certValidDays, hosts); err != nil {
			return fmt.Errorf("generate server certificate: %w", err)
		}

		fmt.Printf("Server certificate generated successfully:\n")
		fmt.Printf("  Certificate: %s/%s.crt\n", certOutputDir, certName)
		fmt.Printf("  Private key: %s/%s.key\n", certOutputDir, certName)
		return nil
	},
}

// certAgentCmd represents the cert agent command
var certAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Generate an agent certificate",
	Long: `Generate an agent certificate signed by the CA.

The certificate will include the local hostname in the
Subject Alternative Names (SAN).

Example:
  blazelog cert agent --ca-dir ./certs --name agent1 --out ./certs
  blazelog cert agent --ca-dir ./certs --name web-server-1 --out ./certs --valid-days 365`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateCertFlags(); err != nil {
			return err
		}

		PrintVerbose("Generating agent certificate...")
		PrintVerbose("  CA directory: %s", certCADir)
		PrintVerbose("  Name: %s", certName)
		PrintVerbose("  Output directory: %s", certOutputDir)
		PrintVerbose("  Validity: %d days", certValidDays)

		if err := security.GenerateAgentCert(certCADir, certName, certOutputDir, certValidDays); err != nil {
			return fmt.Errorf("generate agent certificate: %w", err)
		}

		fmt.Printf("Agent certificate generated successfully:\n")
		fmt.Printf("  Certificate: %s/%s.crt\n", certOutputDir, certName)
		fmt.Printf("  Private key: %s/%s.key\n", certOutputDir, certName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(certCmd)
	certCmd.AddCommand(certServerCmd)
	certCmd.AddCommand(certAgentCmd)

	// Common flags for both server and agent
	for _, cmd := range []*cobra.Command{certServerCmd, certAgentCmd} {
		cmd.Flags().StringVar(&certCADir, "ca-dir", "", "directory containing CA certificate and key (required)")
		cmd.Flags().StringVar(&certName, "name", "", "certificate name (required)")
		cmd.Flags().StringVar(&certOutputDir, "out", "", "output directory for certificate files (required)")
		cmd.Flags().IntVar(&certValidDays, "valid-days", security.DefaultCertValidDays, "certificate validity in days")
		cmd.MarkFlagRequired("ca-dir")
		cmd.MarkFlagRequired("name")
		cmd.MarkFlagRequired("out")
	}

	// Server-specific flags
	certServerCmd.Flags().StringVar(&certHosts, "hosts", "", "comma-separated list of additional hostnames/IPs for SAN")
}

func validateCertFlags() error {
	if certCADir == "" {
		return fmt.Errorf("--ca-dir is required")
	}
	if certName == "" {
		return fmt.Errorf("--name is required")
	}
	if certOutputDir == "" {
		return fmt.Errorf("--out is required")
	}
	return nil
}

func parseHosts(hosts string) []string {
	if hosts == "" {
		return nil
	}
	parts := strings.Split(hosts, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
