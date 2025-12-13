package parser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// MockParser is a test parser that parses simple "LEVEL: message" format.
type MockParser struct {
	*BaseParser
}

func NewMockParser(opts *ParserOptions) *MockParser {
	return &MockParser{
		BaseParser: NewBaseParser(opts),
	}
}

func (p *MockParser) Parse(line string) (*models.LogEntry, error) {
	return p.ParseWithContext(context.Background(), line)
}

func (p *MockParser) ParseWithContext(ctx context.Context, line string) (*models.LogEntry, error) {
	if line == "" {
		return nil, ErrEmptyLine
	}

	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidFormat
	}

	entry := models.NewLogEntry()
	entry.Level = models.ParseLogLevel(parts[0])
	entry.Message = parts[1]
	entry.Timestamp = time.Now()
	entry.Type = models.LogTypeUnknown

	p.ApplyOptions(entry, line)
	return entry, nil
}

func (p *MockParser) Name() string {
	return "mock"
}

func (p *MockParser) Type() models.LogType {
	return models.LogTypeUnknown
}

func (p *MockParser) CanParse(line string) bool {
	parts := strings.SplitN(line, ": ", 2)
	return len(parts) == 2
}

func TestMockParser_Parse(t *testing.T) {
	parser := NewMockParser(nil)

	tests := []struct {
		line          string
		expectError   bool
		expectedLevel models.LogLevel
		expectedMsg   string
	}{
		{"INFO: test message", false, models.LevelInfo, "test message"},
		{"ERROR: something went wrong", false, models.LevelError, "something went wrong"},
		{"DEBUG: debugging info", false, models.LevelDebug, "debugging info"},
		{"invalid line", true, "", ""},
		{"", true, "", ""},
	}

	for _, tt := range tests {
		entry, err := parser.Parse(tt.line)

		if tt.expectError {
			if err == nil {
				t.Errorf("Parse(%q): expected error, got nil", tt.line)
			}
			continue
		}

		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tt.line, err)
			continue
		}

		if entry.Level != tt.expectedLevel {
			t.Errorf("Parse(%q): expected level %v, got %v", tt.line, tt.expectedLevel, entry.Level)
		}

		if entry.Message != tt.expectedMsg {
			t.Errorf("Parse(%q): expected message %q, got %q", tt.line, tt.expectedMsg, entry.Message)
		}
	}
}

func TestMockParser_CanParse(t *testing.T) {
	parser := NewMockParser(nil)

	tests := []struct {
		line     string
		expected bool
	}{
		{"INFO: test message", true},
		{"ERROR: something", true},
		{"invalid line", false},
		{"", false},
	}

	for _, tt := range tests {
		got := parser.CanParse(tt.line)
		if got != tt.expected {
			t.Errorf("CanParse(%q): expected %v, got %v", tt.line, tt.expected, got)
		}
	}
}

func TestBaseParser_ApplyOptions(t *testing.T) {
	opts := &ParserOptions{
		IncludeRaw: true,
		Source:     "test-source",
		Labels: map[string]string{
			"env": "test",
		},
	}

	parser := NewMockParser(opts)
	entry, _ := parser.Parse("INFO: test")

	if entry.Raw != "INFO: test" {
		t.Errorf("Expected raw line 'INFO: test', got '%s'", entry.Raw)
	}

	if entry.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got '%s'", entry.Source)
	}

	if entry.GetLabel("env") != "test" {
		t.Errorf("Expected label env='test', got '%s'", entry.GetLabel("env"))
	}
}

func TestDefaultParserOptions(t *testing.T) {
	opts := DefaultParserOptions()

	if opts == nil {
		t.Fatal("DefaultParserOptions returned nil")
	}

	if !opts.IncludeRaw {
		t.Error("Expected IncludeRaw to be true by default")
	}

	if opts.Labels == nil {
		t.Error("Expected Labels map to be initialized")
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	parser := NewMockParser(nil)

	// Test Register
	registry.Register(parser)

	// Test Get
	got, ok := registry.Get(models.LogTypeUnknown)
	if !ok {
		t.Error("Get should return true for registered parser")
	}
	if got != parser {
		t.Error("Get should return the registered parser")
	}

	// Test Get for unregistered type
	_, ok = registry.Get(models.LogTypeNginx)
	if ok {
		t.Error("Get should return false for unregistered type")
	}

	// Test GetByName
	got, ok = registry.GetByName("mock")
	if !ok {
		t.Error("GetByName should return true for registered parser")
	}
	if got != parser {
		t.Error("GetByName should return the registered parser")
	}

	// Test GetByName for unregistered name
	_, ok = registry.GetByName("nginx")
	if ok {
		t.Error("GetByName should return false for unregistered name")
	}

	// Test AutoDetect
	got, ok = registry.AutoDetect("INFO: test message")
	if !ok {
		t.Error("AutoDetect should return true for matching parser")
	}
	if got != parser {
		t.Error("AutoDetect should return the matching parser")
	}

	// Test AutoDetect for non-matching line
	_, ok = registry.AutoDetect("invalid line format")
	if ok {
		t.Error("AutoDetect should return false for non-matching line")
	}

	// Test All
	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 parser, got %d", len(all))
	}

	// Test Types
	types := registry.Types()
	if len(types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(types))
	}
}

func TestParseStreamDefault(t *testing.T) {
	parser := NewMockParser(nil)
	input := "INFO: message 1\nERROR: message 2\ninvalid\nDEBUG: message 3"
	reader := strings.NewReader(input)

	entries := make(chan *models.LogEntry, 10)
	ctx := context.Background()

	err := ParseStreamDefault(ctx, parser, reader, entries)
	if err != nil {
		t.Fatalf("ParseStreamDefault error: %v", err)
	}

	// Collect entries
	var result []*models.LogEntry
	for entry := range entries {
		result = append(result, entry)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(result))
	}

	// Verify line numbers
	if result[0].LineNumber != 1 {
		t.Errorf("First entry should be line 1, got %d", result[0].LineNumber)
	}
	if result[1].LineNumber != 2 {
		t.Errorf("Second entry should be line 2, got %d", result[1].LineNumber)
	}
	if result[2].LineNumber != 4 {
		t.Errorf("Third entry should be line 4, got %d", result[2].LineNumber)
	}
}

func TestParseStreamDefault_CancelContext(t *testing.T) {
	parser := NewMockParser(nil)
	input := "INFO: message 1\nINFO: message 2\nINFO: message 3"
	reader := strings.NewReader(input)

	entries := make(chan *models.LogEntry, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := ParseStreamDefault(ctx, parser, reader, entries)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}
