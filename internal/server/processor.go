// Package server provides the BlazeLog server implementation.
package server

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
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
	verbose   bool
	logBuffer LogBuffer // nil if ClickHouse disabled
}

// NewProcessor creates a new log processor.
func NewProcessor(verbose bool, logBuffer LogBuffer) *Processor {
	return &Processor{
		verbose:   verbose,
		logBuffer: logBuffer,
	}
}

// ProcessBatch processes a batch of log entries.
func (p *Processor) ProcessBatch(batch *blazelogv1.LogBatch) error {
	// Console output
	for _, entry := range batch.Entries {
		output := p.formatEntry(entry, batch.AgentId)
		log.Print(output)
	}

	// ClickHouse insertion via buffer
	if p.logBuffer != nil {
		records := p.convertToRecords(batch)
		if err := p.logBuffer.AddBatch(records); err != nil {
			log.Printf("log buffer error: %v", err)
			// Don't fail the batch - logs already printed
		}
	}

	return nil
}

// convertToRecords converts a proto batch to storage records.
func (p *Processor) convertToRecords(batch *blazelogv1.LogBatch) []*LogRecord {
	records := make([]*LogRecord, 0, len(batch.Entries))
	for _, entry := range batch.Entries {
		var ts time.Time
		if entry.Timestamp != nil {
			ts = entry.Timestamp.AsTime()
		} else {
			ts = time.Now()
		}

		record := &LogRecord{
			ID:         uuid.New().String(),
			Timestamp:  ts,
			Level:      levelToString(entry.Level),
			Message:    entry.Message,
			Source:     entry.Source,
			Type:       typeToString(entry.Type),
			Raw:        entry.Raw,
			AgentID:    batch.AgentId,
			FilePath:   entry.FilePath,
			LineNumber: entry.LineNumber,
			Labels:     entry.Labels,
		}

		// Convert protobuf struct to map
		if entry.Fields != nil {
			record.Fields = entry.Fields.AsMap()
			// Extract denormalized fields for fast filtering
			if status, ok := record.Fields["status"].(float64); ok {
				record.HTTPStatus = int(status)
			}
			if method, ok := record.Fields["method"].(string); ok {
				record.HTTPMethod = method
			}
			if uri, ok := record.Fields["request_uri"].(string); ok {
				record.URI = uri
			}
		}

		records = append(records, record)
	}
	return records
}

// levelToString converts proto LogLevel to string.
func levelToString(level blazelogv1.LogLevel) string {
	switch level {
	case blazelogv1.LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case blazelogv1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case blazelogv1.LogLevel_LOG_LEVEL_WARNING:
		return "warning"
	case blazelogv1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	case blazelogv1.LogLevel_LOG_LEVEL_FATAL:
		return "fatal"
	default:
		return "unknown"
	}
}

// typeToString converts proto LogType to string.
func typeToString(logType blazelogv1.LogType) string {
	switch logType {
	case blazelogv1.LogType_LOG_TYPE_NGINX:
		return "nginx"
	case blazelogv1.LogType_LOG_TYPE_APACHE:
		return "apache"
	case blazelogv1.LogType_LOG_TYPE_MAGENTO:
		return "magento"
	case blazelogv1.LogType_LOG_TYPE_PRESTASHOP:
		return "prestashop"
	case blazelogv1.LogType_LOG_TYPE_WORDPRESS:
		return "wordpress"
	default:
		return "unknown"
	}
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
