// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// ApacheAccessParser parses Apache access logs.
// Supports both combined and common log formats.
type ApacheAccessParser struct {
	*BaseParser
	combinedRegex *regexp.Regexp
	commonRegex   *regexp.Regexp
}

// Apache access log timestamp format (same as Nginx)
const apacheAccessTimeFormat = "02/Jan/2006:15:04:05 -0700"

// NewApacheAccessParser creates a new Apache access log parser.
func NewApacheAccessParser(opts *ParserOptions) *ApacheAccessParser {
	return &ApacheAccessParser{
		BaseParser: NewBaseParser(opts),
		// Combined format: %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"
		// Example: 127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326 "http://www.example.com/" "Mozilla/5.0"
		combinedRegex: regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"$`),
		// Common format: %h %l %u %t "%r" %>s %b
		// Example: 127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326
		commonRegex: regexp.MustCompile(`^(\S+) (\S+) (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+|-)$`),
	}
}

// Parse parses a single Apache access log line.
func (p *ApacheAccessParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single Apache access log line with context support.
func (p *ApacheAccessParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeApache

	// Try combined format first (most common in production)
	if matches := p.combinedRegex.FindStringSubmatch(line); matches != nil {
		return p.parseCombined(entry, line, matches)
	}

	// Try common format
	if matches := p.commonRegex.FindStringSubmatch(line); matches != nil {
		return p.parseCommon(entry, line, matches)
	}

	return nil, ErrInvalidFormat
}

// parseCombined parses the combined log format.
func (p *ApacheAccessParser) parseCombined(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	// Parse common fields first (indices 1-9)
	if err := p.parseCommonFields(entry, matches[1:10]); err != nil {
		return nil, err
	}

	// Additional fields from combined format
	referer := matches[10]
	if referer != "-" && referer != "" {
		entry.SetField("http_referer", referer)
	}

	userAgent := matches[11]
	if userAgent != "-" && userAgent != "" {
		entry.SetField("http_user_agent", userAgent)
	}

	// Build message
	entry.Message = buildApacheAccessMessage(entry)

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseCommon parses the common log format.
func (p *ApacheAccessParser) parseCommon(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	if err := p.parseCommonFields(entry, matches[1:10]); err != nil {
		return nil, err
	}

	// Build message
	entry.Message = buildApacheAccessMessage(entry)

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseCommonFields parses fields common to both formats.
// Fields: remote_host, ident, remote_user, time_local, method, request_uri, protocol, status, bytes_sent
func (p *ApacheAccessParser) parseCommonFields(entry *models.LogEntry, fields []string) error {
	// Remote host (IP address)
	entry.SetField("remote_host", fields[0])

	// Ident (usually "-")
	ident := fields[1]
	if ident != "-" {
		entry.SetField("ident", ident)
	}

	// Remote user (may be "-")
	remoteUser := fields[2]
	if remoteUser != "-" {
		entry.SetField("remote_user", remoteUser)
	}

	// Timestamp
	timestamp, err := time.Parse(apacheAccessTimeFormat, fields[3])
	if err != nil {
		return ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Request parts
	entry.SetField("method", fields[4])
	entry.SetField("request_uri", fields[5])
	entry.SetField("protocol", fields[6])

	// Status code
	status, err := strconv.Atoi(fields[7])
	if err != nil {
		return ErrInvalidFormat
	}
	entry.SetField("status", status)

	// Set log level based on status code
	entry.Level = apacheStatusToLevel(status)

	// Bytes sent (may be "-" for no content)
	bytesSent := fields[8]
	if bytesSent != "-" {
		bytes, err := strconv.Atoi(bytesSent)
		if err == nil {
			entry.SetField("bytes_sent", bytes)
		}
	} else {
		entry.SetField("bytes_sent", 0)
	}

	return nil
}

// apacheStatusToLevel converts HTTP status code to log level.
func apacheStatusToLevel(status int) models.LogLevel {
	switch {
	case status >= 500:
		return models.LevelError
	case status >= 400:
		return models.LevelWarning
	default:
		return models.LevelInfo
	}
}

// buildApacheAccessMessage builds a human-readable message from access log fields.
func buildApacheAccessMessage(entry *models.LogEntry) string {
	method := entry.GetFieldString("method")
	uri := entry.GetFieldString("request_uri")
	status := entry.GetFieldInt("status")
	return method + " " + uri + " " + strconv.Itoa(status)
}

// Name returns the parser name.
func (p *ApacheAccessParser) Name() string {
	return "apache-access"
}

// Type returns the log type this parser handles.
func (p *ApacheAccessParser) Type() models.LogType {
	return models.LogTypeApache
}

// CanParse returns true if the line looks like an Apache access log.
func (p *ApacheAccessParser) CanParse(line string) bool {
	return p.combinedRegex.MatchString(line) || p.commonRegex.MatchString(line)
}
