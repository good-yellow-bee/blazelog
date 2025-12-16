// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// WordPressParser parses WordPress debug.log and PHP error logs.
// WordPress debug.log format: [DD-Mon-YYYY HH:MM:SS TZ] PHP Level: message
// Examples:
//
//	[15-Jan-2024 10:23:45 UTC] PHP Notice:  Undefined variable: foo
//	[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Exception: ...
//	[15-Jan-2024 10:23:45 UTC] WordPress database error ...
type WordPressParser struct {
	*BaseParser
	// Main regex for parsing log lines
	// Groups: 1=timestamp, 2=timezone, 3=message_type (PHP level or WordPress), 4=message
	regex *regexp.Regexp
	// Regex to detect the start of a new log entry
	startRegex *regexp.Regexp
	// Regex to extract PHP file location: "in /path/file.php on line 123"
	inLineRegex *regexp.Regexp
	// Regex to extract PHP file location: "in /path/file.php:123"
	colonRegex *regexp.Regexp
}

// WordPress timestamp format: 15-Jan-2024 10:23:45
const wordpressTimeFormat = "02-Jan-2006 15:04:05"

// NewWordPressParser creates a new WordPress log parser.
func NewWordPressParser(opts *ParserOptions) *WordPressParser {
	return &WordPressParser{
		BaseParser: NewBaseParser(opts),
		// Main pattern: [timestamp timezone] PHP Level: message or [timestamp timezone] WordPress ...
		// Groups: 1=timestamp, 2=timezone, 3=rest of line
		regex: regexp.MustCompile(`^\[(\d{2}-[A-Za-z]{3}-\d{4} \d{2}:\d{2}:\d{2}) ([A-Z]{2,4})\] (.*)$`),
		// Pattern to detect start of a new entry
		startRegex: regexp.MustCompile(`^\[\d{2}-[A-Za-z]{3}-\d{4} \d{2}:\d{2}:\d{2}`),
		// Pattern: in /path/to/file.php on line 123
		inLineRegex: regexp.MustCompile(` in ([^\s]+\.php) on line (\d+)`),
		// Pattern: in /path/to/file.php:123
		colonRegex: regexp.MustCompile(` in ([^\s]+\.php):(\d+)`),
	}
}

// Parse parses a single WordPress log line.
func (p *WordPressParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single WordPress log line with context support.
func (p *WordPressParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	matches := p.regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrInvalidFormat
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeWordPress

	// Parse timestamp
	timestamp, err := time.Parse(wordpressTimeFormat, matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Store timezone as field
	timezone := matches[2]
	entry.SetField("timezone", timezone)

	// Parse message part
	message := matches[3]

	// Extract PHP level if present
	if strings.HasPrefix(message, "PHP ") {
		entry.SetField("source_type", "php")
		phpMessage := message[4:] // Remove "PHP " prefix

		// Parse PHP error level and message
		level, cleanMessage := parseWordPressPHPLevel(phpMessage)
		entry.Level = level
		entry.Message = cleanMessage

		// Extract file and line from PHP errors
		p.extractPHPLocation(entry, cleanMessage)
	} else if strings.HasPrefix(message, "WordPress database error") {
		entry.SetField("source_type", "wordpress_database")
		entry.Level = models.LevelError
		entry.Message = message
	} else if strings.HasPrefix(message, "WordPress ") {
		entry.SetField("source_type", "wordpress")
		entry.Level = models.LevelInfo
		entry.Message = message
	} else {
		// Unknown format, keep as-is
		entry.SetField("source_type", "unknown")
		entry.Level = models.LevelUnknown
		entry.Message = message
	}

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseWordPressPHPLevel extracts the log level from a PHP error message.
// PHP error levels: Notice, Warning, Fatal error, Parse error, Deprecated, Strict Standards
func parseWordPressPHPLevel(message string) (models.LogLevel, string) {
	// Check for various PHP error levels
	levels := []struct {
		prefix string
		level  models.LogLevel
	}{
		{"Fatal error:", models.LevelFatal},
		{"Parse error:", models.LevelFatal},
		{"Catchable fatal error:", models.LevelFatal},
		{"Error:", models.LevelError},
		{"Warning:", models.LevelWarning},
		{"Notice:", models.LevelInfo},
		{"Strict Standards:", models.LevelInfo},
		{"Deprecated:", models.LevelWarning},
	}

	for _, l := range levels {
		if strings.HasPrefix(message, l.prefix) {
			cleanMessage := strings.TrimSpace(message[len(l.prefix):])
			return l.level, cleanMessage
		}
	}

	// If no level found, return the message as-is with unknown level
	return models.LevelUnknown, message
}

// extractPHPLocation extracts file path and line number from PHP error messages.
// Common format: "... in /path/to/file.php on line 123"
// Or: "... in /path/to/file.php:123"
func (p *WordPressParser) extractPHPLocation(entry *models.LogEntry, message string) {
	// Pattern: in /path/to/file.php on line 123
	if matches := p.inLineRegex.FindStringSubmatch(message); matches != nil {
		entry.SetField("php_file", matches[1])
		entry.SetField("php_line", matches[2])
		return
	}

	// Pattern: in /path/to/file.php:123
	if matches := p.colonRegex.FindStringSubmatch(message); matches != nil {
		entry.SetField("php_file", matches[1])
		entry.SetField("php_line", matches[2])
	}
}

// Name returns the parser name.
func (p *WordPressParser) Name() string {
	return "wordpress"
}

// Type returns the log type this parser handles.
func (p *WordPressParser) Type() models.LogType {
	return models.LogTypeWordPress
}

// CanParse returns true if the line looks like a WordPress debug.log line.
func (p *WordPressParser) CanParse(line string) bool {
	if !p.regex.MatchString(line) {
		return false
	}

	// Additional check: should contain "PHP " or "WordPress" after the timestamp
	matches := p.regex.FindStringSubmatch(line)
	if len(matches) < 4 {
		return false
	}
	message := matches[3]
	return strings.HasPrefix(message, "PHP ") || strings.HasPrefix(message, "WordPress")
}

// IsStartOfEntry returns true if the line is the start of a new log entry.
// This is used for multiline parsing (e.g., stack traces).
func (p *WordPressParser) IsStartOfEntry(line string) bool {
	return p.startRegex.MatchString(line)
}

// ParseMultiLine parses multiple lines as a single log entry.
// This handles stack traces and other multiline content in WordPress logs.
func (p *WordPressParser) ParseMultiLine(lines []string) (*models.LogEntry, error) {
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

		// Count stack frames (lines starting with "#" after trimming)
		frameCount := 0
		for _, line := range stackTraceLines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
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
