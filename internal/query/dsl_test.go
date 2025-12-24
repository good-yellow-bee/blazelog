package query

import (
	"testing"
)

func TestQueryDSL_Parse(t *testing.T) {
	dsl := NewQueryDSL(DefaultFields)

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		// Valid expressions
		{"simple equality", `level == "error"`, false},
		{"in operator", `level in ["error", "fatal"]`, false},
		{"and logic", `level == "error" and http_status >= 500`, false},
		{"or logic", `level == "error" or level == "fatal"`, false},
		{"not logic", `not (message contains "debug")`, false},
		{"contains", `message contains "timeout"`, false},
		{"startsWith", `uri startsWith "/api/"`, false},
		{"endsWith", `file_path endsWith ".log"`, false},
		{"numeric comparison", `http_status >= 500`, false},
		{"numeric less than", `http_status < 400`, false},
		{"complex boolean", `level == "error" and (http_status >= 500 or message contains "fatal")`, false},
		{"time function", `timestamp > now() - duration("1h")`, false},

		// Invalid expressions
		{"empty expression", ``, true},
		{"unknown field", `foo == "bar"`, true},
		{"syntax error", `level ==`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dsl.Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryDSL_ParseWithJSONFields(t *testing.T) {
	dsl := NewQueryDSL(DefaultFields)

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"json field access", `fields.status == "200"`, false},
		{"labels access", `labels.env == "production"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dsl.Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFieldDef_IsOperatorAllowed(t *testing.T) {
	field := FieldDef{
		Operators: []string{"==", "!=", "in"},
	}

	tests := []struct {
		op   string
		want bool
	}{
		{"==", true},
		{"!=", true},
		{"in", true},
		{">=", false},
		{"contains", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			if got := field.IsOperatorAllowed(tt.op); got != tt.want {
				t.Errorf("IsOperatorAllowed(%q) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}
