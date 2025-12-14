// Package storage provides database storage interfaces and implementations.
package storage

import (
	"context"
	"time"
)

// SearchMode defines the full-text search behavior.
type SearchMode int

const (
	// SearchModeToken uses hasToken() for word boundary matching (default).
	SearchModeToken SearchMode = iota
	// SearchModeSubstring uses position() for substring matching.
	SearchModeSubstring
	// SearchModePhrase uses multiple hasToken() for phrase matching.
	SearchModePhrase
)

// LogStorage defines operations for log persistence.
// This is separate from the main Storage interface as logs have
// different access patterns (high-volume writes, time-series queries).
type LogStorage interface {
	// Open initializes the log storage connection.
	Open() error
	// Close closes the log storage connection.
	Close() error
	// Migrate creates or updates the log storage schema.
	Migrate() error
	// Ping checks the connection health.
	Ping(ctx context.Context) error

	// Logs returns the log repository.
	Logs() LogRepository
}

// LogRepository defines log CRUD operations.
type LogRepository interface {
	// InsertBatch inserts multiple log entries in a single batch.
	InsertBatch(ctx context.Context, entries []*LogRecord) error

	// Query retrieves logs matching the given filters.
	Query(ctx context.Context, filter *LogFilter) (*LogQueryResult, error)

	// Count returns the count of logs matching the filter.
	Count(ctx context.Context, filter *LogFilter) (int64, error)

	// DeleteBefore removes logs older than the specified time.
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)

	// GetErrorRates returns error statistics for the given filter.
	GetErrorRates(ctx context.Context, filter *AggregationFilter) (*ErrorRateResult, error)

	// GetTopSources returns the top sources by log count.
	GetTopSources(ctx context.Context, filter *AggregationFilter, limit int) ([]*SourceCount, error)

	// GetLogVolume returns time-series log volume data.
	// interval: "hour", "day", "minute"
	GetLogVolume(ctx context.Context, filter *AggregationFilter, interval string) ([]*VolumePoint, error)

	// GetHTTPStats returns HTTP status code distribution.
	GetHTTPStats(ctx context.Context, filter *AggregationFilter) (*HTTPStatsResult, error)
}

// LogRecord represents a log entry for storage.
type LogRecord struct {
	// ID is the unique identifier for the log entry.
	ID string

	// Timestamp is when the log event occurred.
	Timestamp time.Time

	// Level is the severity level (debug, info, warning, error, fatal, unknown).
	Level string

	// Message is the main log message content.
	Message string

	// Source identifies where the log came from.
	Source string

	// Type is the log format type (nginx, apache, magento, prestashop, wordpress, unknown).
	Type string

	// Raw is the original unparsed log line.
	Raw string

	// AgentID is the ID of the agent that sent the log.
	AgentID string

	// FilePath is the path to the source file.
	FilePath string

	// LineNumber is the position in the source file.
	LineNumber int64

	// Fields contains parser-specific extracted data (status, method, uri, pid, etc.).
	Fields map[string]interface{}

	// Labels are key-value pairs for categorization.
	Labels map[string]string

	// Denormalized fields for fast filtering (extracted from Fields).
	HTTPStatus int
	HTTPMethod string
	URI        string
}

// LogFilter defines query parameters for log retrieval.
type LogFilter struct {
	// Time range (required for efficient queries).
	StartTime time.Time
	EndTime   time.Time

	// Optional filters.
	AgentID  string
	Level    string   // Single level or empty for all.
	Levels   []string // Multiple levels.
	Type     string
	Types    []string
	Source   string
	FilePath string

	// Full-text search.
	MessageContains string
	SearchMode      SearchMode // Token (default), Substring, or Phrase

	// Pagination.
	Limit  int
	Offset int

	// Sorting (default: timestamp DESC).
	OrderBy   string // "timestamp", "level"
	OrderDesc bool
}

// LogQueryResult contains query results with pagination info.
type LogQueryResult struct {
	// Entries contains the matching log records.
	Entries []*LogRecord

	// Total is the total number of matching records (for pagination).
	Total int64

	// HasMore indicates if there are more results available.
	HasMore bool
}

// AggregationFilter defines parameters for aggregation queries.
type AggregationFilter struct {
	StartTime time.Time
	EndTime   time.Time
	AgentID   string
	Type      string
}

// ErrorRateResult contains error statistics.
type ErrorRateResult struct {
	TotalLogs    int64
	ErrorCount   int64
	WarningCount int64
	FatalCount   int64
	ErrorRate    float64 // (error + fatal) / total
}

// SourceCount represents log count per source.
type SourceCount struct {
	Source     string
	Count      int64
	ErrorCount int64
}

// VolumePoint represents a time-series data point.
type VolumePoint struct {
	Timestamp  time.Time
	TotalCount int64
	ErrorCount int64
}

// HTTPStatsResult contains HTTP status code distribution.
type HTTPStatsResult struct {
	Total2xx int64
	Total3xx int64
	Total4xx int64
	Total5xx int64
	TopURIs  []*URICount
}

// URICount represents request count per URI.
type URICount struct {
	URI   string
	Count int64
}
