package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

// ClickHouseConfig holds ClickHouse connection settings.
type ClickHouseConfig struct {
	// Addresses are the ClickHouse server addresses (host:port).
	Addresses []string

	// Database is the ClickHouse database name.
	Database string

	// Username for authentication.
	Username string

	// Password for authentication.
	Password string

	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int

	// DialTimeout is the connection timeout.
	DialTimeout time.Duration

	// Compression enables LZ4 compression.
	Compression bool

	// RetentionDays is the TTL in days for log retention.
	RetentionDays int
}

// ClickHouseStorage implements LogStorage for ClickHouse.
type ClickHouseStorage struct {
	config *ClickHouseConfig
	db     *sql.DB
	logs   *clickhouseLogRepo
}

// NewClickHouseStorage creates a new ClickHouse storage.
func NewClickHouseStorage(config *ClickHouseConfig) *ClickHouseStorage {
	// Apply defaults
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 5
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.RetentionDays == 0 {
		config.RetentionDays = 30
	}

	return &ClickHouseStorage{config: config}
}

// Open initializes the ClickHouse connection.
func (s *ClickHouseStorage) Open() error {
	opts := &clickhouse.Options{
		Addr: s.config.Addresses,
		Auth: clickhouse.Auth{
			Database: s.config.Database,
			Username: s.config.Username,
			Password: s.config.Password,
		},
		DialTimeout:  s.config.DialTimeout,
		MaxOpenConns: s.config.MaxOpenConns,
		MaxIdleConns: s.config.MaxIdleConns,
	}

	if s.config.Compression {
		opts.Compression = &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		}
	}

	db := clickhouse.OpenDB(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), s.config.DialTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}

	s.db = db
	s.logs = &clickhouseLogRepo{db: db}
	return nil
}

// Close closes the database connection.
func (s *ClickHouseStorage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate creates the logs table if it doesn't exist.
func (s *ClickHouseStorage) Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create logs table
	createTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS logs (
			id UUID DEFAULT generateUUIDv4(),
			timestamp DateTime64(3, 'UTC'),
			level LowCardinality(String),
			message String,
			source String,
			type LowCardinality(String),
			raw String,
			agent_id String,
			file_path String,
			line_number Int64,
			fields String,
			labels String,
			http_status UInt16 DEFAULT 0,
			http_method LowCardinality(String) DEFAULT '',
			uri String DEFAULT '',
			_date Date DEFAULT toDate(timestamp)
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMM(_date)
		ORDER BY (agent_id, type, level, timestamp, id)
		TTL _date + INTERVAL %d DAY DELETE
		SETTINGS index_granularity = 8192
	`, s.config.RetentionDays)

	if _, err := s.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("create logs table: %w", err)
	}

	// Add indexes (these are idempotent in ClickHouse)
	indexes := []string{
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_message message TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 4",
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_source source TYPE bloom_filter(0.01) GRANULARITY 4",
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_file_path file_path TYPE bloom_filter(0.01) GRANULARITY 4",
		// Advanced indexes (Milestone 21)
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_message_ngram message TYPE ngrambf_v1(3, 65536, 3, 0) GRANULARITY 4",
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_timestamp_minmax timestamp TYPE minmax GRANULARITY 3",
		"ALTER TABLE logs ADD INDEX IF NOT EXISTS idx_http_status http_status TYPE set(100) GRANULARITY 4",
	}

	for _, idx := range indexes {
		if _, err := s.db.ExecContext(ctx, idx); err != nil {
			// Log warning but don't fail - index creation may not be supported in all ClickHouse versions
			fmt.Printf("warning: failed to create index: %v\n", err)
		}
	}

	// Create materialized views for dashboards (Milestone 21)
	materializedViews := []string{
		// Hourly error counts for error rate dashboards
		`CREATE MATERIALIZED VIEW IF NOT EXISTS logs_hourly_errors_mv
		ENGINE = SummingMergeTree()
		PARTITION BY toYYYYMM(hour)
		ORDER BY (agent_id, type, level, hour)
		AS SELECT
			agent_id,
			type,
			level,
			toStartOfHour(timestamp) AS hour,
			count() AS count
		FROM logs
		WHERE level IN ('error', 'fatal', 'warning')
		GROUP BY agent_id, type, level, hour`,

		// Daily log volume for capacity planning
		`CREATE MATERIALIZED VIEW IF NOT EXISTS logs_daily_volume_mv
		ENGINE = SummingMergeTree()
		PARTITION BY toYYYYMM(day)
		ORDER BY (agent_id, type, day)
		AS SELECT
			agent_id,
			type,
			toDate(timestamp) AS day,
			count() AS total_count,
			countIf(level = 'error') AS error_count,
			countIf(level = 'fatal') AS fatal_count,
			countIf(level = 'warning') AS warning_count
		FROM logs
		GROUP BY agent_id, type, day`,

		// HTTP status distribution for web server monitoring
		`CREATE MATERIALIZED VIEW IF NOT EXISTS logs_http_stats_mv
		ENGINE = SummingMergeTree()
		PARTITION BY toYYYYMM(hour)
		ORDER BY (agent_id, hour, http_status)
		AS SELECT
			agent_id,
			toStartOfHour(timestamp) AS hour,
			http_status,
			count() AS count
		FROM logs
		WHERE http_status > 0
		GROUP BY agent_id, hour, http_status`,
	}

	for _, mv := range materializedViews {
		if _, err := s.db.ExecContext(ctx, mv); err != nil {
			fmt.Printf("warning: failed to create materialized view: %v\n", err)
		}
	}

	return nil
}

// Ping checks the connection health.
func (s *ClickHouseStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Logs returns the log repository.
func (s *ClickHouseStorage) Logs() LogRepository {
	return s.logs
}

// clickhouseLogRepo implements LogRepository for ClickHouse.
type clickhouseLogRepo struct {
	db *sql.DB
}

// InsertBatch inserts multiple log entries using batch insert.
func (r *clickhouseLogRepo) InsertBatch(ctx context.Context, entries []*LogRecord) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO logs (
			id, timestamp, level, message, source, type, raw,
			agent_id, file_path, line_number, fields, labels,
			http_status, http_method, uri
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		id := entry.ID
		if id == "" {
			id = uuid.New().String()
		}

		fieldsJSON, _ := json.Marshal(entry.Fields)
		labelsJSON, _ := json.Marshal(entry.Labels)

		_, err := stmt.ExecContext(ctx,
			id,
			entry.Timestamp,
			entry.Level,
			entry.Message,
			entry.Source,
			entry.Type,
			entry.Raw,
			entry.AgentID,
			entry.FilePath,
			entry.LineNumber,
			string(fieldsJSON),
			string(labelsJSON),
			entry.HTTPStatus,
			entry.HTTPMethod,
			entry.URI,
		)
		if err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// Query retrieves logs matching the filter.
func (r *clickhouseLogRepo) Query(ctx context.Context, filter *LogFilter) (*LogQueryResult, error) {
	query, args := r.buildQuery(filter, false)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var entries []*LogRecord
	for rows.Next() {
		entry := &LogRecord{}
		var fieldsJSON, labelsJSON string

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Level,
			&entry.Message,
			&entry.Source,
			&entry.Type,
			&entry.Raw,
			&entry.AgentID,
			&entry.FilePath,
			&entry.LineNumber,
			&fieldsJSON,
			&labelsJSON,
			&entry.HTTPStatus,
			&entry.HTTPMethod,
			&entry.URI,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		// Parse JSON fields
		if fieldsJSON != "" {
			json.Unmarshal([]byte(fieldsJSON), &entry.Fields)
		}
		if labelsJSON != "" {
			json.Unmarshal([]byte(labelsJSON), &entry.Labels)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	// Get total count
	total, err := r.Count(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}

	return &LogQueryResult{
		Entries: entries,
		Total:   total,
		HasMore: int64(filter.Offset+len(entries)) < total,
	}, nil
}

// Count returns the count of logs matching the filter.
func (r *clickhouseLogRepo) Count(ctx context.Context, filter *LogFilter) (int64, error) {
	query, args := r.buildQuery(filter, true)

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return count, nil
}

// DeleteBefore removes logs older than the specified time.
func (r *clickhouseLogRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	// First get count for return value
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT count() FROM logs WHERE timestamp < ?", before).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	// Delete using ALTER TABLE DELETE (async in ClickHouse)
	_, err = r.db.ExecContext(ctx, "ALTER TABLE logs DELETE WHERE timestamp < ?", before)
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}

	return count, nil
}

// buildQuery constructs the SQL query based on filter.
func (r *clickhouseLogRepo) buildQuery(filter *LogFilter, countOnly bool) (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}
	var prewhereArgs []interface{}

	if countOnly {
		sb.WriteString("SELECT count() FROM logs")
	} else {
		sb.WriteString(`
			SELECT id, timestamp, level, message, source, type, raw,
			       agent_id, file_path, line_number, fields, labels,
			       http_status, http_method, uri
			FROM logs
		`)
	}

	// Build PREWHERE clause for indexed columns (timestamp optimization)
	var prewhereConditions []string
	if !filter.StartTime.IsZero() {
		prewhereConditions = append(prewhereConditions, "timestamp >= ?")
		prewhereArgs = append(prewhereArgs, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		prewhereConditions = append(prewhereConditions, "timestamp <= ?")
		prewhereArgs = append(prewhereArgs, filter.EndTime)
	}

	// Build WHERE clause
	var conditions []string

	// Agent filter
	if filter.AgentID != "" {
		conditions = append(conditions, "agent_id = ?")
		args = append(args, filter.AgentID)
	}

	// Level filter
	if filter.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filter.Level)
	}
	if len(filter.Levels) > 0 {
		placeholders := make([]string, len(filter.Levels))
		for i, l := range filter.Levels {
			placeholders[i] = "?"
			args = append(args, l)
		}
		conditions = append(conditions, fmt.Sprintf("level IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Type filter
	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filter.Type)
	}
	if len(filter.Types) > 0 {
		placeholders := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Source filter
	if filter.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, filter.Source)
	}

	// File path filter
	if filter.FilePath != "" {
		conditions = append(conditions, "file_path = ?")
		args = append(args, filter.FilePath)
	}

	// Full-text search on message with search mode support (Milestone 21)
	if filter.MessageContains != "" {
		switch filter.SearchMode {
		case SearchModeSubstring:
			conditions = append(conditions, "position(message, ?) > 0")
			args = append(args, filter.MessageContains)
		case SearchModePhrase:
			words := strings.Fields(filter.MessageContains)
			for _, word := range words {
				conditions = append(conditions, "hasToken(message, ?)")
				args = append(args, word)
			}
		default: // SearchModeToken
			conditions = append(conditions, "hasToken(message, ?)")
			args = append(args, filter.MessageContains)
		}
	}

	// Append PREWHERE clause (ClickHouse optimization for indexed columns)
	if len(prewhereConditions) > 0 {
		sb.WriteString(" PREWHERE ")
		sb.WriteString(strings.Join(prewhereConditions, " AND "))
	}

	// Append WHERE clause
	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// Combine args: prewhere args first, then where args
	allArgs := append(prewhereArgs, args...)

	// Skip ORDER BY and LIMIT for count queries
	if countOnly {
		return sb.String(), allArgs
	}

	// ORDER BY
	orderBy := "timestamp"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	orderDir := "DESC"
	if !filter.OrderDesc && filter.OrderBy != "" {
		orderDir = "ASC"
	}
	sb.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir))

	// LIMIT and OFFSET
	limit := filter.Limit
	if limit == 0 {
		limit = 100 // Default limit
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	if filter.Offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", filter.Offset))
	}

	return sb.String(), allArgs
}

// GetErrorRates returns error statistics for the given filter.
func (r *clickhouseLogRepo) GetErrorRates(ctx context.Context, filter *AggregationFilter) (*ErrorRateResult, error) {
	query := `
		SELECT
			count() AS total,
			countIf(level = 'error') AS errors,
			countIf(level = 'warning') AS warnings,
			countIf(level = 'fatal') AS fatals
		FROM logs
	`
	args, whereClause := r.buildAggregationWhere(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	result := &ErrorRateResult{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&result.TotalLogs,
		&result.ErrorCount,
		&result.WarningCount,
		&result.FatalCount,
	)
	if err != nil {
		return nil, fmt.Errorf("get error rates: %w", err)
	}

	if result.TotalLogs > 0 {
		result.ErrorRate = float64(result.ErrorCount+result.FatalCount) / float64(result.TotalLogs)
	}

	return result, nil
}

// GetTopSources returns the top sources by log count.
func (r *clickhouseLogRepo) GetTopSources(ctx context.Context, filter *AggregationFilter, limit int) ([]*SourceCount, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			source,
			count() AS total,
			countIf(level IN ('error', 'fatal')) AS errors
		FROM logs
	`
	args, whereClause := r.buildAggregationWhere(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += fmt.Sprintf(" GROUP BY source ORDER BY total DESC LIMIT %d", limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get top sources: %w", err)
	}
	defer rows.Close()

	var results []*SourceCount
	for rows.Next() {
		sc := &SourceCount{}
		if err := rows.Scan(&sc.Source, &sc.Count, &sc.ErrorCount); err != nil {
			return nil, fmt.Errorf("scan source count: %w", err)
		}
		results = append(results, sc)
	}

	return results, rows.Err()
}

// GetLogVolume returns time-series log volume data.
func (r *clickhouseLogRepo) GetLogVolume(ctx context.Context, filter *AggregationFilter, interval string) ([]*VolumePoint, error) {
	// Determine time function based on interval
	var timeFunc string
	switch interval {
	case "minute":
		timeFunc = "toStartOfMinute(timestamp)"
	case "day":
		timeFunc = "toStartOfDay(timestamp)"
	default: // hour
		timeFunc = "toStartOfHour(timestamp)"
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS ts,
			count() AS total,
			countIf(level IN ('error', 'fatal')) AS errors
		FROM logs
	`, timeFunc)

	args, whereClause := r.buildAggregationWhere(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " GROUP BY ts ORDER BY ts ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get log volume: %w", err)
	}
	defer rows.Close()

	var results []*VolumePoint
	for rows.Next() {
		vp := &VolumePoint{}
		if err := rows.Scan(&vp.Timestamp, &vp.TotalCount, &vp.ErrorCount); err != nil {
			return nil, fmt.Errorf("scan volume point: %w", err)
		}
		results = append(results, vp)
	}

	return results, rows.Err()
}

// GetHTTPStats returns HTTP status code distribution.
func (r *clickhouseLogRepo) GetHTTPStats(ctx context.Context, filter *AggregationFilter) (*HTTPStatsResult, error) {
	query := `
		SELECT
			countIf(http_status >= 200 AND http_status < 300) AS total_2xx,
			countIf(http_status >= 300 AND http_status < 400) AS total_3xx,
			countIf(http_status >= 400 AND http_status < 500) AS total_4xx,
			countIf(http_status >= 500 AND http_status < 600) AS total_5xx
		FROM logs
	`
	args, whereClause := r.buildAggregationWhere(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause + " AND http_status > 0"
	} else {
		query += " WHERE http_status > 0"
	}

	result := &HTTPStatsResult{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&result.Total2xx,
		&result.Total3xx,
		&result.Total4xx,
		&result.Total5xx,
	)
	if err != nil {
		return nil, fmt.Errorf("get http stats: %w", err)
	}

	// Get top URIs
	uriQuery := `
		SELECT uri, count() AS cnt
		FROM logs
	`
	if whereClause != "" {
		uriQuery += " WHERE " + whereClause + " AND http_status > 0 AND uri != ''"
	} else {
		uriQuery += " WHERE http_status > 0 AND uri != ''"
	}
	uriQuery += " GROUP BY uri ORDER BY cnt DESC LIMIT 10"

	rows, err := r.db.QueryContext(ctx, uriQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("get top uris: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		uc := &URICount{}
		if err := rows.Scan(&uc.URI, &uc.Count); err != nil {
			return nil, fmt.Errorf("scan uri count: %w", err)
		}
		result.TopURIs = append(result.TopURIs, uc)
	}

	return result, rows.Err()
}

// buildAggregationWhere builds WHERE clause for aggregation queries.
func (r *clickhouseLogRepo) buildAggregationWhere(filter *AggregationFilter) ([]interface{}, string) {
	var conditions []string
	var args []interface{}

	if !filter.StartTime.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.EndTime)
	}
	if filter.AgentID != "" {
		conditions = append(conditions, "agent_id = ?")
		args = append(args, filter.AgentID)
	}
	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filter.Type)
	}

	return args, strings.Join(conditions, " AND ")
}
