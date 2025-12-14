//go:build integration

package storage

import (
	"context"
	"testing"
	"time"
)

// Integration tests require running ClickHouse.
// Run with: go test -tags=integration ./internal/storage/...

func setupClickHouseTest(t *testing.T) (*ClickHouseStorage, func()) {
	t.Helper()

	config := &ClickHouseConfig{
		Addresses:     []string{"localhost:9000"},
		Database:      "blazelog_test",
		Username:      "default",
		Password:      "",
		MaxOpenConns:  2,
		MaxIdleConns:  2,
		DialTimeout:   5 * time.Second,
		Compression:   true,
		RetentionDays: 1,
	}

	storage := NewClickHouseStorage(config)
	if err := storage.Open(); err != nil {
		t.Skipf("ClickHouse not available: %v", err)
	}

	if err := storage.Migrate(); err != nil {
		storage.Close()
		t.Fatalf("migrate: %v", err)
	}

	cleanup := func() {
		// Truncate test table
		storage.db.Exec("TRUNCATE TABLE logs")
		storage.Close()
	}

	return storage, cleanup
}

func TestClickHouseStorage_InsertBatch_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()
	entries := []*LogRecord{
		{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Test log message",
			Source:    "test",
			Type:      "nginx",
			AgentID:   "test-agent",
			Fields:    map[string]interface{}{"status": float64(200)},
			Labels:    map[string]string{"env": "test"},
		},
	}

	err := store.Logs().InsertBatch(ctx, entries)
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}

	// Verify insertion
	result, err := store.Logs().Query(ctx, &LogFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		AgentID:   "test-agent",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
}

func TestClickHouseStorage_Query_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	entries := []*LogRecord{
		{Timestamp: time.Now(), Level: "info", Message: "test1", AgentID: "agent1", Type: "nginx"},
		{Timestamp: time.Now(), Level: "error", Message: "test2", AgentID: "agent1", Type: "nginx"},
		{Timestamp: time.Now(), Level: "info", Message: "test3", AgentID: "agent2", Type: "apache"},
	}
	store.Logs().InsertBatch(ctx, entries)

	// Query by agent
	result, err := store.Logs().Query(ctx, &LogFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		AgentID:   "agent1",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries for agent1, got %d", len(result.Entries))
	}

	// Query by level
	result, err = store.Logs().Query(ctx, &LogFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		Level:     "error",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 error entry, got %d", len(result.Entries))
	}
}

func TestClickHouseStorage_Count_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	entries := make([]*LogRecord, 5)
	for i := 0; i < 5; i++ {
		entries[i] = &LogRecord{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "test",
			AgentID:   "test-agent",
		}
	}
	store.Logs().InsertBatch(ctx, entries)

	// Count
	count, err := store.Logs().Count(ctx, &LogFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		AgentID:   "test-agent",
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

func TestClickHouseStorage_DeleteBefore_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert old data
	oldTime := time.Now().Add(-48 * time.Hour)
	entries := []*LogRecord{
		{Timestamp: oldTime, Level: "info", Message: "old", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Message: "new", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	// Delete old entries
	deleted, err := store.Logs().DeleteBefore(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

// Milestone 21: Advanced ClickHouse Integration Tests

func TestClickHouseStorage_SearchModes_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data with various messages
	entries := []*LogRecord{
		{Timestamp: time.Now(), Level: "info", Message: "database connection established", AgentID: "test"},
		{Timestamp: time.Now(), Level: "error", Message: "database error occurred", AgentID: "test"},
		{Timestamp: time.Now(), Level: "warning", Message: "slow database query detected", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Message: "user logged in successfully", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	// Test SearchModeToken (default) - word boundary matching
	result, err := store.Logs().Query(ctx, &LogFilter{
		StartTime:       time.Now().Add(-time.Hour),
		EndTime:         time.Now().Add(time.Hour),
		MessageContains: "database",
		SearchMode:      SearchModeToken,
	})
	if err != nil {
		t.Fatalf("query token mode: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Errorf("token search: expected 3 entries with 'database', got %d", len(result.Entries))
	}

	// Test SearchModeSubstring - substring matching
	result, err = store.Logs().Query(ctx, &LogFilter{
		StartTime:       time.Now().Add(-time.Hour),
		EndTime:         time.Now().Add(time.Hour),
		MessageContains: "base",
		SearchMode:      SearchModeSubstring,
	})
	if err != nil {
		t.Fatalf("query substring mode: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Errorf("substring search: expected 3 entries with 'base', got %d", len(result.Entries))
	}

	// Test SearchModePhrase - multiple words AND matching
	result, err = store.Logs().Query(ctx, &LogFilter{
		StartTime:       time.Now().Add(-time.Hour),
		EndTime:         time.Now().Add(time.Hour),
		MessageContains: "database error",
		SearchMode:      SearchModePhrase,
	})
	if err != nil {
		t.Fatalf("query phrase mode: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("phrase search: expected 1 entry with 'database' AND 'error', got %d", len(result.Entries))
	}
}

func TestClickHouseStorage_GetErrorRates_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	entries := []*LogRecord{
		{Timestamp: time.Now(), Level: "info", Message: "test", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Message: "test", AgentID: "test"},
		{Timestamp: time.Now(), Level: "warning", Message: "test", AgentID: "test"},
		{Timestamp: time.Now(), Level: "error", Message: "test", AgentID: "test"},
		{Timestamp: time.Now(), Level: "fatal", Message: "test", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	result, err := store.Logs().GetErrorRates(ctx, &AggregationFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("get error rates: %v", err)
	}

	if result.TotalLogs != 5 {
		t.Errorf("expected TotalLogs 5, got %d", result.TotalLogs)
	}
	if result.ErrorCount != 1 {
		t.Errorf("expected ErrorCount 1, got %d", result.ErrorCount)
	}
	if result.FatalCount != 1 {
		t.Errorf("expected FatalCount 1, got %d", result.FatalCount)
	}
	if result.WarningCount != 1 {
		t.Errorf("expected WarningCount 1, got %d", result.WarningCount)
	}
	// ErrorRate = (error + fatal) / total = 2/5 = 0.4
	if result.ErrorRate != 0.4 {
		t.Errorf("expected ErrorRate 0.4, got %f", result.ErrorRate)
	}
}

func TestClickHouseStorage_GetTopSources_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	entries := []*LogRecord{
		{Timestamp: time.Now(), Level: "info", Source: "nginx", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Source: "nginx", AgentID: "test"},
		{Timestamp: time.Now(), Level: "error", Source: "nginx", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Source: "apache", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", Source: "mysql", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	result, err := store.Logs().GetTopSources(ctx, &AggregationFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	}, 10)
	if err != nil {
		t.Fatalf("get top sources: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 sources, got %d", len(result))
	}
	// Nginx should be first (3 entries)
	if len(result) > 0 && result[0].Source != "nginx" {
		t.Errorf("expected nginx as top source, got %s", result[0].Source)
	}
	if len(result) > 0 && result[0].Count != 3 {
		t.Errorf("expected nginx count 3, got %d", result[0].Count)
	}
	if len(result) > 0 && result[0].ErrorCount != 1 {
		t.Errorf("expected nginx error count 1, got %d", result[0].ErrorCount)
	}
}

func TestClickHouseStorage_GetLogVolume_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	now := time.Now()
	entries := []*LogRecord{
		{Timestamp: now, Level: "info", AgentID: "test"},
		{Timestamp: now, Level: "error", AgentID: "test"},
		{Timestamp: now.Add(-time.Hour), Level: "info", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	result, err := store.Logs().GetLogVolume(ctx, &AggregationFilter{
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(time.Hour),
	}, "hour")
	if err != nil {
		t.Fatalf("get log volume: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected at least 1 volume point")
	}

	// Verify total counts
	var totalCount int64
	for _, vp := range result {
		totalCount += vp.TotalCount
	}
	if totalCount != 3 {
		t.Errorf("expected total count 3, got %d", totalCount)
	}
}

func TestClickHouseStorage_GetHTTPStats_Integration(t *testing.T) {
	store, cleanup := setupClickHouseTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data with HTTP status codes
	entries := []*LogRecord{
		{Timestamp: time.Now(), Level: "info", HTTPStatus: 200, URI: "/api/users", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", HTTPStatus: 201, URI: "/api/users", AgentID: "test"},
		{Timestamp: time.Now(), Level: "info", HTTPStatus: 301, URI: "/old-page", AgentID: "test"},
		{Timestamp: time.Now(), Level: "warning", HTTPStatus: 404, URI: "/not-found", AgentID: "test"},
		{Timestamp: time.Now(), Level: "error", HTTPStatus: 500, URI: "/api/error", AgentID: "test"},
		{Timestamp: time.Now(), Level: "error", HTTPStatus: 503, URI: "/api/error", AgentID: "test"},
	}
	store.Logs().InsertBatch(ctx, entries)

	result, err := store.Logs().GetHTTPStats(ctx, &AggregationFilter{
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("get http stats: %v", err)
	}

	if result.Total2xx != 2 {
		t.Errorf("expected Total2xx 2, got %d", result.Total2xx)
	}
	if result.Total3xx != 1 {
		t.Errorf("expected Total3xx 1, got %d", result.Total3xx)
	}
	if result.Total4xx != 1 {
		t.Errorf("expected Total4xx 1, got %d", result.Total4xx)
	}
	if result.Total5xx != 2 {
		t.Errorf("expected Total5xx 2, got %d", result.Total5xx)
	}
}
