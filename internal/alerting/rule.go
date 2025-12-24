// Package alerting provides alert rules engine for BlazeLog.
// It supports pattern-based (regex) and threshold-based alerting with
// sliding window aggregation and cooldown/deduplication.
package alerting

import (
	"fmt"
	"regexp"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// RuleType defines the type of alert rule.
type RuleType string

const (
	// RuleTypePattern triggers on regex pattern match.
	RuleTypePattern RuleType = "pattern"
	// RuleTypeThreshold triggers when count exceeds threshold in window.
	RuleTypeThreshold RuleType = "threshold"
	// RuleTypeExpr triggers based on expr-lang expression with aggregation.
	RuleTypeExpr RuleType = "expr"
)

// Severity represents the severity level of an alert.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// ParseSeverity converts a string to Severity.
func ParseSeverity(s string) Severity {
	switch s {
	case "low", "LOW":
		return SeverityLow
	case "medium", "MEDIUM":
		return SeverityMedium
	case "high", "HIGH":
		return SeverityHigh
	case "critical", "CRITICAL":
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

// AggregationConfig defines aggregation for expr-based rules.
type AggregationConfig struct {
	// Function is the aggregation function: "count" or "rate".
	Function string `yaml:"function"`
	// Threshold is the value that triggers the alert.
	Threshold float64 `yaml:"threshold"`
	// Operator is the comparison operator (default: ">=").
	Operator string `yaml:"operator,omitempty"`
	// Window is the time window for aggregation (e.g., "5m", "1h").
	Window string `yaml:"window"`

	// windowDuration is the parsed window duration (internal use).
	windowDuration time.Duration
}

// Condition defines the alert trigger condition.
type Condition struct {
	// Pattern is the regex pattern for pattern-based rules.
	Pattern string `yaml:"pattern,omitempty"`
	// CaseSensitive controls whether pattern matching is case-sensitive.
	CaseSensitive bool `yaml:"case_sensitive,omitempty"`
	// Field is the log field to check (e.g., "level", "status", "message").
	Field string `yaml:"field,omitempty"`
	// Value is the value to match against for threshold rules.
	Value interface{} `yaml:"value,omitempty"`
	// Operator is the comparison operator (e.g., ">=", "<=", "==", "!=", ">", "<").
	Operator string `yaml:"operator,omitempty"`
	// Threshold is the count that triggers the alert.
	Threshold int `yaml:"threshold,omitempty"`
	// Window is the time window for threshold counting (e.g., "5m", "1h").
	Window string `yaml:"window,omitempty"`
	// LogType filters by log type (e.g., "nginx", "magento").
	LogType string `yaml:"log_type,omitempty"`

	// Expression is the expr-lang filter expression for expr-based rules.
	Expression string `yaml:"expression,omitempty"`
	// Aggregation defines how matching entries are aggregated for expr rules.
	Aggregation *AggregationConfig `yaml:"aggregation,omitempty"`

	// compiledPattern is the compiled regex (internal use).
	compiledPattern *regexp.Regexp
	// compiledExpr is the compiled expr matcher (internal use).
	compiledExpr *ExprMatcher
	// windowDuration is the parsed window duration (internal use).
	windowDuration time.Duration
}

// Rule represents a single alert rule.
type Rule struct {
	// Name is the unique identifier for the rule.
	Name string `yaml:"name"`
	// Description provides details about what the rule detects.
	Description string `yaml:"description,omitempty"`
	// Type is either "pattern" or "threshold".
	Type RuleType `yaml:"type"`
	// Condition defines when the rule triggers.
	Condition Condition `yaml:"condition"`
	// Severity indicates the importance of the alert.
	Severity Severity `yaml:"severity"`
	// Notify lists the notification channels to use.
	Notify []string `yaml:"notify,omitempty"`
	// Cooldown is the minimum time between repeated alerts.
	Cooldown string `yaml:"cooldown,omitempty"`
	// Labels filter which logs this rule applies to.
	Labels map[string]string `yaml:"labels,omitempty"`
	// Enabled controls whether the rule is active.
	Enabled *bool `yaml:"enabled,omitempty"`

	// cooldownDuration is the parsed cooldown duration (internal use).
	cooldownDuration time.Duration
}

// IsEnabled returns whether the rule is enabled.
func (r *Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// Validate validates and compiles the rule configuration.
func (r *Rule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	if r.Type == "" {
		return fmt.Errorf("rule type is required for rule %q", r.Name)
	}

	if r.Type != RuleTypePattern && r.Type != RuleTypeThreshold && r.Type != RuleTypeExpr {
		return fmt.Errorf("invalid rule type %q for rule %q", r.Type, r.Name)
	}

	// Validate pattern rules
	if r.Type == RuleTypePattern {
		if r.Condition.Pattern == "" {
			return fmt.Errorf("pattern is required for pattern rule %q", r.Name)
		}
		// Compile pattern
		flags := ""
		if !r.Condition.CaseSensitive {
			flags = "(?i)"
		}
		compiled, err := regexp.Compile(flags + r.Condition.Pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %q for rule %q: %w", r.Condition.Pattern, r.Name, err)
		}
		r.Condition.compiledPattern = compiled
	}

	// Validate threshold rules
	if r.Type == RuleTypeThreshold {
		if r.Condition.Threshold <= 0 {
			return fmt.Errorf("threshold must be positive for rule %q", r.Name)
		}
		if r.Condition.Window == "" {
			return fmt.Errorf("window is required for threshold rule %q", r.Name)
		}
		windowDur, err := time.ParseDuration(r.Condition.Window)
		if err != nil {
			return fmt.Errorf("invalid window %q for rule %q: %w", r.Condition.Window, r.Name, err)
		}
		r.Condition.windowDuration = windowDur

		// Default operator to "=="
		if r.Condition.Operator == "" {
			r.Condition.Operator = "=="
		}
		// Validate operator
		switch r.Condition.Operator {
		case "==", "!=", ">", ">=", "<", "<=":
			// Valid
		default:
			return fmt.Errorf("invalid operator %q for rule %q", r.Condition.Operator, r.Name)
		}
	}

	// Validate expr rules
	if r.Type == RuleTypeExpr {
		if r.Condition.Expression == "" {
			return fmt.Errorf("expression is required for expr rule %q", r.Name)
		}

		// Compile expression
		matcher, err := NewExprMatcher(r.Condition.Expression)
		if err != nil {
			return fmt.Errorf("invalid expression for rule %q: %w", r.Name, err)
		}
		r.Condition.compiledExpr = matcher

		// Validate aggregation config
		if r.Condition.Aggregation == nil {
			return fmt.Errorf("aggregation is required for expr rule %q", r.Name)
		}
		agg := r.Condition.Aggregation

		// Validate function
		switch agg.Function {
		case "count", "rate":
			// Valid
		default:
			return fmt.Errorf("invalid aggregation function %q for rule %q (must be count or rate)", agg.Function, r.Name)
		}

		// Validate threshold
		if agg.Threshold <= 0 {
			return fmt.Errorf("aggregation threshold must be positive for rule %q", r.Name)
		}

		// Default operator
		if agg.Operator == "" {
			agg.Operator = ">="
		}
		switch agg.Operator {
		case "==", "!=", ">", ">=", "<", "<=":
			// Valid
		default:
			return fmt.Errorf("invalid aggregation operator %q for rule %q", agg.Operator, r.Name)
		}

		// Validate window
		if agg.Window == "" {
			return fmt.Errorf("aggregation window is required for expr rule %q", r.Name)
		}
		windowDur, err := time.ParseDuration(agg.Window)
		if err != nil {
			return fmt.Errorf("invalid aggregation window %q for rule %q: %w", agg.Window, r.Name, err)
		}
		agg.windowDuration = windowDur
	}

	// Parse cooldown
	if r.Cooldown != "" {
		cooldownDur, err := time.ParseDuration(r.Cooldown)
		if err != nil {
			return fmt.Errorf("invalid cooldown %q for rule %q: %w", r.Cooldown, r.Name, err)
		}
		r.cooldownDuration = cooldownDur
	}

	// Default severity
	if r.Severity == "" {
		r.Severity = SeverityMedium
	}

	return nil
}

// GetCompiledPattern returns the compiled regex pattern.
func (r *Rule) GetCompiledPattern() *regexp.Regexp {
	return r.Condition.compiledPattern
}

// GetWindowDuration returns the parsed window duration.
func (r *Rule) GetWindowDuration() time.Duration {
	return r.Condition.windowDuration
}

// GetCooldownDuration returns the parsed cooldown duration.
func (r *Rule) GetCooldownDuration() time.Duration {
	return r.cooldownDuration
}

// GetExprMatcher returns the compiled expression matcher.
func (r *Rule) GetExprMatcher() *ExprMatcher {
	return r.Condition.compiledExpr
}

// GetAggregationWindowDuration returns the aggregation window duration.
func (r *Rule) GetAggregationWindowDuration() time.Duration {
	if r.Condition.Aggregation == nil {
		return 0
	}
	return r.Condition.Aggregation.windowDuration
}

// MatchesLabels checks if the rule's label filter matches the log entry.
func (r *Rule) MatchesLabels(entry *models.LogEntry) bool {
	if len(r.Labels) == 0 {
		return true
	}
	for key, value := range r.Labels {
		if value == "*" {
			continue
		}
		entryValue := entry.GetLabel(key)
		if entryValue != value {
			return false
		}
	}
	return true
}

// MatchesLogType checks if the rule's log type filter matches.
func (r *Rule) MatchesLogType(entry *models.LogEntry) bool {
	if r.Condition.LogType == "" || r.Condition.LogType == "*" {
		return true
	}
	return string(entry.Type) == r.Condition.LogType
}

// Alert represents a triggered alert.
type Alert struct {
	// RuleName is the name of the rule that triggered.
	RuleName string `json:"rule_name"`
	// Description is the rule description.
	Description string `json:"description,omitempty"`
	// Severity is the alert severity.
	Severity Severity `json:"severity"`
	// Message provides details about what triggered the alert.
	Message string `json:"message"`
	// Timestamp is when the alert was triggered.
	Timestamp time.Time `json:"timestamp"`
	// Count is the number of matching events (for threshold alerts).
	Count int `json:"count,omitempty"`
	// Threshold is the configured threshold (for threshold alerts).
	Threshold int `json:"threshold,omitempty"`
	// Window is the configured window (for threshold alerts).
	Window string `json:"window,omitempty"`
	// TriggeringEntry is the log entry that triggered the alert (for pattern alerts).
	TriggeringEntry *models.LogEntry `json:"triggering_entry,omitempty"`
	// Notify is the list of notification channels.
	Notify []string `json:"notify,omitempty"`
	// Labels from the rule.
	Labels map[string]string `json:"labels,omitempty"`
}

// RulesConfig represents the top-level YAML configuration.
type RulesConfig struct {
	Rules []*Rule `yaml:"rules"`
}
