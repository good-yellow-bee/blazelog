package alerting

import (
	"strings"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestRuleValidation(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty name",
			rule:    Rule{},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing type",
			rule: Rule{
				Name: "test-rule",
			},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "invalid type",
			rule: Rule{
				Name: "test-rule",
				Type: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid rule type",
		},
		{
			name: "pattern rule without pattern",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypePattern,
			},
			wantErr: true,
			errMsg:  "pattern is required",
		},
		{
			name: "pattern rule with invalid regex",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypePattern,
				Condition: Condition{
					Pattern: "[invalid(regex",
				},
			},
			wantErr: true,
			errMsg:  "invalid pattern",
		},
		{
			name: "threshold rule without threshold",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypeThreshold,
				Condition: Condition{
					Window: "5m",
				},
			},
			wantErr: true,
			errMsg:  "threshold must be positive",
		},
		{
			name: "threshold rule without window",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypeThreshold,
				Condition: Condition{
					Threshold: 10,
				},
			},
			wantErr: true,
			errMsg:  "window is required",
		},
		{
			name: "threshold rule with invalid window",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypeThreshold,
				Condition: Condition{
					Threshold: 10,
					Window:    "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid window",
		},
		{
			name: "threshold rule with invalid operator",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypeThreshold,
				Condition: Condition{
					Threshold: 10,
					Window:    "5m",
					Operator:  "~~~",
				},
			},
			wantErr: true,
			errMsg:  "invalid operator",
		},
		{
			name: "valid pattern rule",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypePattern,
				Condition: Condition{
					Pattern: "ERROR|FATAL",
				},
			},
			wantErr: false,
		},
		{
			name: "valid threshold rule",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypeThreshold,
				Condition: Condition{
					Field:     "level",
					Value:     "error",
					Threshold: 10,
					Window:    "5m",
				},
			},
			wantErr: false,
		},
		{
			name: "rule with invalid cooldown",
			rule: Rule{
				Name: "test-rule",
				Type: RuleTypePattern,
				Condition: Condition{
					Pattern: "ERROR",
				},
				Cooldown: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid cooldown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadRulesFromBytes(t *testing.T) {
	yaml := `
rules:
  - name: "High Error Rate"
    description: "More than 10 errors in 5 minutes"
    type: "threshold"
    condition:
      field: "level"
      value: "error"
      threshold: 10
      window: "5m"
    severity: "critical"
    notify:
      - "slack"
      - "email"
    cooldown: "15m"

  - name: "Fatal Error Detected"
    description: "FATAL error in logs"
    type: "pattern"
    condition:
      pattern: "FATAL|CRITICAL"
      case_sensitive: false
    severity: "critical"
    notify:
      - "slack"
`

	rules, err := LoadRulesFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to load rules: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	// Verify first rule (threshold)
	r1 := rules[0]
	if r1.Name != "High Error Rate" {
		t.Errorf("expected name 'High Error Rate', got %q", r1.Name)
	}
	if r1.Type != RuleTypeThreshold {
		t.Errorf("expected type 'threshold', got %q", r1.Type)
	}
	if r1.Condition.Threshold != 10 {
		t.Errorf("expected threshold 10, got %d", r1.Condition.Threshold)
	}
	if r1.GetWindowDuration() != 5*time.Minute {
		t.Errorf("expected window 5m, got %v", r1.GetWindowDuration())
	}
	if r1.GetCooldownDuration() != 15*time.Minute {
		t.Errorf("expected cooldown 15m, got %v", r1.GetCooldownDuration())
	}
	if r1.Severity != SeverityCritical {
		t.Errorf("expected severity 'critical', got %q", r1.Severity)
	}

	// Verify second rule (pattern)
	r2 := rules[1]
	if r2.Name != "Fatal Error Detected" {
		t.Errorf("expected name 'Fatal Error Detected', got %q", r2.Name)
	}
	if r2.Type != RuleTypePattern {
		t.Errorf("expected type 'pattern', got %q", r2.Type)
	}
	if r2.GetCompiledPattern() == nil {
		t.Error("expected compiled pattern, got nil")
	}
}

func TestMatcherPatternMatching(t *testing.T) {
	rule := &Rule{
		Name: "error-pattern",
		Type: RuleTypePattern,
		Condition: Condition{
			Pattern:       "ERROR|FATAL",
			CaseSensitive: false,
		},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	matcher := NewMatcher()

	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"matches ERROR", "An ERROR occurred in the system", true},
		{"matches error lowercase", "an error occurred", true},
		{"matches FATAL", "FATAL: system crash", true},
		{"no match", "Everything is fine", false},
		{"partial match", "This is erroneous", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := models.NewLogEntry()
			entry.Message = tt.message

			got := matcher.MatchPattern(rule, entry)
			if got != tt.want {
				t.Errorf("MatchPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatcherPatternWithLabels(t *testing.T) {
	rule := &Rule{
		Name: "error-pattern",
		Type: RuleTypePattern,
		Condition: Condition{
			Pattern: "ERROR",
		},
		Labels: map[string]string{
			"environment": "production",
		},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	matcher := NewMatcher()

	// Entry with matching label
	entry1 := models.NewLogEntry()
	entry1.Message = "ERROR occurred"
	entry1.SetLabel("environment", "production")

	if !matcher.MatchPattern(rule, entry1) {
		t.Error("expected match for entry with matching label")
	}

	// Entry with non-matching label
	entry2 := models.NewLogEntry()
	entry2.Message = "ERROR occurred"
	entry2.SetLabel("environment", "staging")

	if matcher.MatchPattern(rule, entry2) {
		t.Error("expected no match for entry with non-matching label")
	}

	// Entry without label
	entry3 := models.NewLogEntry()
	entry3.Message = "ERROR occurred"

	if matcher.MatchPattern(rule, entry3) {
		t.Error("expected no match for entry without label")
	}
}

func TestMatcherThresholdCondition(t *testing.T) {
	matcher := NewMatcher()

	// Test level field
	rule1 := &Rule{
		Name: "error-threshold",
		Type: RuleTypeThreshold,
		Condition: Condition{
			Field:     "level",
			Value:     "error",
			Operator:  "==",
			Threshold: 10,
			Window:    "5m",
		},
	}
	if err := rule1.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	entry1 := models.NewLogEntry()
	entry1.Level = models.LevelError

	if !matcher.MatchThresholdCondition(rule1, entry1) {
		t.Error("expected match for error level")
	}

	entry2 := models.NewLogEntry()
	entry2.Level = models.LevelInfo

	if matcher.MatchThresholdCondition(rule1, entry2) {
		t.Error("expected no match for info level")
	}

	// Test numeric field comparison
	rule2 := &Rule{
		Name: "status-threshold",
		Type: RuleTypeThreshold,
		Condition: Condition{
			Field:     "status",
			Value:     500,
			Operator:  ">=",
			Threshold: 10,
			Window:    "1m",
		},
	}
	if err := rule2.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	entry3 := models.NewLogEntry()
	entry3.SetField("status", 500)

	if !matcher.MatchThresholdCondition(rule2, entry3) {
		t.Error("expected match for status >= 500")
	}

	entry4 := models.NewLogEntry()
	entry4.SetField("status", 200)

	if matcher.MatchThresholdCondition(rule2, entry4) {
		t.Error("expected no match for status 200")
	}

	entry5 := models.NewLogEntry()
	entry5.SetField("status", 503)

	if !matcher.MatchThresholdCondition(rule2, entry5) {
		t.Error("expected match for status 503 >= 500")
	}
}

func TestSlidingWindow(t *testing.T) {
	window := NewSlidingWindow(5 * time.Second)
	baseTime := time.Now()

	// Add events
	window.AddAt(baseTime)
	window.AddAt(baseTime.Add(1 * time.Second))
	window.AddAt(baseTime.Add(2 * time.Second))

	// Count at 3 seconds - all 3 events should be within window
	count := window.CountAt(baseTime.Add(3 * time.Second))
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	// Count at 6 seconds - first event should be expired
	count = window.CountAt(baseTime.Add(6 * time.Second))
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// Count at 7 seconds - first two events expired (at 0s and 1s), one remains (at 2s)
	count = window.CountAt(baseTime.Add(7 * time.Second))
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// Count at 8 seconds - all events should be expired (cutoff = 3s, all events <= 2s)
	count = window.CountAt(baseTime.Add(8 * time.Second))
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestCooldownManager(t *testing.T) {
	cm := NewCooldownManager()
	baseTime := time.Now()
	ruleName := "test-rule"

	// Initially not on cooldown
	if cm.IsOnCooldown(ruleName, baseTime) {
		t.Error("expected not on cooldown initially")
	}

	// Set 10 second cooldown
	cm.SetCooldown(ruleName, 10*time.Second, baseTime)

	// Should be on cooldown at 5 seconds
	if !cm.IsOnCooldown(ruleName, baseTime.Add(5*time.Second)) {
		t.Error("expected on cooldown at 5 seconds")
	}

	// Should still be on cooldown at 9 seconds
	if !cm.IsOnCooldown(ruleName, baseTime.Add(9*time.Second)) {
		t.Error("expected on cooldown at 9 seconds")
	}

	// Should not be on cooldown at 11 seconds
	if cm.IsOnCooldown(ruleName, baseTime.Add(11*time.Second)) {
		t.Error("expected not on cooldown at 11 seconds")
	}

	// Test remaining cooldown
	remaining := cm.GetCooldownRemaining(ruleName, baseTime.Add(5*time.Second))
	if remaining < 4*time.Second || remaining > 6*time.Second {
		t.Errorf("expected ~5 seconds remaining, got %v", remaining)
	}
}

func TestEnginePatternAlert(t *testing.T) {
	rule := &Rule{
		Name:     "fatal-error",
		Type:     RuleTypePattern,
		Severity: SeverityCritical,
		Condition: Condition{
			Pattern: "FATAL",
		},
		Notify: []string{"slack"},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	// Non-matching entry
	entry1 := models.NewLogEntry()
	entry1.Message = "INFO: all systems operational"

	alerts := engine.Evaluate(entry1)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}

	// Matching entry
	entry2 := models.NewLogEntry()
	entry2.Message = "FATAL: system crash"

	alerts = engine.Evaluate(entry2)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.RuleName != "fatal-error" {
		t.Errorf("expected rule name 'fatal-error', got %q", alert.RuleName)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("expected severity 'critical', got %q", alert.Severity)
	}
	if alert.TriggeringEntry != entry2 {
		t.Error("expected triggering entry to be set")
	}
}

func TestEngineThresholdAlert(t *testing.T) {
	rule := &Rule{
		Name:     "error-rate",
		Type:     RuleTypeThreshold,
		Severity: SeverityHigh,
		Condition: Condition{
			Field:     "level",
			Value:     "error",
			Threshold: 3,
			Window:    "5m",
		},
		Notify: []string{"email"},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	baseTime := time.Now()

	// First error - no alert
	entry1 := models.NewLogEntry()
	entry1.Level = models.LevelError
	alerts := engine.EvaluateAt(entry1, baseTime)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after 1 error, got %d", len(alerts))
	}

	// Second error - no alert
	entry2 := models.NewLogEntry()
	entry2.Level = models.LevelError
	alerts = engine.EvaluateAt(entry2, baseTime.Add(1*time.Second))
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after 2 errors, got %d", len(alerts))
	}

	// Third error - should trigger alert (threshold reached)
	entry3 := models.NewLogEntry()
	entry3.Level = models.LevelError
	alerts = engine.EvaluateAt(entry3, baseTime.Add(2*time.Second))
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert after 3 errors, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Count != 3 {
		t.Errorf("expected count 3, got %d", alert.Count)
	}
	if alert.Threshold != 3 {
		t.Errorf("expected threshold 3, got %d", alert.Threshold)
	}
}

func TestEngineCooldown(t *testing.T) {
	rule := &Rule{
		Name:     "error-alert",
		Type:     RuleTypePattern,
		Severity: SeverityMedium,
		Condition: Condition{
			Pattern: "ERROR",
		},
		Cooldown: "10s",
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	baseTime := time.Now()

	// First error - should trigger
	entry1 := models.NewLogEntry()
	entry1.Message = "ERROR occurred"
	alerts := engine.EvaluateAt(entry1, baseTime)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	// Second error at 5s - should be suppressed (cooldown)
	entry2 := models.NewLogEntry()
	entry2.Message = "ERROR again"
	alerts = engine.EvaluateAt(entry2, baseTime.Add(5*time.Second))
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts during cooldown, got %d", len(alerts))
	}

	// Third error at 11s - should trigger (cooldown expired)
	entry3 := models.NewLogEntry()
	entry3.Message = "ERROR once more"
	alerts = engine.EvaluateAt(entry3, baseTime.Add(11*time.Second))
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert after cooldown, got %d", len(alerts))
	}
}

func TestEngineStats(t *testing.T) {
	rule := &Rule{
		Name:     "test-pattern",
		Type:     RuleTypePattern,
		Severity: SeverityLow,
		Condition: Condition{
			Pattern: "TEST",
		},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	// Evaluate some entries
	for i := 0; i < 5; i++ {
		entry := models.NewLogEntry()
		entry.Message = "TEST message"
		engine.Evaluate(entry)
	}

	for i := 0; i < 3; i++ {
		entry := models.NewLogEntry()
		entry.Message = "No match"
		engine.Evaluate(entry)
	}

	stats := engine.Stats()
	if stats.EntriesEvaluated != 8 {
		t.Errorf("expected 8 entries evaluated, got %d", stats.EntriesEvaluated)
	}
	if stats.PatternMatches != 5 {
		t.Errorf("expected 5 pattern matches, got %d", stats.PatternMatches)
	}
}

func TestEngineDisabledRule(t *testing.T) {
	enabled := false
	rule := &Rule{
		Name:    "disabled-rule",
		Type:    RuleTypePattern,
		Enabled: &enabled,
		Condition: Condition{
			Pattern: "ERROR",
		},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	entry := models.NewLogEntry()
	entry.Message = "ERROR occurred"

	alerts := engine.Evaluate(entry)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts from disabled rule, got %d", len(alerts))
	}
}

func TestEngineAddRemoveRule(t *testing.T) {
	engine := NewEngine(nil, nil)
	defer engine.Close()

	rule := &Rule{
		Name:     "dynamic-rule",
		Type:     RuleTypePattern,
		Severity: SeverityLow,
		Condition: Condition{
			Pattern: "DYNAMIC",
		},
	}

	// Add rule
	if err := engine.AddRule(rule); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	if len(engine.Rules()) != 1 {
		t.Errorf("expected 1 rule, got %d", len(engine.Rules()))
	}

	// Test that rule works
	entry := models.NewLogEntry()
	entry.Message = "DYNAMIC event"
	alerts := engine.Evaluate(entry)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	// Remove rule
	if !engine.RemoveRule("dynamic-rule") {
		t.Error("expected RemoveRule to return true")
	}

	if len(engine.Rules()) != 0 {
		t.Errorf("expected 0 rules, got %d", len(engine.Rules()))
	}

	// Test that rule no longer triggers
	alerts = engine.Evaluate(entry)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after rule removal, got %d", len(alerts))
	}
}

func TestEngineReloadRules(t *testing.T) {
	rule1 := &Rule{
		Name:     "rule1",
		Type:     RuleTypePattern,
		Severity: SeverityLow,
		Condition: Condition{
			Pattern: "FIRST",
		},
	}
	if err := rule1.Validate(); err != nil {
		t.Fatalf("rule1 validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule1}, nil)
	defer engine.Close()

	// New rules to reload
	rule2 := &Rule{
		Name:     "rule2",
		Type:     RuleTypePattern,
		Severity: SeverityHigh,
		Condition: Condition{
			Pattern: "SECOND",
		},
	}

	if err := engine.ReloadRules([]*Rule{rule2}); err != nil {
		t.Fatalf("failed to reload rules: %v", err)
	}

	if len(engine.Rules()) != 1 {
		t.Errorf("expected 1 rule after reload, got %d", len(engine.Rules()))
	}

	// Old rule should not trigger
	entry1 := models.NewLogEntry()
	entry1.Message = "FIRST event"
	alerts := engine.Evaluate(entry1)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts from old rule, got %d", len(alerts))
	}

	// New rule should trigger
	entry2 := models.NewLogEntry()
	entry2.Message = "SECOND event"
	alerts = engine.Evaluate(entry2)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert from new rule, got %d", len(alerts))
	}
}

func TestLogTypeFilter(t *testing.T) {
	rule := &Rule{
		Name:     "nginx-errors",
		Type:     RuleTypePattern,
		Severity: SeverityMedium,
		Condition: Condition{
			Pattern: "ERROR",
			LogType: "nginx",
		},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("rule validation failed: %v", err)
	}

	engine := NewEngine([]*Rule{rule}, nil)
	defer engine.Close()

	// Nginx error - should match
	entry1 := models.NewLogEntry()
	entry1.Message = "ERROR in request"
	entry1.Type = models.LogTypeNginx

	alerts := engine.Evaluate(entry1)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert for nginx error, got %d", len(alerts))
	}

	// Apache error - should not match (wrong log type)
	entry2 := models.NewLogEntry()
	entry2.Message = "ERROR in request"
	entry2.Type = models.LogTypeApache

	alerts = engine.Evaluate(entry2)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for apache error, got %d", len(alerts))
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  Severity
	}{
		{"low", SeverityLow},
		{"LOW", SeverityLow},
		{"medium", SeverityMedium},
		{"MEDIUM", SeverityMedium},
		{"high", SeverityHigh},
		{"HIGH", SeverityHigh},
		{"critical", SeverityCritical},
		{"CRITICAL", SeverityCritical},
		{"unknown", SeverityMedium}, // defaults to medium
		{"", SeverityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseSeverity(tt.input)
			if got != tt.want {
				t.Errorf("ParseSeverity(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWindowManager(t *testing.T) {
	wm := NewWindowManager()

	// Get or create a window
	w1 := wm.GetOrCreate("rule1", 5*time.Minute)
	if w1 == nil {
		t.Fatal("expected window to be created")
	}

	// Same call should return same window
	w2 := wm.GetOrCreate("rule1", 5*time.Minute)
	if w1 != w2 {
		t.Error("expected same window instance")
	}

	// Add events
	w1.Add()
	w1.Add()
	w1.Add()

	if wm.Count("rule1") != 3 {
		t.Errorf("expected count 3, got %d", wm.Count("rule1"))
	}

	// Reset specific window
	wm.Reset("rule1")
	if wm.Count("rule1") != 0 {
		t.Errorf("expected count 0 after reset, got %d", wm.Count("rule1"))
	}
}

func TestRuleIsEnabled(t *testing.T) {
	// Default (nil) should be enabled
	r1 := &Rule{Name: "test"}
	if !r1.IsEnabled() {
		t.Error("expected rule to be enabled by default")
	}

	// Explicitly enabled
	enabled := true
	r2 := &Rule{Name: "test", Enabled: &enabled}
	if !r2.IsEnabled() {
		t.Error("expected rule to be enabled when Enabled=true")
	}

	// Explicitly disabled
	disabled := false
	r3 := &Rule{Name: "test", Enabled: &disabled}
	if r3.IsEnabled() {
		t.Error("expected rule to be disabled when Enabled=false")
	}
}
