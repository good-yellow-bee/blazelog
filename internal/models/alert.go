package models

import (
	"encoding/json"
	"time"
)

// AlertType represents the type of alert rule.
type AlertType string

const (
	AlertTypePattern   AlertType = "pattern"
	AlertTypeThreshold AlertType = "threshold"
)

// Severity represents alert severity level.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// AlertRule represents a persistent alert configuration.
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        AlertType     `json:"type"`
	Condition   string        `json:"condition"` // JSON-encoded condition
	Severity    Severity      `json:"severity"`
	Window      time.Duration `json:"window"`
	Cooldown    time.Duration `json:"cooldown"`
	Notify      []string      `json:"notify"`
	Enabled     bool          `json:"enabled"`
	ProjectID   string        `json:"project_id,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// NewAlertRule creates a new AlertRule with initialized timestamps.
func NewAlertRule(name string, alertType AlertType, severity Severity) *AlertRule {
	now := time.Now()
	return &AlertRule{
		Name:      name,
		Type:      alertType,
		Severity:  severity,
		Enabled:   true,
		Notify:    []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SetCondition sets the condition from a structured value.
func (a *AlertRule) SetCondition(condition interface{}) error {
	data, err := json.Marshal(condition)
	if err != nil {
		return err
	}
	a.Condition = string(data)
	return nil
}

// GetCondition unmarshals the condition into the provided target.
func (a *AlertRule) GetCondition(target interface{}) error {
	return json.Unmarshal([]byte(a.Condition), target)
}

// ParseAlertType converts a string to AlertType.
func ParseAlertType(s string) AlertType {
	switch s {
	case "pattern":
		return AlertTypePattern
	case "threshold":
		return AlertTypeThreshold
	default:
		return AlertTypePattern
	}
}

// ParseSeverity converts a string to Severity.
func ParseSeverity(s string) Severity {
	switch s {
	case "low":
		return SeverityLow
	case "medium":
		return SeverityMedium
	case "high":
		return SeverityHigh
	case "critical":
		return SeverityCritical
	default:
		return SeverityMedium
	}
}
