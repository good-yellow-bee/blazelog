// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// ApacheErrorParser parses Apache error logs.
// Supports Apache 2.4+ format and Apache 2.2 format.
type ApacheErrorParser struct {
	*BaseParser
	// Apache 2.4+ format: [timestamp] [module:level] [pid pid:tid tid] [client ip:port] message
	regex24     *regexp.Regexp
	// Apache 2.2 format: [timestamp] [level] [client ip] message
	regex22     *regexp.Regexp
	// Simplified 2.2 format without client: [timestamp] [level] message
	regex22NoClient *regexp.Regexp
}

// Apache 2.4+ error log timestamp format: Sat Oct 10 14:32:52.123456 2020
const apache24ErrorTimeFormat = "Mon Jan 02 15:04:05.000000 2006"

// Apache 2.2 error log timestamp format: Sat Oct 10 14:32:52 2020
const apache22ErrorTimeFormat = "Mon Jan 02 15:04:05 2006"

// NewApacheErrorParser creates a new Apache error log parser.
func NewApacheErrorParser(opts *Options) *ApacheErrorParser {
	return &ApacheErrorParser{
		BaseParser: NewBaseParser(opts),
		// Apache 2.4+ format with full details
		// Example: [Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 123456789] [client 192.168.1.1:56789] AH00124: Request exceeded the limit
		regex24: regexp.MustCompile(`^\[([^\]]+)\] \[([^:\]]+):([^\]]+)\] \[pid (\d+):tid (\d+)\](?: \[client ([^\]]+)\])? (.+)$`),
		// Apache 2.2 format with client
		// Example: [Sat Oct 10 14:32:52 2020] [error] [client 192.168.1.1] File does not exist: /var/www/html/missing.html
		regex22: regexp.MustCompile(`^\[([^\]]+)\] \[(\w+)\] \[client ([^\]]+)\] (.+)$`),
		// Apache 2.2 format without client
		// Example: [Sat Oct 10 14:32:52 2020] [notice] Apache/2.2.22 configured
		regex22NoClient: regexp.MustCompile(`^\[([^\]]+)\] \[(\w+)\] (.+)$`),
	}
}

// Parse parses a single Apache error log line.
func (p *ApacheErrorParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single Apache error log line with context support.
func (p *ApacheErrorParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeApache

	// Try Apache 2.4+ format first (most common)
	if matches := p.regex24.FindStringSubmatch(line); matches != nil {
		return p.parseApache24(entry, line, matches)
	}

	// Try Apache 2.2 format with client
	if matches := p.regex22.FindStringSubmatch(line); matches != nil {
		return p.parseApache22WithClient(entry, line, matches)
	}

	// Try Apache 2.2 format without client
	if matches := p.regex22NoClient.FindStringSubmatch(line); matches != nil {
		return p.parseApache22NoClient(entry, line, matches)
	}

	return nil, ErrInvalidFormat
}

// parseApache24 parses Apache 2.4+ error log format.
func (p *ApacheErrorParser) parseApache24(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	// Parse timestamp
	timestamp, err := parseApacheErrorTimestamp(matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Module
	module := matches[2]
	entry.SetField("module", module)

	// Level
	level := strings.ToLower(matches[3])
	entry.Level = apacheLevelToLogLevel(level)
	entry.SetField("apache_level", level)

	// PID
	pid, _ := strconv.Atoi(matches[4])
	entry.SetField("pid", pid)

	// TID
	tid, _ := strconv.Atoi(matches[5])
	entry.SetField("tid", tid)

	// Client (optional)
	if matches[6] != "" {
		client := matches[6]
		entry.SetField("client", client)
		// Extract just the IP if it includes port
		if idx := strings.LastIndex(client, ":"); idx != -1 {
			entry.SetField("client_ip", client[:idx])
			if port, err := strconv.Atoi(client[idx+1:]); err == nil {
				entry.SetField("client_port", port)
			}
		} else {
			entry.SetField("client_ip", client)
		}
	}

	// Message
	entry.Message = matches[7]

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseApache22WithClient parses Apache 2.2 error log format with client.
func (p *ApacheErrorParser) parseApache22WithClient(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	// Parse timestamp
	timestamp, err := parseApacheErrorTimestamp(matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Level
	level := strings.ToLower(matches[2])
	entry.Level = apacheLevelToLogLevel(level)
	entry.SetField("apache_level", level)

	// Client
	client := matches[3]
	entry.SetField("client", client)
	entry.SetField("client_ip", client)

	// Message
	entry.Message = matches[4]

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseApache22NoClient parses Apache 2.2 error log format without client.
func (p *ApacheErrorParser) parseApache22NoClient(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	// Parse timestamp
	timestamp, err := parseApacheErrorTimestamp(matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Level
	level := strings.ToLower(matches[2])
	entry.Level = apacheLevelToLogLevel(level)
	entry.SetField("apache_level", level)

	// Message
	entry.Message = matches[3]

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseApacheErrorTimestamp parses Apache error log timestamp in various formats.
func parseApacheErrorTimestamp(s string) (time.Time, error) {
	// Try Apache 2.4+ format first (with microseconds)
	if t, err := time.Parse(apache24ErrorTimeFormat, s); err == nil {
		return t, nil
	}
	// Try Apache 2.2 format (without microseconds)
	if t, err := time.Parse(apache22ErrorTimeFormat, s); err == nil {
		return t, nil
	}
	// Try common variations
	formats := []string{
		"Mon Jan 2 15:04:05.000000 2006",
		"Mon Jan 2 15:04:05 2006",
		"Mon Jan _2 15:04:05.000000 2006",
		"Mon Jan _2 15:04:05 2006",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrInvalidFormat
}

// apacheLevelToLogLevel converts Apache log level to models.LogLevel.
func apacheLevelToLogLevel(level string) models.LogLevel {
	switch level {
	case "trace1", "trace2", "trace3", "trace4", "trace5", "trace6", "trace7", "trace8", "debug":
		return models.LevelDebug
	case "info", "notice":
		return models.LevelInfo
	case "warn":
		return models.LevelWarning
	case "error":
		return models.LevelError
	case "crit", "alert", "emerg":
		return models.LevelFatal
	default:
		return models.LevelUnknown
	}
}

// Name returns the parser name.
func (p *ApacheErrorParser) Name() string {
	return "apache-error"
}

// Type returns the log type this parser handles.
func (p *ApacheErrorParser) Type() models.LogType {
	return models.LogTypeApache
}

// CanParse returns true if the line looks like an Apache error log.
func (p *ApacheErrorParser) CanParse(line string) bool {
	return p.regex24.MatchString(line) || p.regex22.MatchString(line) || p.regex22NoClient.MatchString(line)
}
