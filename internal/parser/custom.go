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

// CustomParserConfig defines a custom log parser via YAML configuration.
type CustomParserConfig struct {
	// Name is the unique identifier for this parser.
	Name string `yaml:"name"`
	// Pattern is the regex pattern with named capture groups.
	Pattern string `yaml:"pattern,omitempty"`
	// JSONMode parses logs as JSON instead of regex.
	JSONMode bool `yaml:"json_mode,omitempty"`
	// StartPattern identifies the start of a new log entry (for multiline).
	StartPattern string `yaml:"start_pattern,omitempty"`
	// TimestampField is the name of the field/group containing the timestamp.
	TimestampField string `yaml:"timestamp_field,omitempty"`
	// TimestampFormat is the Go time format for parsing timestamps.
	TimestampFormat string `yaml:"timestamp_format,omitempty"`
	// LevelField is the name of the field/group containing the log level.
	LevelField string `yaml:"level_field,omitempty"`
	// MessageField is the name of the field/group containing the message.
	MessageField string `yaml:"message_field,omitempty"`
	// LevelMapping maps parsed level values to standard levels.
	LevelMapping map[string]string `yaml:"level_mapping,omitempty"`
	// DefaultLevel is used when level cannot be determined.
	DefaultLevel string `yaml:"default_level,omitempty"`
	// Labels are static labels added to all parsed entries.
	Labels map[string]string `yaml:"labels,omitempty"`
}

// CustomParser implements the Parser interface for user-defined log formats.
type CustomParser struct {
	*BaseParser
	config     *CustomParserConfig
	regex      *regexp.Regexp
	startRegex *regexp.Regexp
	groupNames map[string]int
}

// NewCustomParser creates a new custom parser from configuration.
func NewCustomParser(cfg *CustomParserConfig, opts *ParserOptions) (*CustomParser, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("parser name is required")
	}

	if !cfg.JSONMode && cfg.Pattern == "" {
		return nil, fmt.Errorf("pattern is required for regex-based parser %q", cfg.Name)
	}

	p := &CustomParser{
		BaseParser: NewBaseParser(opts),
		config:     cfg,
		groupNames: make(map[string]int),
	}

	// Compile main pattern
	if cfg.Pattern != "" {
		regex, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern for parser %q: %w", cfg.Name, err)
		}
		p.regex = regex

		// Build group name index
		for i, name := range regex.SubexpNames() {
			if name != "" {
				p.groupNames[name] = i
			}
		}
	}

	// Compile start pattern for multiline
	if cfg.StartPattern != "" {
		startRegex, err := regexp.Compile(cfg.StartPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid start_pattern for parser %q: %w", cfg.Name, err)
		}
		p.startRegex = startRegex
	}

	// Set defaults
	if cfg.TimestampField == "" {
		cfg.TimestampField = "timestamp"
	}
	if cfg.TimestampFormat == "" {
		cfg.TimestampFormat = time.RFC3339
	}
	if cfg.DefaultLevel == "" {
		cfg.DefaultLevel = "info"
	}

	return p, nil
}

// Name returns the parser name.
func (p *CustomParser) Name() string {
	return p.config.Name
}

// Type returns the log type.
func (p *CustomParser) Type() models.LogType {
	return models.LogTypeCustom
}

// CanParse checks if the line matches this parser's pattern.
func (p *CustomParser) CanParse(line string) bool {
	if p.config.JSONMode {
		line = strings.TrimSpace(line)
		return len(line) > 0 && line[0] == '{'
	}
	if p.regex == nil {
		return false
	}
	return p.regex.MatchString(line)
}

// Parse parses a single log line.
func (p *CustomParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

// ParseWithContext parses a single log line with context.
func (p *CustomParser) ParseWithContext(_ context.Context, line string) (*models.LogEntry, error) {
	if p.config.JSONMode {
		return p.parseJSON(line)
	}
	return p.parseRegex(line)
}

// parseRegex parses a log line using the configured regex pattern.
func (p *CustomParser) parseRegex(line string) (*models.LogEntry, error) {
	if p.regex == nil {
		return nil, ErrInvalidFormat
	}

	matches := p.regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrInvalidFormat
	}

	entry := &models.LogEntry{
		Timestamp: time.Now(),
		Level:     models.LogLevel(p.config.DefaultLevel),
		Type:      models.LogTypeCustom,
		Fields:    make(map[string]interface{}),
		Labels:    make(map[string]string),
	}

	// Extract named groups into fields
	for name, idx := range p.groupNames {
		if idx < len(matches) && matches[idx] != "" {
			entry.Fields[name] = matches[idx]
		}
	}

	// Extract timestamp
	if tsField := p.config.TimestampField; tsField != "" {
		if idx, ok := p.groupNames[tsField]; ok && idx < len(matches) {
			if ts, err := time.Parse(p.config.TimestampFormat, matches[idx]); err == nil {
				entry.Timestamp = ts
			}
		}
	}

	// Extract level
	if levelField := p.config.LevelField; levelField != "" {
		if idx, ok := p.groupNames[levelField]; ok && idx < len(matches) {
			entry.Level = p.mapLevel(matches[idx])
		}
	}

	// Extract message
	if msgField := p.config.MessageField; msgField != "" {
		if idx, ok := p.groupNames[msgField]; ok && idx < len(matches) {
			entry.Message = matches[idx]
		}
	}

	// Apply static labels
	for k, v := range p.config.Labels {
		entry.Labels[k] = v
	}

	// Apply base parser options
	p.ApplyOptions(entry, line)

	return entry, nil
}

// parseJSON parses a JSON log line.
func (p *CustomParser) parseJSON(line string) (*models.LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] != '{' {
		return nil, ErrInvalidFormat
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}

	entry := &models.LogEntry{
		Timestamp: time.Now(),
		Level:     models.LogLevel(p.config.DefaultLevel),
		Type:      models.LogTypeCustom,
		Fields:    data,
		Labels:    make(map[string]string),
	}

	// Extract timestamp
	if tsField := p.config.TimestampField; tsField != "" {
		if ts, ok := data[tsField]; ok {
			switch v := ts.(type) {
			case string:
				if parsed, err := time.Parse(p.config.TimestampFormat, v); err == nil {
					entry.Timestamp = parsed
				}
			case float64:
				// Unix timestamp (seconds)
				entry.Timestamp = time.Unix(int64(v), 0)
			}
		}
	}

	// Extract level
	if levelField := p.config.LevelField; levelField != "" {
		if level, ok := data[levelField].(string); ok {
			entry.Level = p.mapLevel(level)
		}
	}

	// Extract message
	if msgField := p.config.MessageField; msgField != "" {
		if msg, ok := data[msgField].(string); ok {
			entry.Message = msg
		}
	}

	// Apply static labels
	for k, v := range p.config.Labels {
		entry.Labels[k] = v
	}

	// Apply base parser options
	p.ApplyOptions(entry, line)

	return entry, nil
}

// mapLevel maps a parsed level string to a standard LogLevel.
func (p *CustomParser) mapLevel(level string) models.LogLevel {
	level = strings.ToUpper(strings.TrimSpace(level))

	// Check custom mapping first
	if p.config.LevelMapping != nil {
		if mapped, ok := p.config.LevelMapping[level]; ok {
			return models.LogLevel(strings.ToLower(mapped))
		}
	}

	// Standard mappings
	switch level {
	case "DEBUG", "TRACE":
		return models.LevelDebug
	case "INFO", "NOTICE":
		return models.LevelInfo
	case "WARN", "WARNING":
		return models.LevelWarning
	case "ERROR", "ERR":
		return models.LevelError
	case "FATAL", "CRITICAL", "CRIT", "EMERGENCY", "EMERG", "ALERT":
		return models.LevelFatal
	default:
		return models.LogLevel(p.config.DefaultLevel)
	}
}

// IsStartOfEntry checks if a line starts a new log entry (for multiline parsing).
func (p *CustomParser) IsStartOfEntry(line string) bool {
	if p.startRegex != nil {
		return p.startRegex.MatchString(line)
	}
	// For JSON mode, each line starting with { is a new entry
	if p.config.JSONMode {
		return strings.HasPrefix(strings.TrimSpace(line), "{")
	}
	// For regex mode, if pattern matches it's a start
	if p.regex != nil {
		return p.regex.MatchString(line)
	}
	return true
}

// ParseMultiLine parses multiple lines as a single log entry.
func (p *CustomParser) ParseMultiLine(lines []string) (*models.LogEntry, error) {
	if len(lines) == 0 {
		return nil, ErrEmptyLine
	}

	// Parse the first line
	entry, err := p.Parse(lines[0])
	if err != nil {
		return nil, err
	}

	// Append additional lines to message as stack trace
	if len(lines) > 1 {
		var stackTrace strings.Builder
		for i := 1; i < len(lines); i++ {
			if i > 1 {
				stackTrace.WriteString("\n")
			}
			stackTrace.WriteString(lines[i])
		}

		entry.Fields["stack_trace"] = stackTrace.String()
		entry.Fields["stack_frame_count"] = len(lines) - 1
		entry.Fields["multiline"] = true

		// Append to message if it exists
		if entry.Message != "" {
			entry.Message = entry.Message + "\n" + stackTrace.String()
		}
	}

	return entry, nil
}
