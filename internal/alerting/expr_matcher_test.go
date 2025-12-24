package alerting

import (
	"testing"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestExprMatcher_Compile(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "simple equality",
			expression: `level == "error"`,
			wantErr:    false,
		},
		{
			name:       "numeric comparison",
			expression: `http_status >= 500`,
			wantErr:    false,
		},
		{
			name:       "boolean AND",
			expression: `level == "error" && http_status >= 500`,
			wantErr:    false,
		},
		{
			name:       "boolean OR",
			expression: `level == "error" || level == "fatal"`,
			wantErr:    false,
		},
		{
			name:       "contains operator",
			expression: `message contains "timeout"`,
			wantErr:    false,
		},
		{
			name:       "invalid syntax",
			expression: `level == `,
			wantErr:    true,
		},
		{
			name:       "undefined variable",
			expression: `unknown_field == "test"`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExprMatcher(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExprMatcher() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExprMatcher_Match(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *models.LogEntry
		want       bool
	}{
		{
			name:       "level equals error",
			expression: `level == "error"`,
			entry: &models.LogEntry{
				Level: models.LevelError,
			},
			want: true,
		},
		{
			name:       "level equals error - no match",
			expression: `level == "error"`,
			entry: &models.LogEntry{
				Level: models.LevelInfo,
			},
			want: false,
		},
		{
			name:       "http_status >= 500",
			expression: `http_status >= 500`,
			entry: &models.LogEntry{
				Fields: map[string]any{"status": 503},
			},
			want: true,
		},
		{
			name:       "http_status >= 500 - no match",
			expression: `http_status >= 500`,
			entry: &models.LogEntry{
				Fields: map[string]any{"status": 200},
			},
			want: false,
		},
		{
			name:       "AND condition - both match",
			expression: `level == "error" && http_status >= 500`,
			entry: &models.LogEntry{
				Level:  models.LevelError,
				Fields: map[string]any{"status": 500},
			},
			want: true,
		},
		{
			name:       "AND condition - one fails",
			expression: `level == "error" && http_status >= 500`,
			entry: &models.LogEntry{
				Level:  models.LevelInfo,
				Fields: map[string]any{"status": 500},
			},
			want: false,
		},
		{
			name:       "OR condition - first matches",
			expression: `level == "error" || level == "fatal"`,
			entry: &models.LogEntry{
				Level: models.LevelError,
			},
			want: true,
		},
		{
			name:       "OR condition - second matches",
			expression: `level == "error" || level == "fatal"`,
			entry: &models.LogEntry{
				Level: models.LevelFatal,
			},
			want: true,
		},
		{
			name:       "OR condition - none match",
			expression: `level == "error" || level == "fatal"`,
			entry: &models.LogEntry{
				Level: models.LevelInfo,
			},
			want: false,
		},
		{
			name:       "contains operator",
			expression: `message contains "timeout"`,
			entry: &models.LogEntry{
				Message: "Connection timeout occurred",
			},
			want: true,
		},
		{
			name:       "contains operator - no match",
			expression: `message contains "error"`,
			entry: &models.LogEntry{
				Message: "Connection timeout occurred",
			},
			want: false,
		},
		{
			name:       "startsWith operator",
			expression: `uri startsWith "/api/"`,
			entry: &models.LogEntry{
				Fields: map[string]any{"uri": "/api/users"},
			},
			want: true,
		},
		{
			name:       "in array",
			expression: `level in ["error", "fatal"]`,
			entry: &models.LogEntry{
				Level: models.LevelFatal,
			},
			want: true,
		},
		{
			name:       "field access",
			expression: `fields["duration"] > 5`,
			entry: &models.LogEntry{
				Fields: map[string]any{"duration": 10.5},
			},
			want: true,
		},
		{
			name:       "labels access",
			expression: `labels["env"] == "production"`,
			entry: &models.LogEntry{
				Labels: map[string]string{"env": "production"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := NewExprMatcher(tt.expression)
			if err != nil {
				t.Fatalf("NewExprMatcher() error = %v", err)
			}

			got, err := matcher.Match(tt.entry)
			if err != nil {
				t.Fatalf("Match() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExprRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid expr rule",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Expression: `level == "error"`,
					Aggregation: &AggregationConfig{
						Function:  "count",
						Threshold: 10,
						Window:    "5m",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing expression",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Aggregation: &AggregationConfig{
						Function:  "count",
						Threshold: 10,
						Window:    "5m",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing aggregation",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Expression: `level == "error"`,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid aggregation function",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Expression: `level == "error"`,
					Aggregation: &AggregationConfig{
						Function:  "sum",
						Threshold: 10,
						Window:    "5m",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "rate function",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Expression: `level == "error"`,
					Aggregation: &AggregationConfig{
						Function:  "rate",
						Threshold: 5,
						Window:    "1m",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid window",
			rule: &Rule{
				Name: "test-rule",
				Type: RuleTypeExpr,
				Condition: Condition{
					Expression: `level == "error"`,
					Aggregation: &AggregationConfig{
						Function:  "count",
						Threshold: 10,
						Window:    "invalid",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
