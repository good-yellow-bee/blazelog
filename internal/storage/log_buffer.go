package storage

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// LogBuffer buffers log entries for batch insertion.
// It flushes on either batch size threshold or time interval,
// whichever comes first. It implements backpressure by dropping
// oldest entries when the buffer reaches max capacity.
type LogBuffer struct {
	repo          LogRepository
	batchSize     int
	flushInterval time.Duration
	maxSize       int

	mu       sync.Mutex
	buffer   []*LogRecord
	timer    *time.Timer
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopped  atomic.Bool
	dropped  atomic.Int64
	flushed  atomic.Int64
	inserted atomic.Int64
}

// LogBufferConfig holds LogBuffer configuration.
type LogBufferConfig struct {
	// BatchSize is the number of entries to trigger a flush.
	BatchSize int

	// FlushInterval is the time interval to trigger a flush.
	FlushInterval time.Duration

	// MaxSize is the maximum buffer size. When reached, oldest entries are dropped.
	MaxSize int
}

// NewLogBuffer creates a new log buffer.
func NewLogBuffer(repo LogRepository, config *LogBufferConfig) *LogBuffer {
	// Apply defaults
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100000
	}

	b := &LogBuffer{
		repo:          repo,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		maxSize:       config.MaxSize,
		buffer:        make([]*LogRecord, 0, config.BatchSize),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go b.flushLoop()
	return b
}

// Add adds a log entry to the buffer.
func (b *LogBuffer) Add(entry *LogRecord) error {
	return b.AddBatch([]*LogRecord{entry})
}

// AddBatch adds multiple log entries to the buffer.
func (b *LogBuffer) AddBatch(entries []*LogRecord) error {
	if b.stopped.Load() {
		return nil
	}

	b.mu.Lock()

	// Check if we need to drop old entries (backpressure)
	newLen := len(b.buffer) + len(entries)
	if newLen > b.maxSize {
		// Calculate how many to drop
		toDrop := newLen - b.maxSize
		if toDrop >= len(b.buffer) {
			// Drop all existing + some new (extreme case)
			b.dropped.Add(int64(len(b.buffer)))
			b.buffer = b.buffer[:0]
			// Only keep entries that fit
			keep := b.maxSize
			if keep > len(entries) {
				keep = len(entries)
			}
			drop := len(entries) - keep
			b.dropped.Add(int64(drop))
			entries = entries[drop:]
			log.Printf("warning: log buffer overflow, dropped %d entries", toDrop)
		} else {
			// Drop oldest from existing buffer
			b.dropped.Add(int64(toDrop))
			b.buffer = b.buffer[toDrop:]
			log.Printf("warning: log buffer overflow, dropped %d oldest entries", toDrop)
		}
	}

	b.buffer = append(b.buffer, entries...)
	shouldFlush := len(b.buffer) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		return b.Flush()
	}
	return nil
}

// Flush forces a flush of the current buffer.
func (b *LogBuffer) Flush() error {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return nil
	}

	toFlush := b.buffer
	b.buffer = make([]*LogRecord, 0, b.batchSize)
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := b.repo.InsertBatch(ctx, toFlush); err != nil {
		// Put entries back on error (at front so they're flushed next)
		b.mu.Lock()
		b.buffer = append(toFlush, b.buffer...)
		// Apply max size limit again
		if len(b.buffer) > b.maxSize {
			excess := len(b.buffer) - b.maxSize
			b.dropped.Add(int64(excess))
			b.buffer = b.buffer[excess:]
		}
		b.mu.Unlock()
		return err
	}

	b.flushed.Add(1)
	b.inserted.Add(int64(len(toFlush)))
	return nil
}

// flushLoop periodically flushes the buffer.
func (b *LogBuffer) flushLoop() {
	defer close(b.doneCh)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.Flush(); err != nil {
				log.Printf("log buffer flush error: %v", err)
			}
		case <-b.stopCh:
			// Final flush on shutdown
			if err := b.Flush(); err != nil {
				log.Printf("log buffer final flush error: %v", err)
			}
			return
		}
	}
}

// Close stops the buffer and flushes remaining entries.
func (b *LogBuffer) Close() error {
	if b.stopped.Swap(true) {
		return nil // Already stopped
	}
	close(b.stopCh)
	<-b.doneCh
	return nil
}

// Stats returns buffer statistics.
func (b *LogBuffer) Stats() LogBufferStats {
	b.mu.Lock()
	pending := len(b.buffer)
	b.mu.Unlock()

	return LogBufferStats{
		Pending:  pending,
		Dropped:  b.dropped.Load(),
		Flushed:  b.flushed.Load(),
		Inserted: b.inserted.Load(),
	}
}

// LogBufferStats contains buffer statistics.
type LogBufferStats struct {
	// Pending is the number of entries waiting to be flushed.
	Pending int

	// Dropped is the total number of entries dropped due to backpressure.
	Dropped int64

	// Flushed is the total number of flush operations.
	Flushed int64

	// Inserted is the total number of entries successfully inserted.
	Inserted int64
}
