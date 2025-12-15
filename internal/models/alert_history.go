// Package models defines domain models for BlazeLog.
package models

import "time"

// AlertHistory records when an alert was triggered.
type AlertHistory struct {
	ID          string    `json:"id"`
	AlertID     string    `json:"alert_id"`
	AlertName   string    `json:"alert_name"`
	Severity    Severity  `json:"severity"`
	Message     string    `json:"message"`
	MatchedLogs int       `json:"matched_logs"`
	NotifiedAt  time.Time `json:"notified_at"`
	ProjectID   string    `json:"project_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
