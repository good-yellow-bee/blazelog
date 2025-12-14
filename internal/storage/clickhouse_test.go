package storage

import (
	"context"
	"testing"
	"time"
)

// Unit tests (no ClickHouse required)

func TestLogRecord_Fields(t *testing.T) {
	record := &LogRecord{
		ID:        "test-id",
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Fields: map[string]interface{}{
			"status": float64(200),
			"method": "GET",
		},
		Labels: map[string]string{
			"env": "test",
		},
	}

	if record.Level != "info" {
		t.Errorf("expected level 'info', got %s", record.Level)
	}
	if record.Fields["status"].(float64) != 200 {
		t.Errorf("expected status 200, got %v", record.Fields["status"])
	}
	if record.Labels["env"] != "test" {
		t.Errorf("expected env 'test', got %s", record.Labels["env"])
	}
}

func TestLogFilter_Defaults(t *testing.T) {
	filter := &LogFilter{}

	if filter.Limit != 0 {
		t.Errorf("expected default limit 0, got %d", filter.Limit)
	}
	if filter.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", filter.Offset)
	}
}

func TestLogFilter_TimeRange(t *testing.T) {
	now := time.Now()
	filter := &LogFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
	}

	if filter.StartTime.IsZero() {
		t.Error("expected StartTime to be set")
	}
	if filter.EndTime.IsZero() {
		t.Error("expected EndTime to be set")
	}
}

// LogBuffer unit tests

func TestLogBuffer_AddBatch(t *testing.T) {
	// Create a mock repository
	mock := &mockLogRepo{
		insertBatchCalls: 0,
	}

	config := &LogBufferConfig{
		BatchSize:     3,
		FlushInterval: time.Hour, // Long interval so timer doesn't trigger
		MaxSize:       100,
	}

	buffer := NewLogBuffer(mock, config)
	defer buffer.Close()

	// Add entries below batch size
	err := buffer.AddBatch([]*LogRecord{
		{ID: "1", Message: "test1"},
		{ID: "2", Message: "test2"},
	})
	if err != nil {
		t.Fatalf("AddBatch failed: %v", err)
	}

	// Should not have flushed yet
	if mock.insertBatchCalls != 0 {
		t.Errorf("expected 0 insertBatch calls, got %d", mock.insertBatchCalls)
	}

	// Add more to trigger batch size
	err = buffer.AddBatch([]*LogRecord{
		{ID: "3", Message: "test3"},
	})
	if err != nil {
		t.Fatalf("AddBatch failed: %v", err)
	}

	// Should have flushed
	if mock.insertBatchCalls != 1 {
		t.Errorf("expected 1 insertBatch call, got %d", mock.insertBatchCalls)
	}
	if mock.lastBatchSize != 3 {
		t.Errorf("expected batch size 3, got %d", mock.lastBatchSize)
	}
}

func TestLogBuffer_Flush(t *testing.T) {
	mock := &mockLogRepo{}

	config := &LogBufferConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
		MaxSize:       100,
	}

	buffer := NewLogBuffer(mock, config)
	defer buffer.Close()

	// Add some entries
	buffer.AddBatch([]*LogRecord{
		{ID: "1", Message: "test1"},
	})

	// Manual flush
	err := buffer.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if mock.insertBatchCalls != 1 {
		t.Errorf("expected 1 insertBatch call, got %d", mock.insertBatchCalls)
	}
}

func TestLogBuffer_Backpressure(t *testing.T) {
	mock := &mockLogRepo{
		insertBatchErr: nil,
	}

	config := &LogBufferConfig{
		BatchSize:     10,
		FlushInterval: time.Hour,
		MaxSize:       5, // Small max size to test backpressure
	}

	buffer := NewLogBuffer(mock, config)
	defer buffer.Close()

	// Add more than max size
	entries := make([]*LogRecord, 10)
	for i := 0; i < 10; i++ {
		entries[i] = &LogRecord{ID: string(rune('0' + i)), Message: "test"}
	}

	err := buffer.AddBatch(entries)
	if err != nil {
		t.Fatalf("AddBatch failed: %v", err)
	}

	stats := buffer.Stats()
	if stats.Dropped == 0 {
		t.Error("expected some entries to be dropped")
	}
}

func TestLogBuffer_Stats(t *testing.T) {
	mock := &mockLogRepo{}

	config := &LogBufferConfig{
		BatchSize:     2,
		FlushInterval: time.Hour,
		MaxSize:       100,
	}

	buffer := NewLogBuffer(mock, config)
	defer buffer.Close()

	// Add entries to trigger flush
	buffer.AddBatch([]*LogRecord{
		{ID: "1", Message: "test1"},
		{ID: "2", Message: "test2"},
	})

	stats := buffer.Stats()
	if stats.Flushed != 1 {
		t.Errorf("expected 1 flush, got %d", stats.Flushed)
	}
	if stats.Inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", stats.Inserted)
	}
}

// Mock repository for testing
type mockLogRepo struct {
	insertBatchCalls int
	lastBatchSize    int
	insertBatchErr   error
}

func (m *mockLogRepo) InsertBatch(ctx context.Context, entries []*LogRecord) error {
	m.insertBatchCalls++
	m.lastBatchSize = len(entries)
	return m.insertBatchErr
}

func (m *mockLogRepo) Query(ctx context.Context, filter *LogFilter) (*LogQueryResult, error) {
	return &LogQueryResult{Entries: nil, Total: 0}, nil
}

func (m *mockLogRepo) Count(ctx context.Context, filter *LogFilter) (int64, error) {
	return 0, nil
}

func (m *mockLogRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

// Integration tests are in clickhouse_integration_test.go
// Run with: go test -tags=integration ./internal/storage/...
