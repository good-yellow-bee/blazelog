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
const prestashopMaxJSONSize = 1024 * 1024

// PrestaShopParser parses PrestaShop logs (dev.log, prod.log).
// PrestaShop uses Symfony/Monolog format: [YYYY-MM-DD HH:MM:SS] channel.LEVEL: message {context} [extra]
type PrestaShopParser struct {
	*BaseParser
	// Main regex for parsing log lines
	// Groups: 1=timestamp, 2=channel, 3=level, 4=message_and_context
	regex *regexp.Regexp
	// Regex to detect the start of a new log entry
	startRegex *regexp.Regexp
}

// PrestaShop timestamp format (same as Magento, using Monolog default)
const prestashopTimeFormat = "2006-01-02 15:04:05"

// NewPrestaShopParser creates a new PrestaShop log parser.
func NewPrestaShopParser(opts *Options) *PrestaShopParser {
	return &PrestaShopParser{
		BaseParser: NewBaseParser(opts),
		// Main pattern: [timestamp] channel.LEVEL: message {context} [extra]
		// The message can contain anything including JSON objects
		regex: regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+)\.(\w+): (.*)$`),
		// Pattern to detect start of a new entry
		startRegex: regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]`),
	}
}

// Parse parses a single PrestaShop log line.
func (p *PrestaShopParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single PrestaShop log line with context support.
func (p *PrestaShopParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	matches := p.regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrInvalidFormat
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypePrestaShop

	// Parse timestamp
	timestamp, err := time.Parse(prestashopTimeFormat, matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Parse channel (e.g., request, php, security, console, app)
	channel := matches[2]
	entry.SetField("channel", channel)

	// Parse level
	level := strings.ToUpper(matches[3])
	entry.Level = prestashopLevelToLogLevel(level)
	entry.SetField("prestashop_level", level)

	// Parse message and context
	messageAndContext := matches[4]
	message, prestashopContext, extra := parsePrestashopMessageAndContext(messageAndContext)
	entry.Message = message

	// Store context if present
	if prestashopContext != nil {
		entry.SetField("context", prestashopContext)
		// Check for exception-related fields in context
		if exceptionClass, ok := prestashopContext["exception"].(map[string]interface{}); ok {
			if class, ok := exceptionClass["class"].(string); ok {
				entry.SetField("exception_class", class)
			}
			if file, ok := exceptionClass["file"].(string); ok {
				entry.SetField("exception_file", file)
			}
			if line, ok := exceptionClass["line"].(float64); ok {
				entry.SetField("exception_line", int(line))
			}
		}
		// Handle Doctrine DBAL exceptions
		if _, ok := prestashopContext["exception"]; ok {
			entry.SetField("is_exception", true)
		}
		// Handle request-specific fields
		if uri, ok := prestashopContext["uri"].(string); ok {
			entry.SetField("uri", uri)
		}
		if method, ok := prestashopContext["method"].(string); ok {
			entry.SetField("method", method)
		}
	}

	// Store extra if present
	if len(extra) > 0 {
		entry.SetField("extra", extra)
	}

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parsePrestashopMessageAndContext extracts the message, context object, and extra array from the log line.
// Format: message {context} [extra] or message [] []
// Symfony/Monolog format typically ends with: {} [] or {"key":"value"} [] or [] []
func parsePrestashopMessageAndContext(s string) (string, map[string]interface{}, []interface{}) {
	s = strings.TrimSpace(s)

	// Reject lines that are too large to prevent unbounded JSON parsing
	if len(s) > prestashopMaxJSONSize {
		return fmt.Sprintf("[line truncated: exceeds %d bytes]", prestashopMaxJSONSize), nil, nil
	}

	var extra []interface{}
	var prestashopContext map[string]interface{}

	// The standard Symfony format ends with {context} [extra] or [] []
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
		prestashopContext = nil // Empty context object
	} else if strings.HasSuffix(s, " []") {
		// Context can also be an empty array in Monolog format
		s = strings.TrimSuffix(s, " []")
		prestashopContext = nil // Empty context (as array)
	} else {
		// Try to find a JSON object at the end
		lastBrace := strings.LastIndex(s, " {")
		if lastBrace != -1 {
			possibleContext := s[lastBrace+1:]
			var parsedContext map[string]interface{}
			if err := json.Unmarshal([]byte(possibleContext), &parsedContext); err == nil {
				prestashopContext = parsedContext
				s = strings.TrimSpace(s[:lastBrace])
			}
		}
	}

	return s, prestashopContext, extra
}

// prestashopLevelToLogLevel converts Symfony/Monolog log level to models.LogLevel.
func prestashopLevelToLogLevel(level string) models.LogLevel {
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
func (p *PrestaShopParser) Name() string {
	return "prestashop"
}

// Type returns the log type this parser handles.
func (p *PrestaShopParser) Type() models.LogType {
	return models.LogTypePrestaShop
}

// CanParse returns true if the line looks like a PrestaShop log.
func (p *PrestaShopParser) CanParse(line string) bool {
	return p.regex.MatchString(line)
}

// IsStartOfEntry returns true if the line is the start of a new log entry.
// This is used for multiline parsing (e.g., stack traces).
func (p *PrestaShopParser) IsStartOfEntry(line string) bool {
	return p.startRegex.MatchString(line)
}

// ParseMultiLine parses multiple lines as a single log entry.
// This handles stack traces and other multiline content in PrestaShop logs.
func (p *PrestaShopParser) ParseMultiLine(lines []string) (*models.LogEntry, error) {
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
