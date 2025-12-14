package tailer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// MultiTailer watches multiple files matching glob patterns and emits lines
// from all of them.
type MultiTailer struct {
	patterns []string
	opts     *TailerOptions

	tailers map[string]*Tailer
	watcher *fsnotify.Watcher

	lines chan Line
	done  chan struct{}

	mu     sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

// NewMultiTailer creates a new MultiTailer for the given patterns.
// Patterns can be file paths or glob patterns (e.g., "/var/log/*.log").
func NewMultiTailer(patterns []string, opts *TailerOptions) (*MultiTailer, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	mt := &MultiTailer{
		patterns: patterns,
		opts:     opts,
		tailers:  make(map[string]*Tailer),
		watcher:  watcher,
		lines:    make(chan Line, 100),
		done:     make(chan struct{}),
	}

	return mt, nil
}

// Lines returns a channel that emits lines from all watched files.
func (mt *MultiTailer) Lines() <-chan Line {
	return mt.lines
}

// Start begins tailing all files matching the patterns.
// When Follow is false, reads existing content from the beginning.
// When Follow is true, starts from the end and follows new content.
func (mt *MultiTailer) Start(ctx context.Context) error {
	// Expand patterns and create tailers
	if err := mt.expandPatterns(); err != nil {
		return err
	}

	// Watch directories for new files (only if following)
	if mt.opts.Follow {
		if err := mt.watchDirectories(); err != nil {
			return err
		}
		go mt.watchForNewFiles(ctx)
	}

	// Start all tailers
	for _, t := range mt.tailers {
		var err error
		if mt.opts.Follow {
			err = t.StartFromEnd(ctx)
		} else {
			err = t.Start(ctx)
		}
		if err != nil {
			return err
		}
		mt.wg.Add(1)
		go mt.forwardLines(t)
	}

	return nil
}

// Stop stops all tailers and cleans up resources.
func (mt *MultiTailer) Stop() {
	mt.mu.Lock()
	if mt.closed {
		mt.mu.Unlock()
		return
	}
	mt.closed = true
	mt.mu.Unlock()

	close(mt.done)
	mt.watcher.Close()

	for _, t := range mt.tailers {
		t.Stop()
	}

	// Wait for all forwardLines goroutines to exit before closing the channel
	mt.wg.Wait()
	close(mt.lines)
}

// Files returns the list of files currently being tailed.
func (mt *MultiTailer) Files() []string {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	files := make([]string, 0, len(mt.tailers))
	for path := range mt.tailers {
		files = append(files, path)
	}
	return files
}

func (mt *MultiTailer) expandPatterns() error {
	for _, pattern := range mt.patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}

		if len(matches) == 0 && mt.opts.MustExist {
			return fmt.Errorf("no files match pattern: %s", pattern)
		}

		for _, match := range matches {
			if err := mt.addTailer(match); err != nil {
				return err
			}
		}
	}

	return nil
}

func (mt *MultiTailer) addTailer(filePath string) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Check if we're already tailing this file
	if _, exists := mt.tailers[filePath]; exists {
		return nil
	}

	tailer, err := NewTailer(filePath, mt.opts)
	if err != nil {
		return err
	}

	mt.tailers[filePath] = tailer
	return nil
}

func (mt *MultiTailer) watchDirectories() error {
	dirs := make(map[string]bool)

	for _, pattern := range mt.patterns {
		dir := filepath.Dir(pattern)
		if !dirs[dir] {
			dirs[dir] = true
			if err := mt.watcher.Add(dir); err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", dir, err)
			}
		}
	}

	return nil
}

func (mt *MultiTailer) watchForNewFiles(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-mt.done:
			return
		case event, ok := <-mt.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				mt.handleNewFile(ctx, event.Name)
			}
		case <-mt.watcher.Errors:
			// Ignore errors from the directory watcher
		}
	}
}

func (mt *MultiTailer) handleNewFile(ctx context.Context, filePath string) {
	// Check if this file matches any of our patterns
	for _, pattern := range mt.patterns {
		matched, err := filepath.Match(pattern, filePath)
		if err != nil {
			continue
		}

		if matched {
			if err := mt.addTailer(filePath); err != nil {
				continue
			}

			mt.mu.Lock()
			tailer := mt.tailers[filePath]
			mt.mu.Unlock()

			if tailer != nil {
				if err := tailer.StartFromEnd(ctx); err != nil {
					continue
				}
				mt.wg.Add(1)
				go mt.forwardLines(tailer)
			}
			break
		}
	}
}

func (mt *MultiTailer) forwardLines(t *Tailer) {
	defer mt.wg.Done()
	for line := range t.Lines() {
		select {
		case mt.lines <- line:
		case <-mt.done:
			return
		}
	}
}
