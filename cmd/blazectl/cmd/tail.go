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

	"github.com/good-yellow-bee/blazelog/internal/alerting"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/notifier"
	"github.com/good-yellow-bee/blazelog/internal/parser"
	"github.com/good-yellow-bee/blazelog/internal/tailer"
	"github.com/spf13/cobra"
)

var (
	tailFollow     bool
	tailParserType string
	tailShowFile   bool

	// Alert flags
	tailAlertRules string

	// Email notification flags
	tailNotifyEmail []string
	tailSMTPHost    string
	tailSMTPPort    int
	tailSMTPUser    string
	tailSMTPFrom    string

	// Slack notification flags
	tailNotifySlack string

	// Teams notification flags
	tailNotifyTeams string
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
  blazelog tail /var/log/nginx/access.log -o json --follow

  # Tail with email notifications on alerts
  blazelog tail /var/log/nginx/*.log \
    --alert-rules ./alerts.yaml \
    --notify-email admin@example.com \
    --smtp-host smtp.gmail.com \
    --smtp-port 587 \
    --smtp-from "BlazeLog <alerts@example.com>"

  # Tail with Slack notifications
  blazelog tail /var/log/nginx/*.log \
    --alert-rules ./alerts.yaml \
    --notify-slack https://hooks.slack.com/services/T00/B00/xxx

  # Tail with Teams notifications
  blazelog tail /var/log/nginx/*.log \
    --alert-rules ./alerts.yaml \
    --notify-teams https://outlook.office.com/webhook/xxx`,
	Args: cobra.MinimumNArgs(1),
	Run:  runTail,
}

func init() {
	rootCmd.AddCommand(tailCmd)

	tailCmd.Flags().BoolVarP(&tailFollow, "follow", "f", true, "follow the file(s) and output new lines as they're written")
	tailCmd.Flags().StringVarP(&tailParserType, "parser", "p", "", "parser type to use (nginx, apache, magento, prestashop, wordpress, auto)")
	tailCmd.Flags().BoolVar(&tailShowFile, "show-file", true, "show file path for each line (useful with multiple files)")

	// Alert flags
	tailCmd.Flags().StringVar(&tailAlertRules, "alert-rules", "", "path to alert rules YAML file")

	// Email notification flags
	tailCmd.Flags().StringSliceVar(&tailNotifyEmail, "notify-email", nil, "email addresses for notifications (can be specified multiple times)")
	tailCmd.Flags().StringVar(&tailSMTPHost, "smtp-host", "", "SMTP server host")
	tailCmd.Flags().IntVar(&tailSMTPPort, "smtp-port", 587, "SMTP server port (587 for STARTTLS, 465 for implicit TLS)")
	tailCmd.Flags().StringVar(&tailSMTPUser, "smtp-user", "", "SMTP username (optional)")
	tailCmd.Flags().StringVar(&tailSMTPFrom, "smtp-from", "", "sender email address")

	// Slack notification flags
	tailCmd.Flags().StringVar(&tailNotifySlack, "notify-slack", "", "Slack webhook URL for notifications")

	// Teams notification flags
	tailCmd.Flags().StringVar(&tailNotifyTeams, "notify-teams", "", "Microsoft Teams webhook URL for notifications")
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

	// Load alert rules if specified
	var engine *alerting.Engine
	if tailAlertRules != "" {
		rules, err := alerting.LoadRulesFromFile(tailAlertRules)
		if err != nil {
			PrintError(fmt.Sprintf("failed to load alert rules: %v", err), true)
			return
		}
		engine = alerting.NewEngine(rules, nil)
		PrintVerbose("Loaded %d alert rule(s)", len(rules))
	}

	// Set up notification dispatcher
	var dispatcher *notifier.Dispatcher
	if len(tailNotifyEmail) > 0 {
		if tailSMTPHost == "" {
			PrintError("--smtp-host is required when using --notify-email", true)
			return
		}
		if tailSMTPFrom == "" {
			PrintError("--smtp-from is required when using --notify-email", true)
			return
		}

		dispatcher = notifier.NewDispatcher()

		emailConfig := notifier.EmailConfig{
			Host:       tailSMTPHost,
			Port:       tailSMTPPort,
			Username:   tailSMTPUser,
			Password:   getSMTPPassword(),
			From:       tailSMTPFrom,
			Recipients: tailNotifyEmail,
		}

		emailNotifier, err := notifier.NewEmailNotifier(emailConfig)
		if err != nil {
			PrintError(fmt.Sprintf("failed to create email notifier: %v", err), true)
			return
		}
		dispatcher.Register(emailNotifier)
		PrintVerbose("Email notifications enabled for: %v", tailNotifyEmail)
	}

	// Set up Slack notifications
	if tailNotifySlack != "" {
		if dispatcher == nil {
			dispatcher = notifier.NewDispatcher()
		}

		slackConfig := notifier.SlackConfig{
			WebhookURL: tailNotifySlack,
		}

		slackNotifier, err := notifier.NewSlackNotifier(slackConfig)
		if err != nil {
			PrintError(fmt.Sprintf("failed to create slack notifier: %v", err), true)
			return
		}
		dispatcher.Register(slackNotifier)
		PrintVerbose("Slack notifications enabled")
	}

	// Set up Teams notifications
	if tailNotifyTeams != "" {
		if dispatcher == nil {
			dispatcher = notifier.NewDispatcher()
		}

		teamsConfig := notifier.TeamsConfig{
			WebhookURL: tailNotifyTeams,
		}

		teamsNotifier, err := notifier.NewTeamsNotifier(teamsConfig)
		if err != nil {
			PrintError(fmt.Sprintf("failed to create teams notifier: %v", err), true)
			return
		}
		dispatcher.Register(teamsNotifier)
		PrintVerbose("Teams notifications enabled")
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
		if engine != nil {
			engine.Close()
		}
		if dispatcher != nil {
			dispatcher.Close()
		}
	}()

	// Start alert consumer goroutine if engine is configured
	if engine != nil && dispatcher != nil {
		go consumeAlerts(ctx, engine, dispatcher)
	}

	// Start tailing
	if err := mt.Start(ctx); err != nil {
		PrintError(fmt.Sprintf("failed to start tailing: %v", err), true)
		return
	}

	if tailFollow {
		fmt.Fprintf(os.Stderr, "Tailing %d file(s). Press Ctrl+C to stop.\n", len(files))
		if engine != nil {
			fmt.Fprintf(os.Stderr, "Alert rules active. Notifications will be sent on matches.\n")
		}
	}

	// Process lines
	for line := range mt.Lines() {
		if line.Err != nil {
			if IsVerbose() {
				PrintVerbose("Error: %v", line.Err)
			}
			continue
		}

		// Process for output and alerting
		entry := processLine(line, p, multiFile && tailShowFile)

		// Evaluate against alert rules if engine is configured
		if engine != nil && entry != nil {
			engine.Evaluate(entry)
		}
	}
}

// processLine parses and outputs a line, returning the parsed entry for alerting.
func processLine(line tailer.Line, p parser.Parser, showFile bool) *models.LogEntry {
	outputFormat := GetOutput()

	// If no parser specified, output raw line
	if p == nil && tailParserType != "auto" {
		outputRawLine(line, showFile)
		return nil
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
		return nil
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

	return entry
}

// consumeAlerts reads alerts from the engine and dispatches notifications.
func consumeAlerts(ctx context.Context, engine *alerting.Engine, dispatcher *notifier.Dispatcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case alert, ok := <-engine.Alerts():
			if !ok {
				return
			}

			// Log the alert
			PrintVerbose("Alert triggered: %s (severity: %s)", alert.RuleName, alert.Severity)

			// Dispatch to all registered notifiers
			if err := dispatcher.DispatchAll(ctx, alert); err != nil {
				PrintVerbose("Notification error: %v", err)
			}
		}
	}
}

// getSMTPPassword returns the SMTP password from environment variable.
func getSMTPPassword() string {
	return os.Getenv("BLAZELOG_SMTP_PASS")
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
