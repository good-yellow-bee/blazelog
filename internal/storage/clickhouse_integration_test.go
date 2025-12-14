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
