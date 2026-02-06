package alerting

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Engine is the alert rules engine that evaluates log entries against rules.
type Engine struct {
	mu sync.RWMutex

	rules    []*Rule
	matcher  *Matcher
	windows  *WindowManager
	cooldown *CooldownManager

	// alerts is the channel where triggered alerts are sent.
	alerts chan *Alert

	// closed tracks whether Close has been called to prevent
	// sending on a closed channel.
	closed atomic.Bool

	// stats tracks engine statistics.
	stats *EngineStats
}

// EngineStats tracks engine statistics using atomic operations for lock-free access.
type EngineStats struct {
	EntriesEvaluated  atomic.Int64
	PatternMatches    atomic.Int64
	ThresholdTriggers atomic.Int64
	ExprTriggers      atomic.Int64
	AlertsSuppressed  atomic.Int64
	AlertsDropped     atomic.Int64
}

// EngineOptions configures the alert engine.
type EngineOptions struct {
	// AlertBufferSize is the size of the alert channel buffer.
	AlertBufferSize int
}

// DefaultEngineOptions returns default engine options.
func DefaultEngineOptions() *EngineOptions {
	return &EngineOptions{
		AlertBufferSize: 100,
	}
}

// NewEngine creates a new alert engine with the given rules.
func NewEngine(rules []*Rule, opts *EngineOptions) *Engine {
	if opts == nil {
		opts = DefaultEngineOptions()
	}

	return &Engine{
		rules:    rules,
		matcher:  NewMatcher(),
		windows:  NewWindowManager(),
		cooldown: NewCooldownManager(),
		alerts:   make(chan *Alert, opts.AlertBufferSize),
		stats:    &EngineStats{},
	}
}

// Alerts returns the channel where triggered alerts are sent.
func (e *Engine) Alerts() <-chan *Alert {
	return e.alerts
}

// Evaluate evaluates a single log entry against all rules.
// Returns any triggered alerts.
func (e *Engine) Evaluate(entry *models.LogEntry) []*Alert {
	return e.EvaluateAt(entry, time.Now())
}

// EvaluateAt evaluates a log entry at a specific time (useful for testing).
func (e *Engine) EvaluateAt(entry *models.LogEntry, now time.Time) []*Alert {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	e.stats.EntriesEvaluated.Add(1)

	var alerts []*Alert

	for _, rule := range rules {
		if !rule.IsEnabled() {
			continue
		}

		var alert *Alert

		switch rule.Type {
		case RuleTypePattern:
			alert = e.evaluatePattern(rule, entry, now)
		case RuleTypeThreshold:
			alert = e.evaluateThreshold(rule, entry, now)
		case RuleTypeExpr:
			alert = e.evaluateExpr(rule, entry, now)
		}

		if alert != nil {
			alerts = append(alerts, alert)
			// Send to channel (non-blocking), guarded against closed channel
			if !e.closed.Load() {
				select {
				case e.alerts <- alert:
				default:
					// Channel full, drop alert and track
					dropped := e.stats.AlertsDropped.Add(1)
					if dropped == 1 || dropped%100 == 0 {
						log.Printf("warning: alert channel full, dropped %d alerts total", dropped)
					}
				}
			}
		}
	}

	return alerts
}

// EvaluateStream evaluates log entries from a channel.
func (e *Engine) EvaluateStream(ctx context.Context, entries <-chan *models.LogEntry) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-entries:
			if !ok {
				return
			}
			e.Evaluate(entry)
		}
	}
}

// evaluatePattern evaluates a pattern rule against an entry.
func (e *Engine) evaluatePattern(rule *Rule, entry *models.LogEntry, now time.Time) *Alert {
	if !e.matcher.MatchPattern(rule, entry) {
		return nil
	}

	e.stats.PatternMatches.Add(1)

	// Check cooldown
	if e.cooldown.IsOnCooldown(rule.Name, now) {
		e.stats.AlertsSuppressed.Add(1)
		return nil
	}

	// Set cooldown
	if rule.GetCooldownDuration() > 0 {
		e.cooldown.SetCooldown(rule.Name, rule.GetCooldownDuration(), now)
	}

	return &Alert{
		RuleName:        rule.Name,
		Description:     rule.Description,
		Severity:        rule.Severity,
		Message:         fmt.Sprintf("Pattern match: %s", rule.Condition.Pattern),
		Timestamp:       now,
		TriggeringEntry: entry,
		Notify:          rule.Notify,
		Labels:          rule.Labels,
	}
}

// evaluateThreshold evaluates a threshold rule against an entry.
func (e *Engine) evaluateThreshold(rule *Rule, entry *models.LogEntry, now time.Time) *Alert {
	if !e.matcher.MatchThresholdCondition(rule, entry) {
		return nil
	}

	// Add to sliding window via WindowManager to properly track global event count
	e.windows.AddEventAt(rule.Name, rule.GetWindowDuration(), now)

	// Check if threshold is exceeded
	count := e.windows.Count(rule.Name)
	if count < rule.Condition.Threshold {
		return nil
	}

	e.stats.ThresholdTriggers.Add(1)

	// Check cooldown
	if e.cooldown.IsOnCooldown(rule.Name, now) {
		e.stats.AlertsSuppressed.Add(1)
		return nil
	}

	// Set cooldown
	if rule.GetCooldownDuration() > 0 {
		e.cooldown.SetCooldown(rule.Name, rule.GetCooldownDuration(), now)
	}

	// Reset window after alert (prevents repeated alerts for same events)
	e.windows.Reset(rule.Name)

	return &Alert{
		RuleName:    rule.Name,
		Description: rule.Description,
		Severity:    rule.Severity,
		Message: fmt.Sprintf("Threshold exceeded: %d events in %s (threshold: %d)",
			count, rule.Condition.Window, rule.Condition.Threshold),
		Timestamp: now,
		Count:     count,
		Threshold: rule.Condition.Threshold,
		Window:    rule.Condition.Window,
		Notify:    rule.Notify,
		Labels:    rule.Labels,
	}
}

// evaluateExpr evaluates an expr-based rule against an entry.
func (e *Engine) evaluateExpr(rule *Rule, entry *models.LogEntry, now time.Time) *Alert {
	// Check label and log type filters
	if !rule.MatchesLabels(entry) || !rule.MatchesLogType(entry) {
		return nil
	}

	// Evaluate expression
	matcher := rule.GetExprMatcher()
	if matcher == nil {
		return nil
	}

	matched, err := matcher.Match(entry)
	if err != nil || !matched {
		return nil
	}

	// Add to sliding window via WindowManager to properly track global event count
	agg := rule.Condition.Aggregation
	e.windows.AddEventAt(rule.Name, rule.GetAggregationWindowDuration(), now)

	// Get count and check threshold based on function
	count := e.windows.Count(rule.Name)
	var value float64

	switch agg.Function {
	case "count":
		value = float64(count)
	case "rate":
		// Rate = count per minute
		windowMinutes := rule.GetAggregationWindowDuration().Minutes()
		if windowMinutes > 0 {
			value = float64(count) / windowMinutes
		}
	}

	// Check threshold with operator
	if !compareThreshold(value, agg.Threshold, agg.Operator) {
		return nil
	}

	e.stats.ExprTriggers.Add(1)

	// Check cooldown
	if e.cooldown.IsOnCooldown(rule.Name, now) {
		e.stats.AlertsSuppressed.Add(1)
		return nil
	}

	// Set cooldown
	if rule.GetCooldownDuration() > 0 {
		e.cooldown.SetCooldown(rule.Name, rule.GetCooldownDuration(), now)
	}

	// Reset window after alert (via WindowManager to track global event count)
	e.windows.Reset(rule.Name)

	// Build alert message
	message := fmt.Sprintf("Expression matched: %s (%s %.0f %s %.0f in %s)",
		rule.Condition.Expression, agg.Function, value, agg.Operator, agg.Threshold, agg.Window)

	return &Alert{
		RuleName:    rule.Name,
		Description: rule.Description,
		Severity:    rule.Severity,
		Message:     message,
		Timestamp:   now,
		Count:       count,
		Threshold:   int(agg.Threshold),
		Window:      agg.Window,
		Notify:      rule.Notify,
		Labels:      rule.Labels,
	}
}

// floatEpsilon is the tolerance for float64 equality comparison,
// avoiding unreliable direct == on floating-point values.
const floatEpsilon = 1e-9

// compareThreshold compares a value against a threshold using the given operator.
func compareThreshold(value, threshold float64, operator string) bool {
	switch operator {
	case ">=":
		return value >= threshold
	case ">":
		return value > threshold
	case "<=":
		return value <= threshold
	case "<":
		return value < threshold
	case "==":
		diff := value - threshold
		if diff < 0 {
			diff = -diff
		}
		return diff < floatEpsilon
	case "!=":
		diff := value - threshold
		if diff < 0 {
			diff = -diff
		}
		return diff >= floatEpsilon
	default:
		return false
	}
}

// AddRule adds a new rule to the engine.
func (e *Engine) AddRule(rule *Rule) error {
	if err := rule.Validate(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = append(e.rules, rule)
	return nil
}

// RemoveRule removes a rule by name.
func (e *Engine) RemoveRule(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.Name == name {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			e.windows.Delete(name) // Delete window to prevent memory leak
			e.cooldown.Clear(name)
			return true
		}
	}
	return false
}

// GetRule returns a rule by name.
func (e *Engine) GetRule(name string) *Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if rule.Name == name {
			return rule
		}
	}
	return nil
}

// Rules returns all rules.
func (e *Engine) Rules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Rule, len(e.rules))
	copy(result, e.rules)
	return result
}

// ReloadRules replaces all rules with new ones.
func (e *Engine) ReloadRules(rules []*Rule) error {
	// Validate all rules first
	for _, rule := range rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = rules
	e.windows.DeleteAll() // Delete all windows to prevent memory leaks
	e.cooldown.ClearAll()

	return nil
}

// EngineStatsSnapshot is a snapshot of engine statistics for reporting.
type EngineStatsSnapshot struct {
	EntriesEvaluated  int64
	PatternMatches    int64
	ThresholdTriggers int64
	ExprTriggers      int64
	AlertsSuppressed  int64
	AlertsDropped     int64
}

// Stats returns a snapshot of engine statistics.
func (e *Engine) Stats() EngineStatsSnapshot {
	return EngineStatsSnapshot{
		EntriesEvaluated:  e.stats.EntriesEvaluated.Load(),
		PatternMatches:    e.stats.PatternMatches.Load(),
		ThresholdTriggers: e.stats.ThresholdTriggers.Load(),
		ExprTriggers:      e.stats.ExprTriggers.Load(),
		AlertsSuppressed:  e.stats.AlertsSuppressed.Load(),
		AlertsDropped:     e.stats.AlertsDropped.Load(),
	}
}

// Close closes the engine and releases resources.
// Safe to call concurrently with Evaluate.
func (e *Engine) Close() {
	if e.closed.Swap(true) {
		return // Already closed
	}
	close(e.alerts)
}

// CooldownManager tracks alert cooldowns to prevent spam.
type CooldownManager struct {
	mu        sync.RWMutex
	cooldowns map[string]time.Time
}

// NewCooldownManager creates a new cooldown manager.
func NewCooldownManager() *CooldownManager {
	return &CooldownManager{
		cooldowns: make(map[string]time.Time),
	}
}

// IsOnCooldown checks if a rule is currently on cooldown.
func (cm *CooldownManager) IsOnCooldown(ruleName string, now time.Time) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	expiresAt, ok := cm.cooldowns[ruleName]
	if !ok {
		return false
	}
	return now.Before(expiresAt)
}

// SetCooldown sets a cooldown for a rule.
func (cm *CooldownManager) SetCooldown(ruleName string, duration time.Duration, now time.Time) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.cooldowns[ruleName] = now.Add(duration)
}

// Clear removes cooldown for a rule.
func (cm *CooldownManager) Clear(ruleName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.cooldowns, ruleName)
}

// ClearAll removes all cooldowns.
func (cm *CooldownManager) ClearAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.cooldowns = make(map[string]time.Time)
}

// GetCooldownRemaining returns the remaining cooldown duration for a rule.
func (cm *CooldownManager) GetCooldownRemaining(ruleName string, now time.Time) time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	expiresAt, ok := cm.cooldowns[ruleName]
	if !ok {
		return 0
	}
	remaining := expiresAt.Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}
