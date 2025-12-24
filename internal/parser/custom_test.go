package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestNewCustomParser(t *testing.T) {
	tests := []struct {
		name    string
		config  CustomParserConfig
		wantErr bool
	}{
		{
			name: "valid regex parser",
			config: CustomParserConfig{
				Name:            "test-parser",
				Pattern:         `^(?P<timestamp>\d{4}-\d{2}-\d{2}) (?P<level>\w+) (?P<message>.*)$`,
				TimestampFormat: "2006-01-02",
			},
			wantErr: false,
		},
		{
			name: "valid json parser",
			config: CustomParserConfig{
				Name:     "json-parser",
				JSONMode: true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: CustomParserConfig{
				Pattern: `^.*$`,
			},
			wantErr: true,
		},
		{
			name: "missing pattern (not json)",
			config: CustomParserConfig{
				Name: "no-pattern",
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			config: CustomParserConfig{
				Name:    "bad-regex",
				Pattern: `(?P<invalid`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCustomParser(&tt.config, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCustomParser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCustomParser_ParseRegex(t *testing.T) {
	cfg := CustomParserConfig{
		Name:            "test-parser",
		Pattern:         `^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}) (?P<level>\w+) (?P<message>.*)$`,
		TimestampFormat: "2006-01-02T15:04:05",
		TimestampField:  "timestamp",
		LevelField:      "level",
		MessageField:    "message",
		Labels:          map[string]string{"app": "test"},
	}

	parser, err := NewCustomParser(&cfg, nil)
	if err != nil {
		t.Fatalf("NewCustomParser() error = %v", err)
	}

	tests := []struct {
		name      string
		line      string
		wantLevel models.LogLevel
		wantMsg   string
		wantErr   bool
	}{
		{
			name:      "valid log line",
			line:      "2024-01-15T10:30:00 ERROR Something went wrong",
			wantLevel: models.LevelError,
			wantMsg:   "Something went wrong",
			wantErr:   false,
		},
		{
			name:      "info level",
			line:      "2024-01-15T10:30:00 INFO Application started",
			wantLevel: models.LevelInfo,
			wantMsg:   "Application started",
			wantErr:   false,
		},
		{
			name:    "invalid format",
			line:    "not a valid log line",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("Message = %v, want %v", entry.Message, tt.wantMsg)
			}
			if entry.Labels["app"] != "test" {
				t.Errorf("Labels[app] = %v, want test", entry.Labels["app"])
			}
		})
	}
}

func TestCustomParser_ParseJSON(t *testing.T) {
	cfg := CustomParserConfig{
		Name:            "json-parser",
		JSONMode:        true,
		TimestampField:  "ts",
		TimestampFormat: time.RFC3339,
		LevelField:      "level",
		MessageField:    "msg",
	}

	parser, err := NewCustomParser(&cfg, nil)
	if err != nil {
		t.Fatalf("NewCustomParser() error = %v", err)
	}

	tests := []struct {
		name      string
		line      string
		wantLevel models.LogLevel
		wantMsg   string
		wantErr   bool
	}{
		{
			name:      "valid json",
			line:      `{"ts":"2024-01-15T10:30:00Z","level":"ERROR","msg":"Connection failed"}`,
			wantLevel: models.LevelError,
			wantMsg:   "Connection failed",
			wantErr:   false,
		},
		{
			name:      "info level",
			line:      `{"ts":"2024-01-15T10:30:00Z","level":"INFO","msg":"Server started"}`,
			wantLevel: models.LevelInfo,
			wantMsg:   "Server started",
			wantErr:   false,
		},
		{
			name:    "invalid json",
			line:    `{not valid json}`,
			wantErr: true,
		},
		{
			name:    "not json",
			line:    "plain text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("Message = %v, want %v", entry.Message, tt.wantMsg)
			}
		})
	}
}

func TestCustomParser_CanParse(t *testing.T) {
	regexParser, _ := NewCustomParser(&CustomParserConfig{
		Name:    "regex",
		Pattern: `^\d{4}-\d{2}-\d{2}`,
	}, nil)

	jsonParser, _ := NewCustomParser(&CustomParserConfig{
		Name:     "json",
		JSONMode: true,
	}, nil)

	tests := []struct {
		name   string
		parser *CustomParser
		line   string
		want   bool
	}{
		{
			name:   "regex matches",
			parser: regexParser,
			line:   "2024-01-15 INFO test",
			want:   true,
		},
		{
			name:   "regex no match",
			parser: regexParser,
			line:   "INFO test",
			want:   false,
		},
		{
			name:   "json valid",
			parser: jsonParser,
			line:   `{"msg":"test"}`,
			want:   true,
		},
		{
			name:   "json invalid",
			parser: jsonParser,
			line:   "not json",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.parser.CanParse(tt.line); got != tt.want {
				t.Errorf("CanParse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomParser_LevelMapping(t *testing.T) {
	cfg := CustomParserConfig{
		Name:    "level-test",
		Pattern: `^(?P<level>\w+): (?P<message>.*)$`,
		LevelField:   "level",
		MessageField: "message",
		LevelMapping: map[string]string{
			"SEVERE": "error",
			"FINE":   "debug",
		},
	}

	parser, _ := NewCustomParser(&cfg, nil)

	tests := []struct {
		line      string
		wantLevel models.LogLevel
	}{
		{"SEVERE: Database error", models.LevelError},
		{"FINE: Debug message", models.LevelDebug},
		{"ERROR: Standard error", models.LevelError},   // standard mapping
		{"UNKNOWN: Something", models.LevelInfo},       // default
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLevel)
			}
		})
	}
}

func TestCustomParser_Multiline(t *testing.T) {
	cfg := CustomParserConfig{
		Name:         "multiline",
		Pattern:      `^(?P<timestamp>\d{4}-\d{2}-\d{2}) (?P<level>\w+) (?P<message>.*)$`,
		StartPattern: `^\d{4}-\d{2}-\d{2}`,
		LevelField:   "level",
		MessageField: "message",
	}

	parser, _ := NewCustomParser(&cfg, nil)

	// Test IsStartOfEntry
	if !parser.IsStartOfEntry("2024-01-15 ERROR Main error") {
		t.Error("IsStartOfEntry should return true for entry start")
	}
	if parser.IsStartOfEntry("    at com.example.Class.method()") {
		t.Error("IsStartOfEntry should return false for continuation")
	}

	// Test ParseMultiLine
	lines := []string{
		"2024-01-15 ERROR Main error",
		"    at com.example.Class.method()",
		"    at com.example.Main.main()",
	}

	entry, err := parser.ParseMultiLine(lines)
	if err != nil {
		t.Fatalf("ParseMultiLine() error = %v", err)
	}

	if entry.Level != models.LevelError {
		t.Errorf("Level = %v, want error", entry.Level)
	}

	if entry.Fields["multiline"] != true {
		t.Error("multiline field should be true")
	}

	if entry.Fields["stack_frame_count"] != 2 {
		t.Errorf("stack_frame_count = %v, want 2", entry.Fields["stack_frame_count"])
	}
}

func TestRegisterCustomParsers(t *testing.T) {
	registry := NewRegistry()

	configs := []CustomParserConfig{
		{
			Name:    "custom1",
			Pattern: `^.*$`,
		},
		{
			Name:     "custom2",
			JSONMode: true,
		},
	}

	err := RegisterCustomParsers(registry, configs)
	if err != nil {
		t.Fatalf("RegisterCustomParsers() error = %v", err)
	}

	// Check parsers were registered
	if _, ok := registry.GetByName("custom1"); !ok {
		t.Error("custom1 should be registered")
	}
	if _, ok := registry.GetByName("custom2"); !ok {
		t.Error("custom2 should be registered")
	}
}

func TestRegisterCustomParsers_DuplicateName(t *testing.T) {
	registry := NewRegistry()

	configs := []CustomParserConfig{
		{Name: "dup", Pattern: `^.*$`},
		{Name: "dup", Pattern: `^.*$`},
	}

	err := RegisterCustomParsers(registry, configs)
	if err == nil {
		t.Error("expected error for duplicate names")
	}
}
