package alerting

import (
	"fmt"
	"strconv"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Matcher evaluates whether a log entry matches a rule's condition.
type Matcher struct{}

// NewMatcher creates a new Matcher.
func NewMatcher() *Matcher {
	return &Matcher{}
}

// MatchPattern checks if a log entry matches a pattern rule.
func (m *Matcher) MatchPattern(rule *Rule, entry *models.LogEntry) bool {
	if rule.Type != RuleTypePattern {
		return false
	}

	pattern := rule.GetCompiledPattern()
	if pattern == nil {
		return false
	}

	// Check label and log type filters first
	if !rule.MatchesLabels(entry) || !rule.MatchesLogType(entry) {
		return false
	}

	// Match against message
	if pattern.MatchString(entry.Message) {
		return true
	}

	// Match against raw log line if available
	if entry.Raw != "" && pattern.MatchString(entry.Raw) {
		return true
	}

	return false
}

// MatchThresholdCondition checks if a single log entry matches
// the threshold rule's filter condition (not the count threshold).
func (m *Matcher) MatchThresholdCondition(rule *Rule, entry *models.LogEntry) bool {
	if rule.Type != RuleTypeThreshold {
		return false
	}

	// Check label and log type filters first
	if !rule.MatchesLabels(entry) || !rule.MatchesLogType(entry) {
		return false
	}

	cond := rule.Condition

	// If no field is specified, match all entries
	if cond.Field == "" {
		return true
	}

	// Get the value from the entry based on the field
	entryValue := m.getFieldValue(entry, cond.Field)

	// Compare using the operator
	return m.compareValues(entryValue, cond.Value, cond.Operator)
}

// getFieldValue retrieves the value of a field from a log entry.
func (m *Matcher) getFieldValue(entry *models.LogEntry, field string) interface{} {
	switch field {
	case "level":
		return string(entry.Level)
	case "message":
		return entry.Message
	case "type":
		return string(entry.Type)
	case "source":
		return entry.Source
	case "raw":
		return entry.Raw
	case "file_path", "filepath":
		return entry.FilePath
	default:
		// Check in Fields map
		if val, ok := entry.GetField(field); ok {
			return val
		}
		// Check in Labels map
		if val := entry.GetLabel(field); val != "" {
			return val
		}
		return nil
	}
}

// compareValues compares two values using the specified operator.
func (m *Matcher) compareValues(entryValue, condValue interface{}, operator string) bool {
	if entryValue == nil {
		return false
	}

	// Handle string comparison
	if strEntry, ok := entryValue.(string); ok {
		strCond := fmt.Sprintf("%v", condValue)
		switch operator {
		case "==":
			return strEntry == strCond
		case "!=":
			return strEntry != strCond
		case ">":
			return strEntry > strCond
		case ">=":
			return strEntry >= strCond
		case "<":
			return strEntry < strCond
		case "<=":
			return strEntry <= strCond
		}
	}

	// Handle numeric comparison
	entryNum, entryOK := toFloat64(entryValue)
	condNum, condOK := toFloat64(condValue)

	if entryOK && condOK {
		switch operator {
		case "==":
			return entryNum == condNum
		case "!=":
			return entryNum != condNum
		case ">":
			return entryNum > condNum
		case ">=":
			return entryNum >= condNum
		case "<":
			return entryNum < condNum
		case "<=":
			return entryNum <= condNum
		}
	}

	// Fallback to string comparison
	strEntry := fmt.Sprintf("%v", entryValue)
	strCond := fmt.Sprintf("%v", condValue)

	switch operator {
	case "==":
		return strEntry == strCond
	case "!=":
		return strEntry != strCond
	default:
		return false
	}
}

// toFloat64 converts an interface to float64 if possible.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return float64(i), true
		}
		return 0, false
	default:
		return 0, false
	}
}
