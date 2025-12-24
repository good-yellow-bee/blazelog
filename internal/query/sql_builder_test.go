package query

import (
	"reflect"
	"testing"
)

func TestSQLBuilder_Build(t *testing.T) {
	dsl := NewQueryDSL(DefaultFields)
	builder := NewSQLBuilder(DefaultFields)

	tests := []struct {
		name          string
		expr          string
		wantSQL       string
		wantArgs      []any
		skipArgsCheck bool // For map-based args where order is non-deterministic
	}{
		{
			name:     "simple equality",
			expr:     `level == "error"`,
			wantSQL:  "(lower(level) = ?)",
			wantArgs: []any{"error"},
		},
		{
			name:          "in operator",
			expr:          `level in ["error", "fatal"]`,
			wantSQL:       "level IN (?, ?)",
			skipArgsCheck: true, // Map iteration order is non-deterministic
		},
		{
			name:     "numeric comparison",
			expr:     `http_status >= 500`,
			wantSQL:  "(http_status >= ?)",
			wantArgs: []any{500},
		},
		{
			name:     "contains",
			expr:     `message contains "timeout"`,
			wantSQL:  "position(lower(message), ?) > 0",
			wantArgs: []any{"timeout"},
		},
		{
			name:     "startsWith",
			expr:     `uri startsWith "/api/"`,
			wantSQL:  "startsWith(lower(uri), ?)",
			wantArgs: []any{"/api/"},
		},
		{
			name:     "and logic",
			expr:     `level == "error" and http_status >= 500`,
			wantSQL:  "((lower(level) = ?) AND (http_status >= ?))",
			wantArgs: []any{"error", 500},
		},
		{
			name:     "or logic",
			expr:     `level == "error" or level == "fatal"`,
			wantSQL:  "((lower(level) = ?) OR (lower(level) = ?))",
			wantArgs: []any{"error", "fatal"},
		},
		{
			name:     "not operator",
			expr:     `not (level == "debug")`,
			wantSQL:  "NOT ((lower(level) = ?))",
			wantArgs: []any{"debug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := dsl.Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			result, err := builder.Build(parsed)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if result.SQL != tt.wantSQL {
				t.Errorf("SQL = %q, want %q", result.SQL, tt.wantSQL)
			}

			if !tt.skipArgsCheck && !reflect.DeepEqual(result.Args, tt.wantArgs) {
				t.Errorf("Args = %v, want %v", result.Args, tt.wantArgs)
			}
		})
	}
}

func TestSQLBuilder_JSONFields(t *testing.T) {
	dsl := NewQueryDSL(DefaultFields)
	builder := NewSQLBuilder(DefaultFields)

	tests := []struct {
		name    string
		expr    string
		wantSQL string
	}{
		{
			name:    "json field access",
			expr:    `fields.status == "200"`,
			wantSQL: "(JSONExtractString(fields, 'status') = ?)",
		},
		{
			name:    "labels access",
			expr:    `labels.env == "production"`,
			wantSQL: "(JSONExtractString(labels, 'env') = ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := dsl.Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			result, err := builder.Build(parsed)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if result.SQL != tt.wantSQL {
				t.Errorf("SQL = %q, want %q", result.SQL, tt.wantSQL)
			}
		})
	}
}

func TestSQLBuilder_TimeFunctions(t *testing.T) {
	dsl := NewQueryDSL(DefaultFields)
	builder := NewSQLBuilder(DefaultFields)

	tests := []struct {
		name    string
		expr    string
		wantSQL string
	}{
		{
			name:    "now function",
			expr:    `timestamp > now()`,
			wantSQL: "(timestamp > now())",
		},
		{
			name:    "duration 1 hour",
			expr:    `timestamp > now() - duration("1h")`,
			wantSQL: "(timestamp > (now() - INTERVAL 1 HOUR))",
		},
		{
			name:    "duration 30 minutes",
			expr:    `timestamp >= now() - duration("30m")`,
			wantSQL: "(timestamp >= (now() - INTERVAL 30 MINUTE))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := dsl.Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			result, err := builder.Build(parsed)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if result.SQL != tt.wantSQL {
				t.Errorf("SQL = %q, want %q", result.SQL, tt.wantSQL)
			}
		})
	}
}
