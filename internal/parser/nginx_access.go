// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// NginxAccessParser parses Nginx access logs.
// Supports both combined and common log formats.
type NginxAccessParser struct {
	*BaseParser
	combinedRegex *regexp.Regexp
	commonRegex   *regexp.Regexp
}

// Nginx access log timestamp format
const nginxAccessTimeFormat = "02/Jan/2006:15:04:05 -0700"

// NewNginxAccessParser creates a new Nginx access log parser.
func NewNginxAccessParser(opts *ParserOptions) *NginxAccessParser {
	return &NginxAccessParser{
		BaseParser: NewBaseParser(opts),
		// Combined format: $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
		combinedRegex: regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+) "([^"]*)" "([^"]*)"$`),
		// Common format: $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent
		commonRegex: regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+)$`),
	}
}

// Parse parses a single Nginx access log line.
func (p *NginxAccessParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single Nginx access log line with context support.
func (p *NginxAccessParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeNginx

	// Try combined format first (most common)
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
func (p *NginxAccessParser) parseCombined(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	// Parse common fields first (indices 1-8 = remote_addr, remote_user, time_local, method, uri, protocol, status, bytes)
	if err := p.parseCommonFields(entry, matches[1:9]); err != nil {
		return nil, err
	}

	// Additional fields from combined format
	referer := matches[9]
	if referer != "-" {
		entry.SetField("http_referer", referer)
	}

	userAgent := matches[10]
	if userAgent != "-" {
		entry.SetField("http_user_agent", userAgent)
	}

	// Build message
	entry.Message = buildAccessMessage(entry)

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseCommon parses the common log format.
func (p *NginxAccessParser) parseCommon(entry *models.LogEntry, line string, matches []string) (*models.LogEntry, error) {
	if err := p.parseCommonFields(entry, matches[1:9]); err != nil {
		return nil, err
	}

	// Build message
	entry.Message = buildAccessMessage(entry)

	p.ApplyOptions(entry, line)
	return entry, nil
}

// parseCommonFields parses fields common to both formats.
// Fields: remote_addr, remote_user, time_local, method, request_uri, protocol, status, body_bytes_sent
func (p *NginxAccessParser) parseCommonFields(entry *models.LogEntry, fields []string) error {
	// Remote address
	entry.SetField("remote_addr", fields[0])

	// Remote user (may be "-")
	remoteUser := fields[1]
	if remoteUser != "-" {
		entry.SetField("remote_user", remoteUser)
	}

	// Timestamp
	timestamp, err := time.Parse(nginxAccessTimeFormat, fields[2])
	if err != nil {
		return ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Request parts
	entry.SetField("method", fields[3])
	entry.SetField("request_uri", fields[4])
	entry.SetField("protocol", fields[5])

	// Status code
	status, err := strconv.Atoi(fields[6])
	if err != nil {
		return ErrInvalidFormat
	}
	entry.SetField("status", status)

	// Set log level based on status code
	entry.Level = statusToLevel(status)

	// Body bytes sent
	bodyBytes, err := strconv.Atoi(fields[7])
	if err != nil {
		return ErrInvalidFormat
	}
	entry.SetField("body_bytes_sent", bodyBytes)

	return nil
}

// statusToLevel converts HTTP status code to log level.
func statusToLevel(status int) models.LogLevel {
	switch {
	case status >= 500:
		return models.LevelError
	case status >= 400:
		return models.LevelWarning
	default:
		return models.LevelInfo
	}
}

// buildAccessMessage builds a human-readable message from access log fields.
func buildAccessMessage(entry *models.LogEntry) string {
	method := entry.GetFieldString("method")
	uri := entry.GetFieldString("request_uri")
	status := entry.GetFieldInt("status")
	return method + " " + uri + " " + strconv.Itoa(status)
}

// Name returns the parser name.
func (p *NginxAccessParser) Name() string {
	return "nginx-access"
}

// Type returns the log type this parser handles.
func (p *NginxAccessParser) Type() models.LogType {
	return models.LogTypeNginx
}

// CanParse returns true if the line looks like a Nginx access log.
func (p *NginxAccessParser) CanParse(line string) bool {
	return p.combinedRegex.MatchString(line) || p.commonRegex.MatchString(line)
}
