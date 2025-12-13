// Package parser provides log parsing functionality for various log formats.
package parser

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Common errors returned by parsers.
var (
	ErrInvalidFormat = errors.New("invalid log format")
	ErrEmptyLine     = errors.New("empty line")
	ErrParseFailed   = errors.New("failed to parse log line")
)

// Parser is the interface that all log parsers must implement.
type Parser interface {
	// Parse parses a single log line and returns a LogEntry.
	// Returns ErrInvalidFormat if the line doesn't match the expected format.
	Parse(line string) (*models.LogEntry, error)

	// ParseWithContext parses a single log line with context support.
	ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error)

	// Name returns the name of the parser (e.g., "nginx", "apache").
	Name() string

	// Type returns the LogType this parser handles.
	Type() models.LogType

	// CanParse returns true if the parser can likely parse the given line.
	// This is used for auto-detection.
	CanParse(line string) bool
}

// MultiLineParser is an interface for parsers that handle multi-line log entries.
// Examples: stack traces, exception logs.
type MultiLineParser interface {
	Parser

	// IsStartOfEntry returns true if the line is the start of a new log entry.
	IsStartOfEntry(line string) bool

	// ParseMultiLine parses multiple lines as a single log entry.
	ParseMultiLine(lines []string) (*models.LogEntry, error)
}

// StreamParser is an interface for parsers that can stream-parse from a reader.
type StreamParser interface {
	Parser

	// ParseStream reads from the reader and sends parsed entries to the channel.
	// The channel is closed when the reader is exhausted or an error occurs.
	ParseStream(ctx context.Context, r io.Reader, entries chan<- *models.LogEntry) error
}

// ParserOptions contains configuration options for parsers.
type ParserOptions struct {
	// TimeFormat specifies the expected timestamp format.
	TimeFormat string

	// TimeZone specifies the timezone for parsing timestamps.
	TimeZone string

	// IncludeRaw includes the original raw line in the LogEntry.
	IncludeRaw bool

	// Labels are default labels to add to all parsed entries.
	Labels map[string]string

	// Source is the source identifier for all parsed entries.
	Source string
}

// DefaultParserOptions returns default parser options.
func DefaultParserOptions() *ParserOptions {
	return &ParserOptions{
		IncludeRaw: true,
		Labels:     make(map[string]string),
	}
}

// BaseParser provides common functionality for parsers.
type BaseParser struct {
	options *ParserOptions
}

// NewBaseParser creates a new BaseParser with the given options.
func NewBaseParser(opts *ParserOptions) *BaseParser {
	if opts == nil {
		opts = DefaultParserOptions()
	}
	return &BaseParser{options: opts}
}

// Options returns the parser options.
func (p *BaseParser) Options() *ParserOptions {
	return p.options
}

// ApplyOptions applies parser options to a log entry.
func (p *BaseParser) ApplyOptions(entry *models.LogEntry, raw string) {
	if p.options == nil {
		return
	}

	if p.options.IncludeRaw {
		entry.Raw = raw
	}

	if p.options.Source != "" {
		entry.Source = p.options.Source
	}

	for k, v := range p.options.Labels {
		entry.SetLabel(k, v)
	}
}

// ParseStreamDefault provides a default implementation of stream parsing.
func ParseStreamDefault(ctx context.Context, p Parser, r io.Reader, entries chan<- *models.LogEntry) error {
	defer close(entries)

	scanner := bufio.NewScanner(r)
	lineNum := int64(0)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lineNum++
		line := scanner.Text()

		if line == "" {
			continue
		}

		entry, err := p.ParseWithContext(ctx, line)
		if err != nil {
			if errors.Is(err, ErrInvalidFormat) || errors.Is(err, ErrEmptyLine) {
				continue
			}
			// Log error but continue processing
			continue
		}

		entry.LineNumber = lineNum

		select {
		case entries <- entry:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return scanner.Err()
}

// Registry holds all registered parsers.
type Registry struct {
	parsers map[models.LogType]Parser
}

// NewRegistry creates a new parser registry.
func NewRegistry() *Registry {
	return &Registry{
		parsers: make(map[models.LogType]Parser),
	}
}

// Register adds a parser to the registry.
func (r *Registry) Register(p Parser) {
	r.parsers[p.Type()] = p
}

// Get returns a parser by type.
func (r *Registry) Get(t models.LogType) (Parser, bool) {
	p, ok := r.parsers[t]
	return p, ok
}

// GetByName returns a parser by name.
func (r *Registry) GetByName(name string) (Parser, bool) {
	for _, p := range r.parsers {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

// AutoDetect tries to detect the appropriate parser for the given line.
func (r *Registry) AutoDetect(line string) (Parser, bool) {
	for _, p := range r.parsers {
		if p.CanParse(line) {
			return p, true
		}
	}
	return nil, false
}

// All returns all registered parsers.
func (r *Registry) All() []Parser {
	result := make([]Parser, 0, len(r.parsers))
	for _, p := range r.parsers {
		result = append(result, p)
	}
	return result
}

// Types returns all registered log types.
func (r *Registry) Types() []models.LogType {
	result := make([]models.LogType, 0, len(r.parsers))
	for t := range r.parsers {
		result = append(result, t)
	}
	return result
}

// DefaultRegistry is the global default parser registry.
var DefaultRegistry = NewRegistry()

// Register adds a parser to the default registry.
func Register(p Parser) {
	DefaultRegistry.Register(p)
}

// Get returns a parser from the default registry.
func Get(t models.LogType) (Parser, bool) {
	return DefaultRegistry.Get(t)
}

// AutoDetect tries to detect a parser from the default registry.
func AutoDetect(line string) (Parser, bool) {
	return DefaultRegistry.AutoDetect(line)
}
