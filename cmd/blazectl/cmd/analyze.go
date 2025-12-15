package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"text/tabwriter"

	"github.com/good-yellow-bee/blazelog/internal/batch"
	"github.com/spf13/cobra"
)

var (
	analyzeFrom     string
	analyzeTo       string
	analyzeWorkers  int
	analyzeParser   string
	analyzeExport   string
	analyzeExportTo string
	analyzeLimit    int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [file...]",
	Short: "Batch analyze log files",
	Long: `Analyze log files with date range filtering and parallel processing.

Supports glob patterns for analyzing multiple files at once.
Generates summary statistics with optional export to CSV or JSON.

Examples:
  # Analyze logs from January 2024
  blazelog analyze /var/log/*.log --from 2024-01-01 --to 2024-01-31

  # Analyze with JSON output
  blazelog analyze /var/log/nginx/*.log -o json

  # Parallel processing with 8 workers
  blazelog analyze /var/log/*.log --workers 8

  # Export results to CSV
  blazelog analyze /var/log/*.log --export csv --export-to report.csv

  # Specify parser type
  blazelog analyze /var/log/*.log --parser nginx

  # Analyze with verbose progress
  blazelog analyze /var/log/*.log -v`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVar(&analyzeFrom, "from", "", "filter entries after date (YYYY-MM-DD or RFC3339)")
	analyzeCmd.Flags().StringVar(&analyzeTo, "to", "", "filter entries before date (YYYY-MM-DD or RFC3339)")
	analyzeCmd.Flags().IntVar(&analyzeWorkers, "workers", 0, "number of parallel workers (0 = auto)")
	analyzeCmd.Flags().StringVarP(&analyzeParser, "parser", "p", "auto", "parser type (nginx, apache, magento, prestashop, wordpress, auto)")
	analyzeCmd.Flags().StringVar(&analyzeExport, "export", "", "export format (json, csv)")
	analyzeCmd.Flags().StringVar(&analyzeExportTo, "export-to", "", "export file path (default: stdout)")
	analyzeCmd.Flags().IntVarP(&analyzeLimit, "limit", "n", 0, "limit entries per file (0 = no limit)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Parse date flags
	from, err := batch.ParseDateFlag(analyzeFrom)
	if err != nil {
		return fmt.Errorf("invalid --from: %w", err)
	}

	to, err := batch.ParseDateFlagEndOfDay(analyzeTo)
	if err != nil {
		return fmt.Errorf("invalid --to: %w", err)
	}

	// Create analyzer options
	opts := &batch.AnalyzerOptions{
		Workers:    analyzeWorkers,
		From:       from,
		To:         to,
		ParserType: analyzeParser,
		Verbose:    IsVerbose(),
		Limit:      analyzeLimit,
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		PrintVerbose("Received interrupt, stopping...")
		cancel()
	}()

	// Create and run analyzer
	analyzer := batch.NewAnalyzer(opts)

	PrintVerbose("Analyzing files with %d workers...", opts.Workers)

	report, err := analyzer.Analyze(ctx, args)
	if err != nil {
		return err
	}

	// Handle export if requested
	if analyzeExport != "" {
		format, ok := batch.ParseExportFormat(analyzeExport)
		if !ok {
			return fmt.Errorf("invalid export format: %s (use json or csv)", analyzeExport)
		}

		var writer = os.Stdout
		if analyzeExportTo != "" {
			file, err := os.Create(analyzeExportTo)
			if err != nil {
				return fmt.Errorf("create export file: %w", err)
			}
			defer file.Close()
			writer = file
		}

		exporter := batch.NewExporter(format, writer)
		if err := exporter.ExportReport(report); err != nil {
			return fmt.Errorf("export: %w", err)
		}

		if analyzeExportTo != "" {
			PrintVerbose("Report exported to %s", analyzeExportTo)
		}
		return nil
	}

	// Output report based on format
	outputReport(report)
	return nil
}

func outputReport(report *batch.Report) {
	switch GetOutput() {
	case "json":
		outputReportJSON(report)
	case "plain":
		outputReportPlain(report)
	default:
		outputReportTable(report)
	}
}

func outputReportJSON(report *batch.Report) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		PrintError(fmt.Sprintf("failed to marshal JSON: %v", err), false)
		return
	}
	fmt.Println(string(data))
}

func outputReportPlain(report *batch.Report) {
	s := report.Summary
	fmt.Printf("Files: %d | Entries: %d | Errors: %d | Parse Errors: %d\n",
		s.TotalFiles, s.TotalEntries, s.TotalErrors, s.ParseErrors)
	fmt.Printf("Duration: %v | Throughput: %.0f entries/sec\n",
		report.Duration.Round(1e6), s.EntriesPerSec)
}

func outputReportTable(report *batch.Report) {
	s := report.Summary

	// Header
	fmt.Println()
	fmt.Println("Batch Analysis Report")
	fmt.Println("=====================")

	// Date range
	if report.DateRange != nil {
		if report.DateRange.Filtered {
			fmt.Printf("Date Filter: %s → %s\n",
				formatDate(report.DateRange.From),
				formatDate(report.DateRange.To))
		}
		if !report.DateRange.Earliest.IsZero() {
			fmt.Printf("Actual Range: %s → %s\n",
				report.DateRange.Earliest.Format("2006-01-02 15:04:05"),
				report.DateRange.Latest.Format("2006-01-02 15:04:05"))
		}
	}

	fmt.Printf("Files: %d | Duration: %v | Throughput: %.0f entries/sec\n",
		s.TotalFiles, report.Duration.Round(1e6), s.EntriesPerSec)
	fmt.Println()

	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  Total Entries:  %d\n", s.TotalEntries)
	fmt.Printf("  Total Errors:   %d\n", s.TotalErrors)
	fmt.Printf("  Parse Errors:   %d\n", s.ParseErrors)
	fmt.Println()

	// By Level
	if len(s.LevelCounts) > 0 {
		fmt.Println("By Level:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  LEVEL\tCOUNT\t%%\n")
		fmt.Fprintf(w, "  -----\t-----\t-\n")

		// Sort levels for consistent output
		levels := sortedKeys(s.LevelCounts)
		for _, level := range levels {
			count := s.LevelCounts[level]
			pct := s.LevelPercentage(level)
			fmt.Fprintf(w, "  %s\t%d\t%.1f%%\n", level, count, pct)
		}
		w.Flush()
		fmt.Println()
	}

	// By Type
	if len(s.TypeCounts) > 0 {
		fmt.Println("By Type:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  TYPE\tCOUNT\t%%\n")
		fmt.Fprintf(w, "  ----\t-----\t-\n")

		types := sortedKeys(s.TypeCounts)
		for _, typ := range types {
			count := s.TypeCounts[typ]
			pct := s.TypePercentage(typ)
			fmt.Fprintf(w, "  %s\t%d\t%.1f%%\n", typ, count, pct)
		}
		w.Flush()
		fmt.Println()
	}

	// Top Sources (show top 10)
	if len(s.SourceCounts) > 0 && IsVerbose() {
		fmt.Println("Top Sources:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  SOURCE\tCOUNT\n")
		fmt.Fprintf(w, "  ------\t-----\n")

		sources := sortedKeysByValue(s.SourceCounts)
		limit := 10
		if len(sources) < limit {
			limit = len(sources)
		}
		for i := 0; i < limit; i++ {
			src := sources[i]
			fmt.Fprintf(w, "  %s\t%d\n", src, s.SourceCounts[src])
		}
		w.Flush()
		fmt.Println()
	}

	// Errors
	if len(report.Errors) > 0 {
		fmt.Printf("Warnings (%d):\n", len(report.Errors))
		for i, err := range report.Errors {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(report.Errors)-5)
				break
			}
			fmt.Printf("  - %s\n", err)
		}
		fmt.Println()
	}
}

func formatDate(t interface{}) string {
	switch v := t.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysByValue(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]] > m[keys[j]]
	})
	return keys
}
