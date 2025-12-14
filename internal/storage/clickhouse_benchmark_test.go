//go:build integration

package storage

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Benchmark tests require running ClickHouse.
// Run with: go test -tags=integration -bench=. ./internal/storage/...

var benchmarkLevels = []string{"debug", "info", "warning", "error", "fatal"}
var benchmarkSources = []string{"nginx", "apache", "magento", "prestashop", "wordpress"}
var benchmarkMessages = []string{
	"Request processed successfully",
	"Database connection established",
	"Cache hit for key",
	"User authentication successful",
	"API request completed",
	"Background job executed",
	"Configuration reloaded",
	"Session created",
	"File uploaded",
	"Email sent successfully",
}

func setupBenchmarkData(b *testing.B, store *ClickHouseStorage, count int) {
	b.Helper()

	ctx := context.Background()
	batchSize := 1000
	now := time.Now()

	for i := 0; i < count; i += batchSize {
		entries := make([]*LogRecord, batchSize)
		for j := 0; j < batchSize && i+j < count; j++ {
			entries[j] = &LogRecord{
				Timestamp:  now.Add(-time.Duration(rand.Intn(24*7)) * time.Hour), // Random time in last 7 days
				Level:      benchmarkLevels[rand.Intn(len(benchmarkLevels))],
				Message:    benchmarkMessages[rand.Intn(len(benchmarkMessages))],
				Source:     benchmarkSources[rand.Intn(len(benchmarkSources))],
				Type:       benchmarkSources[rand.Intn(len(benchmarkSources))],
				AgentID:    fmt.Sprintf("agent-%d", rand.Intn(10)),
				HTTPStatus: rand.Intn(5)*100 + 200, // 200, 300, 400, 500, 600
				URI:        fmt.Sprintf("/api/v1/resource/%d", rand.Intn(100)),
			}
		}
		store.Logs().InsertBatch(ctx, entries)
	}
}

func BenchmarkInsertBatch_1000(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	entries := make([]*LogRecord, 1000)
	for i := 0; i < 1000; i++ {
		entries[i] = &LogRecord{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "benchmark test message",
			AgentID:   "benchmark-agent",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().InsertBatch(ctx, entries)
	}
}

func BenchmarkInsertBatch_5000(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	entries := make([]*LogRecord, 5000)
	for i := 0; i < 5000; i++ {
		entries[i] = &LogRecord{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "benchmark test message",
			AgentID:   "benchmark-agent",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().InsertBatch(ctx, entries)
	}
}

func BenchmarkQuery_LastHour(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		Limit:     100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkQuery_Last24Hours(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
		Limit:     100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkQuery_Last7Days(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime: now.Add(-7 * 24 * time.Hour),
		EndTime:   now,
		Limit:     100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkQuery_WithLevelFilter(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
		Level:     "error",
		Limit:     100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkSearch_Token(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime:       now.Add(-24 * time.Hour),
		EndTime:         now,
		MessageContains: "database",
		SearchMode:      SearchModeToken,
		Limit:           100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkSearch_Substring(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime:       now.Add(-24 * time.Hour),
		EndTime:         now,
		MessageContains: "base",
		SearchMode:      SearchModeSubstring,
		Limit:           100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkSearch_Phrase(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &LogFilter{
		StartTime:       now.Add(-24 * time.Hour),
		EndTime:         now,
		MessageContains: "database connection",
		SearchMode:      SearchModePhrase,
		Limit:           100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().Query(ctx, filter)
	}
}

func BenchmarkAggregation_ErrorRates(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &AggregationFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().GetErrorRates(ctx, filter)
	}
}

func BenchmarkAggregation_TopSources(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &AggregationFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().GetTopSources(ctx, filter, 10)
	}
}

func BenchmarkAggregation_LogVolume(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &AggregationFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().GetLogVolume(ctx, filter, "hour")
	}
}

func BenchmarkAggregation_HTTPStats(b *testing.B) {
	store, cleanup := setupClickHouseTest(&testing.T{})
	defer cleanup()

	setupBenchmarkData(b, store, 10000)

	ctx := context.Background()
	now := time.Now()
	filter := &AggregationFilter{
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Logs().GetHTTPStats(ctx, filter)
	}
}
