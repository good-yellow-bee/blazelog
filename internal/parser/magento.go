// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// maxJSONSize is the maximum size for JSON context/extra in log lines (1MB).
const magentoMaxJSONSize = 1024 * 1024

// MagentoParser parses Magento logs (system.log, exception.log, debug.log).
// Magento uses Monolog format: [YYYY-MM-DD HH:MM:SS] channel.LEVEL: message {context} [extra]
type MagentoParser struct {
	*BaseParser
	// Main regex for parsing log lines
	// Groups: 1=timestamp, 2=channel, 3=level, 4=message_and_context
	regex *regexp.Regexp
	// Regex to detect the start of a new log entry
	startRegex *regexp.Regexp
}

// Magento timestamp format (old style with space separator)
const magentoTimeFormat = "2006-01-02 15:04:05"

// NewMagentoParser creates a new Magento log parser.
func NewMagentoParser(opts *Options) *MagentoParser {
	return &MagentoParser{
		BaseParser: NewBaseParser(opts),
		// Main pattern: [timestamp] channel.LEVEL: message {context} [extra]
		// Supports both old format [2024-01-24 10:00:00] and ISO 8601 [2024-01-24T10:00:00.000000+00:00]
		regex: regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:\d{2})?)\] (\w+)\.(\w+): (.*)$`),
		// Pattern to detect start of a new entry
		startRegex: regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
	}
}

// Parse parses a single Magento log line.
func (p *MagentoParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single Magento log line with context support.
func (p *MagentoParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	matches := p.regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrInvalidFormat
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeMagento

	// Parse timestamp (try ISO 8601 first, then old format)
	tsStr := matches[1]
	var timestamp time.Time
	var err error
	if strings.Contains(tsStr, "T") {
		// ISO 8601 format with T separator
		timestamp, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			// Try without nanoseconds
			timestamp, err = time.Parse(time.RFC3339, tsStr)
		}
	} else {
		// Old format with space separator
		timestamp, err = time.Parse(magentoTimeFormat, tsStr)
	}
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Parse channel
	channel := matches[2]
	entry.SetField("channel", channel)

	// Parse level
	level := strings.ToUpper(matches[3])
	entry.Level = magentoLevelToLogLevel(level)
	entry.SetField("magento_level", level)

	// Parse message and context
	messageAndContext := matches[4]
	message, magentoContext, extra := parseMessageAndContext(messageAndContext)
	entry.Message = message

	// Store context if present
	if magentoContext != nil {
		entry.SetField("context", magentoContext)
		// Check for exception-related fields in context
		if isException, ok := magentoContext["is_exception"].(bool); ok {
			entry.SetField("is_exception", isException)
		}
		if className, ok := magentoContext["class"].(string); ok {
			entry.SetField("exception_class", className)
		}
		if file, ok := magentoContext["file"].(string); ok {
			entry.SetField("exception_file", file)
		}
		if line, ok := magentoContext["line"].(float64); ok {
			entry.SetField("exception_line", int(line))
		}
	}

	// Store extra if present
	if len(extra) > 0 {
		entry.SetField("extra", extra)
	}

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseMessageAndContext extracts the message, context object, and extra array from the log line.
// Format: message {context} [extra] or message [] []
// Magento/Monolog format typically ends with: {} [] or {"key":"value"} [] or [] []
func parseMessageAndContext(s string) (string, map[string]interface{}, []interface{}) {
	s = strings.TrimSpace(s)

	// Reject lines that are too large to prevent unbounded JSON parsing
	if len(s) > magentoMaxJSONSize {
		return fmt.Sprintf("[line truncated: exceeds %d bytes]", magentoMaxJSONSize), nil, nil
	}

	var extra []interface{}
	var magentoContext map[string]interface{}

	// The standard Magento format ends with {context} [extra] or [] []
	// We need to find and parse from the end backwards

	// First, try to find and remove the trailing [] (extra array)
	if strings.HasSuffix(s, " []") {
		s = strings.TrimSuffix(s, " []")
		extra = []interface{}{}
	} else {
		// Try to find a JSON array at the end
		lastBracket := strings.LastIndex(s, " [")
		if lastBracket != -1 {
			possibleExtra := s[lastBracket+1:]
			var parsedExtra []interface{}
			if err := json.Unmarshal([]byte(possibleExtra), &parsedExtra); err == nil {
				extra = parsedExtra
				s = strings.TrimSpace(s[:lastBracket])
			}
		}
	}

	// Now try to find and parse the context - can be {} object or [] array
	if strings.HasSuffix(s, " {}") {
		s = strings.TrimSuffix(s, " {}")
		magentoContext = nil // Empty context object
	} else if strings.HasSuffix(s, " []") {
		// Context can also be an empty array in Monolog format
		s = strings.TrimSuffix(s, " []")
		magentoContext = nil // Empty context (as array)
	} else {
		// Try to find a JSON object at the end
		lastBrace := strings.LastIndex(s, " {")
		if lastBrace != -1 {
			possibleContext := s[lastBrace+1:]
			var parsedContext map[string]interface{}
			if err := json.Unmarshal([]byte(possibleContext), &parsedContext); err == nil {
				magentoContext = parsedContext
				s = strings.TrimSpace(s[:lastBrace])
			}
		}
	}

	return s, magentoContext, extra
}

// magentoLevelToLogLevel converts Magento/Monolog log level to models.LogLevel.
func magentoLevelToLogLevel(level string) models.LogLevel {
	switch level {
	case "DEBUG":
		return models.LevelDebug
	case "INFO", "NOTICE":
		return models.LevelInfo
	case "WARNING":
		return models.LevelWarning
	case "ERROR":
		return models.LevelError
	case "CRITICAL", "ALERT", "EMERGENCY":
		return models.LevelFatal
	default:
		return models.LevelUnknown
	}
}

// Name returns the parser name.
func (p *MagentoParser) Name() string {
	return "magento"
}

// Type returns the log type this parser handles.
func (p *MagentoParser) Type() models.LogType {
	return models.LogTypeMagento
}

// CanParse returns true if the line looks like a Magento log.
func (p *MagentoParser) CanParse(line string) bool {
	return p.regex.MatchString(line)
}

// IsStartOfEntry returns true if the line is the start of a new log entry.
// This is used for multiline parsing (e.g., stack traces).
func (p *MagentoParser) IsStartOfEntry(line string) bool {
	return p.startRegex.MatchString(line)
}

// ParseMultiLine parses multiple lines as a single log entry.
// This handles stack traces and other multiline content in Magento logs.
func (p *MagentoParser) ParseMultiLine(lines []string) (*models.LogEntry, error) {
	if len(lines) == 0 {
		return nil, ErrEmptyLine
	}

	// Parse the first line normally
	entry, err := p.Parse(lines[0])
	if err != nil {
		return nil, err
	}

	// If there are additional lines, they are part of the message (stack trace)
	if len(lines) > 1 {
		// Combine all lines for the raw field
		fullRaw := strings.Join(lines, "\n")

		// Extract stack trace from continuation lines
		stackTraceLines := lines[1:]
		stackTrace := strings.Join(stackTraceLines, "\n")
		entry.SetField("stack_trace", stackTrace)

		// Count stack frames
		frameCount := 0
		for _, line := range stackTraceLines {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				frameCount++
			}
		}
		if frameCount > 0 {
			entry.SetField("stack_frame_count", frameCount)
		}

		// Update raw to include all lines if IncludeRaw is enabled
		if p.options != nil && p.options.IncludeRaw {
			entry.Raw = fullRaw
		}

		// Mark this as a multiline entry
		entry.SetField("multiline", true)
	}

	return entry, nil
}
