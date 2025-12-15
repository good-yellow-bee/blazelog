package batch

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// ExportFormat defines the output format for exports.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

// ParseExportFormat parses a string to ExportFormat.
func ParseExportFormat(s string) (ExportFormat, bool) {
	switch s {
	case "json":
		return ExportJSON, true
	case "csv":
		return ExportCSV, true
	default:
		return "", false
	}
}

// Exporter handles report and entries export to various formats.
type Exporter struct {
	format ExportFormat
	writer io.Writer
}

// NewExporter creates an exporter for the given format.
func NewExporter(format ExportFormat, w io.Writer) *Exporter {
	return &Exporter{
		format: format,
		writer: w,
	}
}

// ExportReport writes the analysis report in the configured format.
func (e *Exporter) ExportReport(report *Report) error {
	switch e.format {
	case ExportCSV:
		return e.exportReportCSV(report)
	default:
		return e.exportReportJSON(report)
	}
}

func (e *Exporter) exportReportJSON(report *Report) error {
	encoder := json.NewEncoder(e.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func (e *Exporter) exportReportCSV(report *Report) error {
	w := csv.NewWriter(e.writer)
	defer w.Flush()

	// Summary header
	w.Write([]string{"# Summary"})
	w.Write([]string{"total_files", strconv.Itoa(report.Summary.TotalFiles)})
	w.Write([]string{"total_entries", strconv.FormatInt(report.Summary.TotalEntries, 10)})
	w.Write([]string{"total_errors", strconv.FormatInt(report.Summary.TotalErrors, 10)})
	w.Write([]string{"parse_errors", strconv.FormatInt(report.Summary.ParseErrors, 10)})
	w.Write([]string{"entries_per_sec", strconv.FormatFloat(report.Summary.EntriesPerSec, 'f', 2, 64)})
	w.Write([]string{"duration_ms", strconv.FormatInt(report.Duration.Milliseconds(), 10)})
	w.Write([]string{})

	// Level counts
	w.Write([]string{"# Level Counts"})
	w.Write([]string{"level", "count"})
	for level, count := range report.Summary.LevelCounts {
		w.Write([]string{level, strconv.FormatInt(count, 10)})
	}
	w.Write([]string{})

	// Type counts
	w.Write([]string{"# Type Counts"})
	w.Write([]string{"type", "count"})
	for typ, count := range report.Summary.TypeCounts {
		w.Write([]string{typ, strconv.FormatInt(count, 10)})
	}
	w.Write([]string{})

	// File details
	w.Write([]string{"# File Details"})
	w.Write([]string{"path", "parsed", "errors", "parse_errors", "parse_time_ms"})
	for _, f := range report.Files {
		w.Write([]string{
			f.Path,
			strconv.FormatInt(f.ParsedCount, 10),
			strconv.FormatInt(f.ErrorCount, 10),
			strconv.FormatInt(f.ParseErrors, 10),
			strconv.FormatInt(f.ParseTime.Milliseconds(), 10),
		})
	}

	return w.Error()
}

// ExportEntries writes log entries in the configured format.
func (e *Exporter) ExportEntries(entries []*models.LogEntry) error {
	switch e.format {
	case ExportCSV:
		return e.exportEntriesCSV(entries)
	default:
		return e.exportEntriesJSON(entries)
	}
}

func (e *Exporter) exportEntriesJSON(entries []*models.LogEntry) error {
	encoder := json.NewEncoder(e.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func (e *Exporter) exportEntriesCSV(entries []*models.LogEntry) error {
	w := csv.NewWriter(e.writer)
	defer w.Flush()

	// Header
	w.Write([]string{"timestamp", "level", "type", "source", "message", "file_path", "line_number"})

	// Rows
	for _, entry := range entries {
		w.Write([]string{
			entry.Timestamp.Format(time.RFC3339),
			string(entry.Level),
			string(entry.Type),
			entry.Source,
			entry.Message,
			entry.FilePath,
			strconv.FormatInt(entry.LineNumber, 10),
		})
	}

	return w.Error()
}

// ExportEntriesStream writes log entries from a channel.
func (e *Exporter) ExportEntriesStream(entries <-chan *models.LogEntry) error {
	switch e.format {
	case ExportCSV:
		return e.exportEntriesStreamCSV(entries)
	default:
		return e.exportEntriesStreamJSON(entries)
	}
}

func (e *Exporter) exportEntriesStreamJSON(entries <-chan *models.LogEntry) error {
	encoder := json.NewEncoder(e.writer)
	e.writer.Write([]byte("[\n"))
	first := true
	for entry := range entries {
		if !first {
			e.writer.Write([]byte(",\n"))
		}
		first = false
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	e.writer.Write([]byte("\n]\n"))
	return nil
}

func (e *Exporter) exportEntriesStreamCSV(entries <-chan *models.LogEntry) error {
	w := csv.NewWriter(e.writer)
	defer w.Flush()

	// Header
	w.Write([]string{"timestamp", "level", "type", "source", "message", "file_path", "line_number"})

	for entry := range entries {
		w.Write([]string{
			entry.Timestamp.Format(time.RFC3339),
			string(entry.Level),
			string(entry.Type),
			entry.Source,
			entry.Message,
			entry.FilePath,
			strconv.FormatInt(entry.LineNumber, 10),
		})
	}

	return w.Error()
}
