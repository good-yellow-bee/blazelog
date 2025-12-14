// Package server provides the BlazeLog server implementation.
package server

import (
	"fmt"
	"log"
	"strings"
	"time"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
)

// ANSI color codes for log levels.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

// Processor handles log processing and output.
type Processor struct {
	verbose bool
}

// NewProcessor creates a new log processor.
func NewProcessor(verbose bool) *Processor {
	return &Processor{
		verbose: verbose,
	}
}

// ProcessBatch processes a batch of log entries.
func (p *Processor) ProcessBatch(batch *blazelogv1.LogBatch) error {
	for _, entry := range batch.Entries {
		output := p.formatEntry(entry, batch.AgentId)
		log.Print(output)
	}
	return nil
}

// formatEntry formats a log entry for console output.
func (p *Processor) formatEntry(entry *blazelogv1.LogEntry, agentID string) string {
	// Format timestamp
	var timestamp string
	if entry.Timestamp != nil {
		timestamp = entry.Timestamp.AsTime().Format(time.DateTime)
	} else {
		timestamp = time.Now().Format(time.DateTime)
	}

	// Format level with color
	level := p.formatLevel(entry.Level)

	// Format source (truncate if too long)
	source := entry.Source
	if len(source) > 15 {
		source = source[:15]
	}

	// Format agent ID (use short form)
	agent := agentID
	if len(agent) > 8 {
		agent = agent[:8]
	}

	// Build output string
	var sb strings.Builder
	sb.WriteString(timestamp)
	sb.WriteString(" ")
	sb.WriteString(level)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%-8s", agent))
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%-15s", source))
	sb.WriteString(" ")
	sb.WriteString(entry.Message)

	return sb.String()
}

// formatLevel formats the log level with ANSI colors.
func (p *Processor) formatLevel(level blazelogv1.LogLevel) string {
	switch level {
	case blazelogv1.LogLevel_LOG_LEVEL_DEBUG:
		return colorGray + "[DEBUG]" + colorReset
	case blazelogv1.LogLevel_LOG_LEVEL_INFO:
		return colorBlue + "[INFO] " + colorReset
	case blazelogv1.LogLevel_LOG_LEVEL_WARNING:
		return colorYellow + "[WARN] " + colorReset
	case blazelogv1.LogLevel_LOG_LEVEL_ERROR:
		return colorRed + "[ERROR]" + colorReset
	case blazelogv1.LogLevel_LOG_LEVEL_FATAL:
		return colorRed + "[FATAL]" + colorReset
	default:
		return "[UNKN] "
	}
}
