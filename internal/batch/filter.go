package batch

import (
	"fmt"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// DateFilter filters log entries by date range.
type DateFilter struct {
	From    time.Time
	To      time.Time
	Enabled bool
}

// NewDateFilter creates a filter from parsed from/to times.
// If both are zero, filter is disabled.
func NewDateFilter(from, to time.Time) *DateFilter {
	return &DateFilter{
		From:    from,
		To:      to,
		Enabled: !from.IsZero() || !to.IsZero(),
	}
}

// Matches returns true if entry timestamp is within the date range.
// If filter is disabled, always returns true.
func (f *DateFilter) Matches(entry *models.LogEntry) bool {
	if !f.Enabled {
		return true
	}

	ts := entry.Timestamp

	// Check from bound (inclusive)
	if !f.From.IsZero() && ts.Before(f.From) {
		return false
	}

	// Check to bound (inclusive - we use end of day)
	if !f.To.IsZero() && ts.After(f.To) {
		return false
	}

	return true
}

// MatchesTime checks if a timestamp is within the date range.
func (f *DateFilter) MatchesTime(ts time.Time) bool {
	if !f.Enabled {
		return true
	}

	if !f.From.IsZero() && ts.Before(f.From) {
		return false
	}

	if !f.To.IsZero() && ts.After(f.To) {
		return false
	}

	return true
}

// ParseDateFlag parses a date string in YYYY-MM-DD or RFC3339 format.
// For YYYY-MM-DD, it returns start of day in local timezone.
func ParseDateFlag(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try date-only format (YYYY-MM-DD)
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid date format: %q (expected YYYY-MM-DD or RFC3339)", s)
}

// ParseDateFlagEndOfDay parses a date string and returns end of day for YYYY-MM-DD format.
// For RFC3339, returns the exact time. For YYYY-MM-DD, returns 23:59:59.999999999.
func ParseDateFlagEndOfDay(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Try RFC3339 first - return exact time
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try date-only format - return end of day
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t.Add(24*time.Hour - time.Nanosecond), nil
	}

	return time.Time{}, fmt.Errorf("invalid date format: %q (expected YYYY-MM-DD or RFC3339)", s)
}
