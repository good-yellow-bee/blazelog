package alerting

import (
	"sync"
	"time"
)

// SlidingWindow maintains a count of events within a sliding time window.
type SlidingWindow struct {
	mu       sync.RWMutex
	window   time.Duration
	events   []time.Time
	maxSize  int
}

// NewSlidingWindow creates a new sliding window with the given duration.
func NewSlidingWindow(window time.Duration) *SlidingWindow {
	// Set a reasonable max size to prevent unbounded growth
	// For a 1-hour window with high traffic, we might see 10k events
	maxSize := 100000

	return &SlidingWindow{
		window:  window,
		events:  make([]time.Time, 0, 1000),
		maxSize: maxSize,
	}
}

// Add adds an event at the current time.
func (w *SlidingWindow) Add() {
	w.AddAt(time.Now())
}

// AddAt adds an event at a specific time.
func (w *SlidingWindow) AddAt(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Prune old events first
	w.pruneOldLocked(t)

	// Add the new event
	w.events = append(w.events, t)

	// If we exceed max size, drop oldest events
	if len(w.events) > w.maxSize {
		// Keep only the most recent half
		w.events = w.events[len(w.events)/2:]
	}
}

// Count returns the number of events within the window.
func (w *SlidingWindow) Count() int {
	return w.CountAt(time.Now())
}

// CountAt returns the number of events within the window at a specific time.
func (w *SlidingWindow) CountAt(t time.Time) int {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pruneOldLocked(t)
	return len(w.events)
}

// pruneOldLocked removes events older than the window.
// Must be called with lock held.
func (w *SlidingWindow) pruneOldLocked(now time.Time) {
	cutoff := now.Add(-w.window)

	// Binary search for the first event after cutoff
	left, right := 0, len(w.events)
	for left < right {
		mid := (left + right) / 2
		if w.events[mid].Before(cutoff) {
			left = mid + 1
		} else {
			right = mid
		}
	}

	if left > 0 {
		// Remove old events
		w.events = w.events[left:]
	}
}

// Reset clears all events from the window.
func (w *SlidingWindow) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.events = w.events[:0]
}

// WindowDuration returns the configured window duration.
func (w *SlidingWindow) WindowDuration() time.Duration {
	return w.window
}

// WindowManager manages sliding windows for multiple rules.
type WindowManager struct {
	mu      sync.RWMutex
	windows map[string]*SlidingWindow
}

// NewWindowManager creates a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		windows: make(map[string]*SlidingWindow),
	}
}

// GetOrCreate returns an existing window or creates a new one.
func (wm *WindowManager) GetOrCreate(ruleName string, windowDuration time.Duration) *SlidingWindow {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if w, ok := wm.windows[ruleName]; ok {
		return w
	}

	w := NewSlidingWindow(windowDuration)
	wm.windows[ruleName] = w
	return w
}

// Get returns a window by rule name, or nil if not found.
func (wm *WindowManager) Get(ruleName string) *SlidingWindow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	return wm.windows[ruleName]
}

// Reset clears a specific window.
func (wm *WindowManager) Reset(ruleName string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if w, ok := wm.windows[ruleName]; ok {
		w.Reset()
	}
}

// ResetAll clears all windows.
func (wm *WindowManager) ResetAll() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, w := range wm.windows {
		w.Reset()
	}
}

// Count returns the event count for a rule.
func (wm *WindowManager) Count(ruleName string) int {
	w := wm.Get(ruleName)
	if w == nil {
		return 0
	}
	return w.Count()
}
