package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/parser"
	"github.com/spf13/cobra"
)

var (
	parseLimit    int
	parseShowRaw  bool
	parseShowLine bool
)

var parseCmd = &cobra.Command{
	Use:   "parse [type] [file]",
	Short: "Parse log files",
	Long: `Parse log files of various formats.

Supported types:
  nginx      - Nginx access and error logs
  apache     - Apache access and error logs
  magento    - Magento system, exception, debug logs
  prestashop - PrestaShop application logs
  wordpress  - WordPress debug.log and PHP errors
  auto       - Auto-detect log format

Examples:
  # Parse a nginx access log
  blazelog parse nginx /var/log/nginx/access.log

  # Parse with JSON output
  blazelog parse nginx /var/log/nginx/access.log -o json

  # Parse first 10 lines only
  blazelog parse nginx /var/log/nginx/access.log --limit 10

  # Auto-detect log format
  blazelog parse auto /var/log/myapp.log`,
	Args: cobra.MinimumNArgs(2),
	Run:  runParse,
}

func init() {
	rootCmd.AddCommand(parseCmd)

	parseCmd.Flags().IntVarP(&parseLimit, "limit", "n", 0, "limit number of entries to parse (0 = no limit)")
	parseCmd.Flags().BoolVar(&parseShowRaw, "raw", false, "show raw log line")
	parseCmd.Flags().BoolVar(&parseShowLine, "line-numbers", true, "show line numbers")
}

func runParse(cmd *cobra.Command, args []string) {
	logType := args[0]
	filePath := args[1]

	// Get parser for the log type
	p, ok := getParser(logType)
	if !ok {
		PrintError(fmt.Sprintf("unknown log type: %s", logType), true)
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		PrintError(fmt.Sprintf("failed to open file: %v", err), true)
		return
	}
	defer file.Close()

	// Parse the file
	entries := make([]*models.LogEntry, 0)
	scanner := bufio.NewScanner(file)
	lineNum := int64(0)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if line == "" {
			continue
		}

		entry, err := p.Parse(line)
		if err != nil {
			if IsVerbose() {
				PrintVerbose("Line %d: parse error: %v", lineNum, err)
			}
			continue
		}

		entry.LineNumber = lineNum
		entry.FilePath = filePath
		entries = append(entries, entry)

		if parseLimit > 0 && len(entries) >= parseLimit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		PrintError(fmt.Sprintf("error reading file: %v", err), true)
		return
	}

	// Output results
	outputEntries(entries)
}

func getParser(logType string) (parser.Parser, bool) {
	// For now, return a placeholder parser
	// This will be implemented in Milestone 2+
	switch logType {
	case "nginx":
		return nil, false // Will be implemented in Milestone 2
	case "apache":
		return nil, false // Will be implemented in Milestone 3
	case "magento":
		return nil, false // Will be implemented in Milestone 4
	case "prestashop":
		return nil, false // Will be implemented in Milestone 5
	case "wordpress":
		return nil, false // Will be implemented in Milestone 6
	case "auto":
		return nil, false // Will be implemented in Milestone 6
	default:
		return nil, false
	}
}

func outputEntries(entries []*models.LogEntry) {
	if len(entries) == 0 {
		fmt.Println("No entries parsed.")
		return
	}

	switch GetOutput() {
	case "json":
		outputJSON(entries)
	case "plain":
		outputPlain(entries)
	default:
		outputTable(entries)
	}
}

func outputJSON(entries []*models.LogEntry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		PrintError(fmt.Sprintf("failed to marshal JSON: %v", err), true)
		return
	}
	fmt.Println(string(data))
}

func outputPlain(entries []*models.LogEntry) {
	for _, entry := range entries {
		fmt.Println(entry.String())
	}
}

func outputTable(entries []*models.LogEntry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if parseShowLine {
		fmt.Fprintf(w, "LINE\t")
	}
	fmt.Fprintf(w, "TIMESTAMP\tLEVEL\tMESSAGE\n")

	// Separator
	if parseShowLine {
		fmt.Fprintf(w, "----\t")
	}
	fmt.Fprintf(w, "---------\t-----\t-------\n")

	// Entries
	for _, entry := range entries {
		if parseShowLine {
			fmt.Fprintf(w, "%d\t", entry.LineNumber)
		}

		timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
		level := string(entry.Level)
		message := entry.Message

		// Truncate message if too long
		if len(message) > 80 {
			message = message[:77] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", timestamp, level, message)
	}

	w.Flush()

	// Summary
	fmt.Printf("\nTotal entries: %d\n", len(entries))
}
