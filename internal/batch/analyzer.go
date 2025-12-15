package batch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/parser"
)

// AnalyzerOptions configures batch analysis.
type AnalyzerOptions struct {
	Workers    int       // Number of parallel workers (0 = auto)
	BufferSize int       // Channel buffer size (0 = auto)
	From       time.Time // Filter: entries on or after this time
	To         time.Time // Filter: entries on or before this time
	ParserType string    // Parser type ("nginx", "apache", etc) or "auto"
	Verbose    bool      // Verbose progress output
	Limit      int       // Limit entries per file (0 = no limit)
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() *AnalyzerOptions {
	return &AnalyzerOptions{
		Workers:    runtime.NumCPU(),
		BufferSize: 0, // Will be set based on workers
		ParserType: "auto",
		Verbose:    false,
		Limit:      0,
	}
}

// Analyzer performs batch analysis on log files.
type Analyzer struct {
	opts   *AnalyzerOptions
	filter *DateFilter
}

// NewAnalyzer creates a new batch analyzer.
func NewAnalyzer(opts *AnalyzerOptions) *Analyzer {
	if opts == nil {
		opts = DefaultOptions()
	}
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU()
	}

	return &Analyzer{
		opts:   opts,
		filter: NewDateFilter(opts.From, opts.To),
	}
}

// Analyze processes files matching the given patterns and returns aggregated stats.
func (a *Analyzer) Analyze(ctx context.Context, patterns []string) (*Report, error) {
	startTime := time.Now()

	// Expand glob patterns to file list
	files := expandGlobs(patterns)
	if len(files) == 0 {
		return nil, fmt.Errorf("no files match the specified patterns")
	}

	// Create worker pool
	pool := NewWorkerPool(a.opts.Workers, a.opts.BufferSize)

	// Start workers with our file processor
	pool.Start(ctx, a.analyzeFile)

	// Submit all files
	go func() {
		for _, f := range files {
			if err := pool.Submit(ctx, f); err != nil {
				break
			}
		}
		pool.Close()
	}()

	// Collect results
	var results []*FileStats
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Collect results from pool
	wg.Add(1)
	go func() {
		defer wg.Done()
		for stats := range pool.Results() {
			mu.Lock()
			results = append(results, stats)
			mu.Unlock()
		}
	}()

	// Collect errors from pool
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range pool.Errors() {
			mu.Lock()
			errors = append(errors, err.Error())
			mu.Unlock()
		}
	}()

	wg.Wait()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Build report
	report := &Report{
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Files:     results,
		Summary:   Aggregate(results, duration),
		Errors:    errors,
	}

	// Add date range info
	if a.filter.Enabled {
		report.DateRange = &DateRange{
			Filtered: true,
			From:     a.opts.From,
			To:       a.opts.To,
		}
	}

	// Calculate actual date range from entries
	for _, f := range results {
		if report.DateRange == nil {
			report.DateRange = &DateRange{}
		}
		if !f.FirstEntry.IsZero() {
			if report.DateRange.Earliest.IsZero() || f.FirstEntry.Before(report.DateRange.Earliest) {
				report.DateRange.Earliest = f.FirstEntry
			}
		}
		if !f.LastEntry.IsZero() {
			if report.DateRange.Latest.IsZero() || f.LastEntry.After(report.DateRange.Latest) {
				report.DateRange.Latest = f.LastEntry
			}
		}
	}

	return report, nil
}

// analyzeFile processes a single file and returns stats.
func (a *Analyzer) analyzeFile(ctx context.Context, path string) (*FileStats, error) {
	parseStart := time.Now()
	stats := NewFileStats(path)

	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	// Get file size
	if fi, err := file.Stat(); err == nil {
		stats.BytesRead = fi.Size()
	}

	// Get parser
	p, err := a.getParser(file, path)
	if err != nil {
		return nil, err
	}

	// Reset file after auto-detection
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek %s: %w", path, err)
	}

	// Check if this is a multiline parser
	multiParser, isMultiLine := p.(parser.MultiLineParser)

	scanner := bufio.NewScanner(file)
	// Increase buffer for potentially long lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	if isMultiLine {
		stats.ParseErrors, stats.ParsedCount, stats.ErrorCount = a.parseMultiLine(ctx, scanner, multiParser, stats, path)
	} else {
		stats.ParseErrors, stats.ParsedCount, stats.ErrorCount = a.parseSingleLine(ctx, scanner, p, stats, path)
	}

	stats.ParseTime = time.Since(parseStart)
	return stats, scanner.Err()
}

func (a *Analyzer) parseSingleLine(ctx context.Context, scanner *bufio.Scanner, p parser.Parser, stats *FileStats, path string) (parseErrors, parsed, errors int64) {
	lineNum := int64(0)
	count := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry, err := p.Parse(line)
		if err != nil {
			parseErrors++
			continue
		}

		entry.LineNumber = lineNum
		entry.FilePath = path

		// Apply date filter
		if !a.filter.Matches(entry) {
			continue
		}

		// Update stats
		parsed++
		stats.LevelCounts[string(entry.Level)]++
		stats.TypeCounts[string(entry.Type)]++

		if entry.Level == models.LevelError || entry.Level == models.LevelFatal {
			errors++
		}

		// Track time range
		if !entry.Timestamp.IsZero() {
			if stats.FirstEntry.IsZero() || entry.Timestamp.Before(stats.FirstEntry) {
				stats.FirstEntry = entry.Timestamp
			}
			if stats.LastEntry.IsZero() || entry.Timestamp.After(stats.LastEntry) {
				stats.LastEntry = entry.Timestamp
			}
		}

		count++
		if a.opts.Limit > 0 && count >= a.opts.Limit {
			break
		}
	}

	return
}

func (a *Analyzer) parseMultiLine(ctx context.Context, scanner *bufio.Scanner, p parser.MultiLineParser, stats *FileStats, path string) (parseErrors, parsed, errors int64) {
	var currentLines []string
	var startLineNum int64
	lineNum := int64(0)
	count := 0

	processEntry := func() {
		if len(currentLines) == 0 {
			return
		}

		entry, err := p.ParseMultiLine(currentLines)
		if err != nil {
			parseErrors++
			return
		}

		entry.LineNumber = startLineNum
		entry.FilePath = path

		// Apply date filter
		if !a.filter.Matches(entry) {
			return
		}

		// Update stats
		parsed++
		stats.LevelCounts[string(entry.Level)]++
		stats.TypeCounts[string(entry.Type)]++

		if entry.Level == models.LevelError || entry.Level == models.LevelFatal {
			errors++
		}

		// Track time range
		if !entry.Timestamp.IsZero() {
			if stats.FirstEntry.IsZero() || entry.Timestamp.Before(stats.FirstEntry) {
				stats.FirstEntry = entry.Timestamp
			}
			if stats.LastEntry.IsZero() || entry.Timestamp.After(stats.LastEntry) {
				stats.LastEntry = entry.Timestamp
			}
		}

		count++
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		if p.IsStartOfEntry(line) {
			// Process previous entry
			processEntry()
			if a.opts.Limit > 0 && count >= a.opts.Limit {
				break
			}

			// Start new entry
			currentLines = []string{line}
			startLineNum = lineNum
		} else if len(currentLines) > 0 {
			currentLines = append(currentLines, line)
		}
	}

	// Process last entry
	if a.opts.Limit == 0 || count < a.opts.Limit {
		processEntry()
	}

	return
}

func (a *Analyzer) getParser(file *os.File, path string) (parser.Parser, error) {
	if a.opts.ParserType != "" && a.opts.ParserType != "auto" {
		p, ok := GetParser(a.opts.ParserType)
		if !ok {
			return nil, fmt.Errorf("unknown parser type: %s", a.opts.ParserType)
		}
		return p, nil
	}

	// Auto-detect from first line
	scanner := bufio.NewScanner(file)
	var firstLine string
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			firstLine = line
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if firstLine == "" {
		return nil, fmt.Errorf("file is empty: %s", path)
	}

	p, ok := parser.AutoDetect(firstLine)
	if !ok {
		return nil, fmt.Errorf("could not auto-detect format for: %s", path)
	}

	return p, nil
}

// GetParser returns a parser for the given type.
func GetParser(logType string) (parser.Parser, bool) {
	switch logType {
	case "nginx", "nginx-access":
		return parser.NewNginxAccessParser(nil), true
	case "nginx-error":
		return parser.NewNginxErrorParser(nil), true
	case "apache", "apache-access":
		return parser.NewApacheAccessParser(nil), true
	case "apache-error":
		return parser.NewApacheErrorParser(nil), true
	case "magento":
		return parser.NewMagentoParser(nil), true
	case "prestashop":
		return parser.NewPrestaShopParser(nil), true
	case "wordpress":
		return parser.NewWordPressParser(nil), true
	default:
		return nil, false
	}
}

// expandGlobs expands glob patterns to absolute file paths.
func expandGlobs(patterns []string) []string {
	var files []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			// Skip directories
			fi, err := os.Stat(match)
			if err != nil || fi.IsDir() {
				continue
			}

			absPath, err := filepath.Abs(match)
			if err != nil {
				continue
			}

			if !seen[absPath] {
				seen[absPath] = true
				files = append(files, absPath)
			}
		}
	}

	return files
}
