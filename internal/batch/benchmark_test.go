package batch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// generateNginxLog creates a test nginx log file with n lines
func generateNginxLog(t testing.TB, dir string, name string, lines int) string {
	t.Helper()

	var buf bytes.Buffer
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	for i := 0; i < lines; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Second)
		ip := fmt.Sprintf("192.168.1.%d", (i%254)+1)
		status := []int{200, 200, 200, 200, 201, 301, 404, 500}[i%8]
		uri := []string{"/", "/index.html", "/api/users", "/api/products", "/about"}[i%5]

		fmt.Fprintf(&buf, "%s - - [%s] \"GET %s HTTP/1.1\" %d %d \"-\" \"Mozilla/5.0\"\n",
			ip,
			ts.Format("02/Jan/2006:15:04:05 -0700"),
			uri,
			status,
			1000+i,
		)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	return path
}

func BenchmarkAnalyze_SingleFile_1000Lines(b *testing.B) {
	dir := b.TempDir()
	path := generateNginxLog(b, dir, "test.log", 1000)

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(context.Background(), []string{path})
		if err != nil {
			b.Fatalf("Analyze failed: %v", err)
		}
	}
}

func BenchmarkAnalyze_SingleFile_10000Lines(b *testing.B) {
	dir := b.TempDir()
	path := generateNginxLog(b, dir, "test.log", 10000)

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(context.Background(), []string{path})
		if err != nil {
			b.Fatalf("Analyze failed: %v", err)
		}
	}
}

func BenchmarkAnalyze_Parallel_4Workers(b *testing.B) {
	dir := b.TempDir()

	// Create 8 files with 5000 lines each
	patterns := make([]string, 8)
	for i := 0; i < 8; i++ {
		path := generateNginxLog(b, dir, fmt.Sprintf("test%d.log", i), 5000)
		patterns[i] = path
	}

	opts := &AnalyzerOptions{
		Workers:    4,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(context.Background(), patterns)
		if err != nil {
			b.Fatalf("Analyze failed: %v", err)
		}
	}
}

func BenchmarkAnalyze_Parallel_8Workers(b *testing.B) {
	dir := b.TempDir()

	// Create 8 files with 5000 lines each
	patterns := make([]string, 8)
	for i := 0; i < 8; i++ {
		path := generateNginxLog(b, dir, fmt.Sprintf("test%d.log", i), 5000)
		patterns[i] = path
	}

	opts := &AnalyzerOptions{
		Workers:    8,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(context.Background(), patterns)
		if err != nil {
			b.Fatalf("Analyze failed: %v", err)
		}
	}
}

func BenchmarkDateFilter_Apply(b *testing.B) {
	filter := NewDateFilter(
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	)

	entries := make([]*models.LogEntry, 1000)
	for i := range entries {
		entries[i] = &models.LogEntry{
			Timestamp: time.Date(2024, 1, 15, 10, i%60, i%60, 0, time.UTC),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, e := range entries {
			filter.Matches(e)
		}
	}
}

func BenchmarkWorkerPool_Throughput(b *testing.B) {
	processor := func(ctx context.Context, path string) (*FileStats, error) {
		return NewFileStats(path), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool := NewWorkerPool(4, 10)
		ctx := context.Background()
		pool.Start(ctx, processor)

		// Submit 10 jobs per iteration
		for j := 0; j < 10; j++ {
			pool.Submit(ctx, fmt.Sprintf("/test/file%d.log", j))
		}
		pool.Close()

		// Drain results
		for range pool.Results() {
		}
	}
}

func BenchmarkExport_JSON_1000Entries(b *testing.B) {
	entries := make([]*models.LogEntry, 1000)
	for i := range entries {
		entries[i] = &models.LogEntry{
			Timestamp: time.Now(),
			Level:     models.LevelInfo,
			Type:      models.LogTypeNginx,
			Message:   fmt.Sprintf("Test message %d", i),
			Source:    "test",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		exporter := NewExporter(ExportJSON, &buf)
		exporter.ExportEntries(entries)
	}
}

func BenchmarkExport_CSV_1000Entries(b *testing.B) {
	entries := make([]*models.LogEntry, 1000)
	for i := range entries {
		entries[i] = &models.LogEntry{
			Timestamp: time.Now(),
			Level:     models.LevelInfo,
			Type:      models.LogTypeNginx,
			Message:   fmt.Sprintf("Test message %d", i),
			Source:    "test",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		exporter := NewExporter(ExportCSV, &buf)
		exporter.ExportEntries(entries)
	}
}

func BenchmarkAggregate_100Files(b *testing.B) {
	files := make([]*FileStats, 100)
	for i := range files {
		files[i] = &FileStats{
			Path:        fmt.Sprintf("/test/file%d.log", i),
			ParsedCount: 10000,
			ErrorCount:  100,
			LevelCounts: map[string]int64{
				"debug":   1000,
				"info":    7000,
				"warning": 1000,
				"error":   900,
				"fatal":   100,
			},
			TypeCounts: map[string]int64{
				"nginx": 10000,
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Aggregate(files, time.Second)
	}
}
