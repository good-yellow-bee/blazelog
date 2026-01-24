// Package tailer provides file tailing functionality with support for
// log rotation and glob pattern matching.
package tailer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Line represents a single line read from a file.
type Line struct {
	Text     string    // The line content
	FilePath string    // The source file path
	Time     time.Time // When the line was read
	Err      error     // Any error that occurred
}

// Options contains options for configuring a Tailer.
type Options struct {
	// Follow indicates whether to continue watching for new lines.
	Follow bool
	// PollInterval is the interval to poll for changes when fsnotify fails.
	PollInterval time.Duration
	// ReOpen indicates whether to reopen the file if it's rotated.
	ReOpen bool
	// MustExist indicates whether the file must exist at startup.
	MustExist bool
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		Follow:       true,
		PollInterval: 250 * time.Millisecond,
		ReOpen:       true,
		MustExist:    true,
	}
}

// Tailer watches a file and emits new lines as they're written.
type Tailer struct {
	filePath string
	opts     *Options
	watcher  *fsnotify.Watcher

	file   *os.File
	reader *bufio.Reader
	offset int64
	size   int64

	lines chan Line
	done  chan struct{}

	mu     sync.Mutex
	closed bool
}

// NewTailer creates a new Tailer for the given file path.
func NewTailer(filePath string, opts *Options) (*Tailer, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) && opts.MustExist {
			return nil, fmt.Errorf("file does not exist: %s", absPath)
		}
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	t := &Tailer{
		filePath: absPath,
		opts:     opts,
		watcher:  watcher,
		lines:    make(chan Line, 100),
		done:     make(chan struct{}),
	}

	// Open file for reading
	if info != nil {
		if err := t.openFile(); err != nil {
			watcher.Close()
			return nil, err
		}
		t.size = info.Size()
	}

	return t, nil
}

// Lines returns a channel that emits lines as they're read.
func (t *Tailer) Lines() <-chan Line {
	return t.lines
}

// Start begins tailing the file. It reads existing content from the current
// position and then watches for new content.
func (t *Tailer) Start(ctx context.Context) error {
	// Add directory to watcher for rotation detection
	dir := filepath.Dir(t.filePath)
	if err := t.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Start the tail goroutine
	go t.run(ctx)

	return nil
}

// StartFromEnd begins tailing from the end of the file (skipping existing content).
func (t *Tailer) StartFromEnd(ctx context.Context) error {
	// Seek to end of file
	if t.file != nil {
		offset, err := t.file.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("failed to seek to end: %w", err)
		}
		t.offset = offset
		t.reader = bufio.NewReader(t.file)
	}

	return t.Start(ctx)
}

// Stop stops the tailer.
func (t *Tailer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	t.closed = true

	close(t.done)
	t.watcher.Close()
	if t.file != nil {
		t.file.Close()
	}
}

func (t *Tailer) openFile() error {
	file, err := os.Open(t.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	t.file = file
	t.reader = bufio.NewReader(file)
	t.offset = 0
	return nil
}

func (t *Tailer) run(ctx context.Context) {
	defer close(t.lines)

	// Read any existing content first
	t.readLines()

	if !t.opts.Follow {
		return
	}

	// Set up a ticker for polling as a fallback
	ticker := time.NewTicker(t.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			t.handleEvent(event)
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.sendLine(Line{Err: fmt.Errorf("watcher error: %w", err)})
		case <-ticker.C:
			// Fallback polling for systems where fsnotify doesn't work well
			t.checkForChanges()
		}
	}
}

func (t *Tailer) handleEvent(event fsnotify.Event) {
	// Only process events for our file
	if event.Name != t.filePath {
		return
	}

	if event.Has(fsnotify.Write) {
		t.readLines()
	} else if event.Has(fsnotify.Create) {
		// File was recreated (rotation)
		if t.opts.ReOpen {
			t.handleRotation()
		}
	}
	// Ignore Remove, Rename, Chmod and other events - wait for create event on rotation
}

func (t *Tailer) checkForChanges() {
	info, err := os.Stat(t.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File might have been rotated, wait for it to reappear
			return
		}
		return
	}

	newSize := info.Size()

	// Check for file truncation (log rotation with copytruncate)
	if newSize < t.size {
		t.handleTruncation()
		return
	}

	// Check for new content
	if newSize > t.size {
		t.size = newSize
		t.readLines()
	}
}

func (t *Tailer) handleRotation() {
	// Close old file
	if t.file != nil {
		t.file.Close()
		t.file = nil
	}

	// Try to open the new file with retries
	for i := 0; i < 10; i++ {
		err := t.openFile()
		if err == nil {
			info, _ := os.Stat(t.filePath)
			if info != nil {
				t.size = info.Size()
			}
			t.readLines()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (t *Tailer) handleTruncation() {
	// File was truncated, seek to beginning
	if t.file != nil {
		t.file.Seek(0, io.SeekStart)
		t.reader = bufio.NewReader(t.file)
		t.offset = 0
		t.size = 0
		t.readLines()
	}
}

func (t *Tailer) readLines() {
	if t.file == nil || t.reader == nil {
		return
	}

	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// No more data available right now
				// Update offset for partial line handling
				if len(line) > 0 {
					// Partial line, we'll read it next time
					// Seek back to re-read it
					seekBack := int64(len(line))
					t.file.Seek(-seekBack, io.SeekCurrent)
					t.reader = bufio.NewReader(t.file)
				}
				return
			}
			t.sendLine(Line{Err: fmt.Errorf("read error: %w", err)})
			return
		}

		// Remove trailing newline
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		// Remove trailing carriage return (Windows)
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		t.sendLine(Line{
			Text:     line,
			FilePath: t.filePath,
			Time:     time.Now(),
		})
	}
}

func (t *Tailer) sendLine(line Line) {
	select {
	case t.lines <- line:
	case <-t.done:
	}
}
