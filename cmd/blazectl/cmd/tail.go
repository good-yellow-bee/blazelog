package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/parser"
	"github.com/good-yellow-bee/blazelog/internal/tailer"
	"github.com/spf13/cobra"
)

var (
	tailFollow     bool
	tailParserType string
	tailShowFile   bool
)

var tailCmd = &cobra.Command{
	Use:   "tail [file...]",
	Short: "Tail log files in real-time",
	Long: `Tail one or more log files and display new lines as they're written.

Supports glob patterns for monitoring multiple files at once.
Automatically handles log rotation scenarios.

Examples:
  # Tail a single file
  blazelog tail /var/log/nginx/access.log

  # Tail with follow mode (continuous watching)
  blazelog tail /var/log/nginx/access.log --follow

  # Tail multiple files using glob pattern
  blazelog tail /var/log/nginx/*.log --follow

  # Tail with auto-detection of log format
  blazelog tail /var/log/nginx/error.log --parser auto

  # Tail with JSON output
  blazelog tail /var/log/nginx/access.log -o json --follow`,
	Args: cobra.MinimumNArgs(1),
	Run:  runTail,
}

func init() {
	rootCmd.AddCommand(tailCmd)

	tailCmd.Flags().BoolVarP(&tailFollow, "follow", "f", true, "follow the file(s) and output new lines as they're written")
	tailCmd.Flags().StringVarP(&tailParserType, "parser", "p", "", "parser type to use (nginx, apache, magento, prestashop, wordpress, auto)")
	tailCmd.Flags().BoolVar(&tailShowFile, "show-file", true, "show file path for each line (useful with multiple files)")
}

func runTail(cmd *cobra.Command, args []string) {
	// Expand glob patterns
	patterns := args

	// Create tailer options
	opts := tailer.DefaultOptions()
	opts.Follow = tailFollow
	opts.ReOpen = true
	opts.MustExist = true

	// Determine if we're tailing multiple files
	files := expandGlobs(patterns)
	multiFile := len(files) > 1

	if len(files) == 0 {
		PrintError("no files match the specified patterns", true)
		return
	}

	PrintVerbose("Tailing %d file(s):", len(files))
	for _, f := range files {
		PrintVerbose("  - %s", f)
	}

	// Get parser if specified
	var p parser.Parser
	if tailParserType != "" {
		if tailParserType == "auto" {
			// We'll auto-detect per line
			p = nil
		} else {
			var ok bool
			p, ok = getParser(tailParserType)
			if !ok {
				PrintError(fmt.Sprintf("unknown parser type: %s", tailParserType), true)
				return
			}
		}
	}

	// Create multi-tailer
	mt, err := tailer.NewMultiTailer(patterns, opts)
	if err != nil {
		PrintError(fmt.Sprintf("failed to create tailer: %v", err), true)
		return
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		mt.Stop()
	}()

	// Start tailing
	if err := mt.Start(ctx); err != nil {
		PrintError(fmt.Sprintf("failed to start tailing: %v", err), true)
		return
	}

	if tailFollow {
		fmt.Fprintf(os.Stderr, "Tailing %d file(s). Press Ctrl+C to stop.\n", len(files))
	}

	// Process lines
	for line := range mt.Lines() {
		if line.Err != nil {
			if IsVerbose() {
				PrintVerbose("Error: %v", line.Err)
			}
			continue
		}

		outputLine(line, p, multiFile && tailShowFile)
	}
}

func expandGlobs(patterns []string) []string {
	var files []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			absPath, err := filepath.Abs(match)
			if err != nil {
				continue
			}

			if !seen[absPath] {
				seen[absPath] = true
				files = append(files, absPath)
			}
		}
	}

	return files
}

func outputLine(line tailer.Line, p parser.Parser, showFile bool) {
	outputFormat := GetOutput()

	// If no parser specified, output raw line
	if p == nil && tailParserType != "auto" {
		outputRawLine(line, showFile)
		return
	}

	// Try to parse the line
	var entry *models.LogEntry
	var err error

	if tailParserType == "auto" {
		// Auto-detect parser for this line
		detectedParser, ok := parser.AutoDetect(line.Text)
		if ok {
			entry, err = detectedParser.Parse(line.Text)
		}
	} else if p != nil {
		entry, err = p.Parse(line.Text)
	}

	if err != nil || entry == nil {
		// Could not parse, output raw line
		outputRawLine(line, showFile)
		return
	}

	entry.FilePath = line.FilePath

	switch outputFormat {
	case "json":
		outputJSONLine(entry)
	case "plain":
		outputPlainLine(entry, showFile)
	default:
		outputFormattedLine(entry, showFile)
	}
}

func outputRawLine(line tailer.Line, showFile bool) {
	switch GetOutput() {
	case "json":
		data := map[string]interface{}{
			"text": line.Text,
			"file": line.FilePath,
			"time": line.Time.Format(time.RFC3339),
		}
		jsonData, _ := json.Marshal(data)
		fmt.Println(string(jsonData))
	default:
		if showFile {
			fmt.Printf("[%s] %s\n", filepath.Base(line.FilePath), line.Text)
		} else {
			fmt.Println(line.Text)
		}
	}
}

func outputJSONLine(entry *models.LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Println(entry.Raw)
		return
	}
	fmt.Println(string(data))
}

func outputPlainLine(entry *models.LogEntry, showFile bool) {
	if showFile {
		fmt.Printf("[%s] %s\n", filepath.Base(entry.FilePath), entry.String())
	} else {
		fmt.Println(entry.String())
	}
}

func outputFormattedLine(entry *models.LogEntry, showFile bool) {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	level := string(entry.Level)
	message := entry.Message

	// Truncate message if too long
	if len(message) > 100 {
		message = message[:97] + "..."
	}

	if showFile {
		fmt.Printf("%s [%-7s] [%s] %s\n", timestamp, level, filepath.Base(entry.FilePath), message)
	} else {
		fmt.Printf("%s [%-7s] %s\n", timestamp, level, message)
	}
}
