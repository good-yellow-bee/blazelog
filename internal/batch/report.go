package batch

import (
	"time"
)

// Report contains complete batch analysis results.
type Report struct {
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Duration  time.Duration  `json:"duration_ms"`
	Files     []*FileStats   `json:"files"`
	Summary   *Summary       `json:"summary"`
	DateRange *DateRange     `json:"date_range,omitempty"`
	Errors    []string       `json:"errors,omitempty"`
}

// FileStats contains per-file statistics.
type FileStats struct {
	Path        string           `json:"path"`
	ParsedCount int64            `json:"parsed_count"`
	ErrorCount  int64            `json:"error_count"`
	ParseErrors int64            `json:"parse_errors"`
	LevelCounts map[string]int64 `json:"level_counts"`
	TypeCounts  map[string]int64 `json:"type_counts"`
	FirstEntry  time.Time        `json:"first_entry,omitempty"`
	LastEntry   time.Time        `json:"last_entry,omitempty"`
	BytesRead   int64            `json:"bytes_read"`
	ParseTime   time.Duration    `json:"parse_time_ms"`
}

// NewFileStats creates a new FileStats with initialized maps.
func NewFileStats(path string) *FileStats {
	return &FileStats{
		Path:        path,
		LevelCounts: make(map[string]int64),
		TypeCounts:  make(map[string]int64),
	}
}

// Summary aggregates statistics across all files.
type Summary struct {
	TotalFiles    int              `json:"total_files"`
	TotalEntries  int64            `json:"total_entries"`
	TotalErrors   int64            `json:"total_errors"`
	ParseErrors   int64            `json:"parse_errors"`
	LevelCounts   map[string]int64 `json:"level_counts"`
	TypeCounts    map[string]int64 `json:"type_counts"`
	SourceCounts  map[string]int64 `json:"source_counts"`
	EntriesPerSec float64          `json:"entries_per_sec"`
}

// NewSummary creates a new Summary with initialized maps.
func NewSummary() *Summary {
	return &Summary{
		LevelCounts:  make(map[string]int64),
		TypeCounts:   make(map[string]int64),
		SourceCounts: make(map[string]int64),
	}
}

// DateRange tracks the actual date range of analyzed entries.
type DateRange struct {
	Earliest time.Time `json:"earliest,omitempty"`
	Latest   time.Time `json:"latest,omitempty"`
	Filtered bool      `json:"filtered"`
	From     time.Time `json:"from,omitempty"`
	To       time.Time `json:"to,omitempty"`
}

// Aggregate combines multiple FileStats into a Summary.
func Aggregate(files []*FileStats, duration time.Duration) *Summary {
	s := NewSummary()
	s.TotalFiles = len(files)

	for _, f := range files {
		s.TotalEntries += f.ParsedCount
		s.TotalErrors += f.ErrorCount
		s.ParseErrors += f.ParseErrors
		s.SourceCounts[f.Path] = f.ParsedCount

		for level, count := range f.LevelCounts {
			s.LevelCounts[level] += count
		}

		for typ, count := range f.TypeCounts {
			s.TypeCounts[typ] += count
		}
	}

	if duration.Seconds() > 0 {
		s.EntriesPerSec = float64(s.TotalEntries) / duration.Seconds()
	}

	return s
}

// LevelPercentage returns the percentage for a given level.
func (s *Summary) LevelPercentage(level string) float64 {
	if s.TotalEntries == 0 {
		return 0
	}
	return float64(s.LevelCounts[level]) / float64(s.TotalEntries) * 100
}

// TypePercentage returns the percentage for a given type.
func (s *Summary) TypePercentage(typ string) float64 {
	if s.TotalEntries == 0 {
		return 0
	}
	return float64(s.TypeCounts[typ]) / float64(s.TotalEntries) * 100
}
