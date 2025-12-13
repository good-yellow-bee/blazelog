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

// NginxErrorParser parses Nginx error logs.
// Format: YYYY/MM/DD HH:MM:SS [level] PID#TID: *CID message
type NginxErrorParser struct {
	*BaseParser
	regex       *regexp.Regexp
	clientRegex *regexp.Regexp
	serverRegex *regexp.Regexp
}

// Nginx error log timestamp format
const nginxErrorTimeFormat = "2006/01/02 15:04:05"

// NewNginxErrorParser creates a new Nginx error log parser.
func NewNginxErrorParser(opts *ParserOptions) *NginxErrorParser {
	return &NginxErrorParser{
		BaseParser: NewBaseParser(opts),
		// Main pattern: timestamp [level] pid#tid: *cid? message
		regex:       regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] (\d+)#(\d+): (?:\*(\d+) )?(.+)$`),
		clientRegex: regexp.MustCompile(`client: ([^,]+)`),
		serverRegex: regexp.MustCompile(`server: ([^,]+)`),
	}
}

// Parse parses a single Nginx error log line.
func (p *NginxErrorParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single Nginx error log line with context support.
func (p *NginxErrorParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	matches := p.regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrInvalidFormat
	}

	entry := models.NewLogEntry()
	entry.Type = models.LogTypeNginx

	// Parse timestamp
	timestamp, err := time.Parse(nginxErrorTimeFormat, matches[1])
	if err != nil {
		return nil, ErrInvalidFormat
	}
	entry.Timestamp = timestamp

	// Parse level
	level := strings.ToLower(matches[2])
	entry.Level = nginxLevelToLogLevel(level)
	entry.SetField("nginx_level", level)

	// Parse PID
	pid, _ := strconv.Atoi(matches[3])
	entry.SetField("pid", pid)

	// Parse TID
	tid, _ := strconv.Atoi(matches[4])
	entry.SetField("tid", tid)

	// Parse connection ID (optional)
	if matches[5] != "" {
		cid, _ := strconv.Atoi(matches[5])
		entry.SetField("cid", cid)
	}

	// Message
	message := matches[6]
	entry.Message = message

	// Extract client from message if present
	if clientMatch := p.clientRegex.FindStringSubmatch(message); clientMatch != nil {
		entry.SetField("client", clientMatch[1])
	}

	// Extract server from message if present
	if serverMatch := p.serverRegex.FindStringSubmatch(message); serverMatch != nil {
		entry.SetField("server", serverMatch[1])
	}

	p.ApplyOptions(entry, line)
	return entry, nil
}

// nginxLevelToLogLevel converts Nginx log level to models.LogLevel.
func nginxLevelToLogLevel(level string) models.LogLevel {
	switch level {
	case "debug":
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
func (p *NginxErrorParser) Name() string {
	return "nginx-error"
}

// Type returns the log type this parser handles.
func (p *NginxErrorParser) Type() models.LogType {
	return models.LogTypeNginx
}

// CanParse returns true if the line looks like a Nginx error log.
func (p *NginxErrorParser) CanParse(line string) bool {
	return p.regex.MatchString(line)
}
