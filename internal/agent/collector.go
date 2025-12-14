package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/parser"
	"github.com/good-yellow-bee/blazelog/internal/tailer"
)

// SourceConfig defines a log source to collect.
type SourceConfig struct {
	Name   string
	Type   string
	Path   string
	Follow bool
}

// Collector collects log entries from a single source.
type Collector struct {
	source     SourceConfig
	tailer     *tailer.Tailer
	parser     parser.Parser
	entries    chan *models.LogEntry
	labels     map[string]string
	lineNumber int64

	mu     sync.Mutex
	closed bool
}

// NewCollector creates a new collector for the given source.
func NewCollector(source SourceConfig, labels map[string]string) (*Collector, error) {
	// Find parser by type name
	p, ok := parser.DefaultRegistry.GetByName(source.Type)
	if !ok {
		// Try to find by log type
		logType := stringToLogType(source.Type)
		p, ok = parser.Get(logType)
		if !ok {
			return nil, fmt.Errorf("unknown parser type: %s", source.Type)
		}
	}

	// Create tailer
	opts := &tailer.TailerOptions{
		Follow:   source.Follow,
		ReOpen:   true,
		MustExist: true,
	}
	t, err := tailer.NewTailer(source.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("create tailer for %s: %w", source.Path, err)
	}

	return &Collector{
		source:  source,
		tailer:  t,
		parser:  p,
		entries: make(chan *models.LogEntry, 100),
		labels:  labels,
	}, nil
}

// Start begins collecting log entries.
func (c *Collector) Start(ctx context.Context) error {
	if err := c.tailer.Start(ctx); err != nil {
		return fmt.Errorf("start tailer: %w", err)
	}

	go c.collect(ctx)
	return nil
}

// collect reads lines from the tailer, parses them, and sends entries.
func (c *Collector) collect(ctx context.Context) {
	defer close(c.entries)

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-c.tailer.Lines():
			if !ok {
				return
			}
			if line.Err != nil {
				continue
			}
			if line.Text == "" {
				continue
			}

			entry, err := c.parser.Parse(line.Text)
			if err != nil {
				// Log parsing error but continue
				continue
			}

			// Enrich entry with source info
			atomic.AddInt64(&c.lineNumber, 1)
			entry.Source = c.source.Name
			entry.FilePath = line.FilePath
			entry.LineNumber = atomic.LoadInt64(&c.lineNumber)
			entry.Raw = line.Text

			// Add labels
			if entry.Labels == nil {
				entry.Labels = make(map[string]string)
			}
			for k, v := range c.labels {
				entry.Labels[k] = v
			}
			entry.Labels["source"] = c.source.Name

			select {
			case c.entries <- entry:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Entries returns the channel for reading parsed log entries.
func (c *Collector) Entries() <-chan *models.LogEntry {
	return c.entries
}

// Stop stops the collector.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	c.tailer.Stop()
}

// Source returns the source configuration.
func (c *Collector) Source() SourceConfig {
	return c.source
}

// stringToLogType converts a string type name to models.LogType.
func stringToLogType(s string) models.LogType {
	switch s {
	case "nginx", "nginx-access", "nginx-error":
		return models.LogTypeNginx
	case "apache", "apache-access", "apache-error":
		return models.LogTypeApache
	case "magento":
		return models.LogTypeMagento
	case "prestashop":
		return models.LogTypePrestaShop
	case "wordpress":
		return models.LogTypeWordPress
	default:
		return models.LogTypeUnknown
	}
}
