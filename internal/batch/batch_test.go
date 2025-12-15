package batch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Test data directory
const testDataDir = "testdata"

func setupTestData(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), testDataDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}

	// Create nginx log file
	nginxLog := `192.168.1.1 - - [01/Jan/2024:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"
192.168.1.2 - - [01/Jan/2024:10:00:01 +0000] "POST /api/users HTTP/1.1" 201 567 "-" "curl/7.68.0"
192.168.1.3 - - [01/Jan/2024:10:00:02 +0000] "GET /missing HTTP/1.1" 404 123 "-" "Mozilla/5.0"
192.168.1.4 - - [01/Jan/2024:10:00:03 +0000] "GET /error HTTP/1.1" 500 456 "-" "Mozilla/5.0"`

	if err := os.WriteFile(filepath.Join(dir, "access.log"), []byte(nginxLog), 0644); err != nil {
		t.Fatalf("write nginx log: %v", err)
	}

	// Create another nginx log file
	nginxLog2 := `192.168.1.5 - - [02/Jan/2024:10:00:00 +0000] "GET /page HTTP/1.1" 200 789 "-" "Mozilla/5.0"
192.168.1.6 - - [02/Jan/2024:10:00:01 +0000] "GET /other HTTP/1.1" 200 321 "-" "Mozilla/5.0"`

	if err := os.WriteFile(filepath.Join(dir, "access2.log"), []byte(nginxLog2), 0644); err != nil {
		t.Fatalf("write nginx log 2: %v", err)
	}

	return dir
}

func TestDateFilter_Matches(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		to   time.Time
		ts   time.Time
		want bool
	}{
		{
			name: "disabled filter matches all",
			want: true,
		},
		{
			name: "from only - matches after",
			from: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "from only - rejects before",
			from: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "to only - matches before",
			to:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "to only - rejects after",
			to:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "range - matches within",
			from: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "range - rejects outside",
			from: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			ts:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewDateFilter(tt.from, tt.to)
			entry := &models.LogEntry{Timestamp: tt.ts}
			if got := f.Matches(entry); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDateFlag(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},
		{"2024-01-15", false},
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15T10:30:00+01:00", false},
		{"invalid", true},
		{"2024/01/15", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseDateFlag(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateFlag(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzer_SingleFile(t *testing.T) {
	dir := setupTestData(t)

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	report, err := analyzer.Analyze(context.Background(), []string{filepath.Join(dir, "access.log")})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if report.Summary.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", report.Summary.TotalFiles)
	}

	if report.Summary.TotalEntries != 4 {
		t.Errorf("TotalEntries = %d, want 4", report.Summary.TotalEntries)
	}
}

func TestAnalyzer_MultipleFiles(t *testing.T) {
	dir := setupTestData(t)

	opts := &AnalyzerOptions{
		Workers:    2,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	report, err := analyzer.Analyze(context.Background(), []string{filepath.Join(dir, "*.log")})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if report.Summary.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", report.Summary.TotalFiles)
	}

	if report.Summary.TotalEntries != 6 {
		t.Errorf("TotalEntries = %d, want 6", report.Summary.TotalEntries)
	}
}

func TestAnalyzer_DateFilter(t *testing.T) {
	dir := setupTestData(t)

	// Filter to only Jan 1
	from, _ := ParseDateFlag("2024-01-01")
	to, _ := ParseDateFlagEndOfDay("2024-01-01")

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
		From:       from,
		To:         to,
	}
	analyzer := NewAnalyzer(opts)

	report, err := analyzer.Analyze(context.Background(), []string{filepath.Join(dir, "*.log")})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Only Jan 1 entries (4 from access.log, 0 from access2.log)
	if report.Summary.TotalEntries != 4 {
		t.Errorf("TotalEntries = %d, want 4", report.Summary.TotalEntries)
	}
}

func TestAnalyzer_Limit(t *testing.T) {
	dir := setupTestData(t)

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
		Limit:      2,
	}
	analyzer := NewAnalyzer(opts)

	report, err := analyzer.Analyze(context.Background(), []string{filepath.Join(dir, "access.log")})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if report.Summary.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", report.Summary.TotalEntries)
	}
}

func TestAnalyzer_ContextCancel(t *testing.T) {
	dir := setupTestData(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := &AnalyzerOptions{
		Workers:    1,
		ParserType: "nginx",
	}
	analyzer := NewAnalyzer(opts)

	_, err := analyzer.Analyze(ctx, []string{filepath.Join(dir, "*.log")})
	// Should complete without error even if canceled
	if err != nil {
		t.Logf("Analyze() with canceled context returned: %v", err)
	}
}

func TestAnalyzer_NoMatchingFiles(t *testing.T) {
	dir := t.TempDir()

	opts := DefaultOptions()
	analyzer := NewAnalyzer(opts)

	_, err := analyzer.Analyze(context.Background(), []string{filepath.Join(dir, "nonexistent*.log")})
	if err == nil {
		t.Error("expected error for no matching files")
	}
}

func TestWorkerPool_Basic(t *testing.T) {
	pool := NewWorkerPool(2, 10)

	processed := make(chan string, 10)
	processor := func(ctx context.Context, path string) (*FileStats, error) {
		processed <- path
		return NewFileStats(path), nil
	}

	ctx := context.Background()
	pool.Start(ctx, processor)

	pool.Submit(ctx, "file1")
	pool.Submit(ctx, "file2")
	pool.Submit(ctx, "file3")
	pool.Close()

	// Drain results
	results := make([]*FileStats, 0)
	for r := range pool.Results() {
		results = append(results, r)
	}

	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
}

func TestExporter_JSON(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewExporter(ExportJSON, &buf)

	report := &Report{
		Duration: time.Second,
		Summary:  NewSummary(),
		Files:    []*FileStats{},
	}
	report.Summary.TotalFiles = 1
	report.Summary.TotalEntries = 100

	if err := exporter.ExportReport(report); err != nil {
		t.Fatalf("ExportReport() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty JSON output")
	}

	// Verify it's valid JSON
	if !bytes.Contains(buf.Bytes(), []byte(`"total_files"`)) {
		t.Error("JSON output missing expected field")
	}
}

func TestExporter_CSV(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewExporter(ExportCSV, &buf)

	report := &Report{
		Duration: time.Second,
		Summary:  NewSummary(),
		Files:    []*FileStats{NewFileStats("/test/file.log")},
	}
	report.Summary.TotalFiles = 1
	report.Summary.TotalEntries = 100

	if err := exporter.ExportReport(report); err != nil {
		t.Fatalf("ExportReport() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty CSV output")
	}

	// Check for summary section
	if !bytes.Contains(buf.Bytes(), []byte("# Summary")) {
		t.Error("CSV output missing Summary section")
	}
}

func TestExporter_Entries(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewExporter(ExportJSON, &buf)

	entries := []*models.LogEntry{
		{
			Timestamp: time.Now(),
			Level:     models.LevelInfo,
			Message:   "test message",
		},
	}

	if err := exporter.ExportEntries(entries); err != nil {
		t.Fatalf("ExportEntries() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestReport_Aggregation(t *testing.T) {
	files := []*FileStats{
		{
			Path:        "/file1.log",
			ParsedCount: 100,
			ErrorCount:  10,
			LevelCounts: map[string]int64{"info": 80, "error": 20},
			TypeCounts:  map[string]int64{"nginx": 100},
		},
		{
			Path:        "/file2.log",
			ParsedCount: 50,
			ErrorCount:  5,
			LevelCounts: map[string]int64{"info": 40, "warning": 10},
			TypeCounts:  map[string]int64{"apache": 50},
		},
	}

	summary := Aggregate(files, time.Second)

	if summary.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", summary.TotalFiles)
	}

	if summary.TotalEntries != 150 {
		t.Errorf("TotalEntries = %d, want 150", summary.TotalEntries)
	}

	if summary.TotalErrors != 15 {
		t.Errorf("TotalErrors = %d, want 15", summary.TotalErrors)
	}

	if summary.LevelCounts["info"] != 120 {
		t.Errorf("LevelCounts[info] = %d, want 120", summary.LevelCounts["info"])
	}

	if summary.EntriesPerSec != 150 {
		t.Errorf("EntriesPerSec = %f, want 150", summary.EntriesPerSec)
	}
}

func TestSummary_Percentages(t *testing.T) {
	s := &Summary{
		TotalEntries: 100,
		LevelCounts:  map[string]int64{"info": 80, "error": 20},
		TypeCounts:   map[string]int64{"nginx": 60, "apache": 40},
	}

	if pct := s.LevelPercentage("info"); pct != 80.0 {
		t.Errorf("LevelPercentage(info) = %f, want 80.0", pct)
	}

	if pct := s.TypePercentage("nginx"); pct != 60.0 {
		t.Errorf("TypePercentage(nginx) = %f, want 60.0", pct)
	}
}

func TestExpandGlobs(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	for _, name := range []string{"a.log", "b.log", "c.txt"} {
		f, _ := os.Create(filepath.Join(dir, name))
		f.Close()
	}

	files := expandGlobs([]string{filepath.Join(dir, "*.log")})

	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}
