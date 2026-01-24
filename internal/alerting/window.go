package alerting

import (
	"sync"
	"time"
)

const (
	maxEventsPerRule = 10000
	maxTotalEvents   = 100000
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
	return &SlidingWindow{
		window:  window,
		events:  make([]time.Time, 0, 1000),
		maxSize: maxEventsPerRule,
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
	mu          sync.RWMutex
	windows     map[string]*SlidingWindow
	totalEvents int
}

// NewWindowManager creates a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		windows: make(map[string]*SlidingWindow),
	}
}

// TotalEvents returns the current total event count across all windows.
func (wm *WindowManager) TotalEvents() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.totalEvents
}

// evictIfNeeded evicts oldest events from windows when global limit exceeded.
// Must be called with write lock held.
func (wm *WindowManager) evictIfNeeded() {
	for wm.totalEvents > maxTotalEvents {
		var (
			oldestWindow *SlidingWindow
			oldestTime   time.Time
			foundAny     bool
		)
		for _, w := range wm.windows {
			w.mu.RLock()
			if len(w.events) > 0 {
				if !foundAny || w.events[0].Before(oldestTime) {
					oldestWindow = w
					oldestTime = w.events[0]
					foundAny = true
				}
			}
			w.mu.RUnlock()
		}
		if !foundAny || oldestWindow == nil {
			break
		}
		oldestWindow.mu.Lock()
		if len(oldestWindow.events) > 0 {
			evictCount := len(oldestWindow.events) / 2
			if evictCount < 1 {
				evictCount = 1
			}
			oldestWindow.events = oldestWindow.events[evictCount:]
			wm.totalEvents -= evictCount
		}
		oldestWindow.mu.Unlock()
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

// AddEvent adds an event to a rule's window, tracking global event count.
func (wm *WindowManager) AddEvent(ruleName string, windowDuration time.Duration) {
	wm.AddEventAt(ruleName, windowDuration, time.Now())
}

// AddEventAt adds an event at a specific time, tracking global event count.
func (wm *WindowManager) AddEventAt(ruleName string, windowDuration time.Duration, t time.Time) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	w, ok := wm.windows[ruleName]
	if !ok {
		w = NewSlidingWindow(windowDuration)
		wm.windows[ruleName] = w
	}

	w.mu.Lock()
	beforeCount := len(w.events)
	w.pruneOldLocked(t)
	w.events = append(w.events, t)
	if len(w.events) > w.maxSize {
		w.events = w.events[len(w.events)/2:]
	}
	afterCount := len(w.events)
	w.mu.Unlock()

	wm.totalEvents += afterCount - beforeCount
	wm.evictIfNeeded()
}

// Get returns a window by rule name, or nil if not found.
func (wm *WindowManager) Get(ruleName string) *SlidingWindow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	return wm.windows[ruleName]
}

// Reset clears a specific window's events without removing the window.
func (wm *WindowManager) Reset(ruleName string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if w, ok := wm.windows[ruleName]; ok {
		w.mu.Lock()
		wm.totalEvents -= len(w.events)
		w.events = w.events[:0]
		w.mu.Unlock()
	}
}

// Delete removes a window from the manager entirely.
// Use this when removing a rule to prevent memory leaks.
func (wm *WindowManager) Delete(ruleName string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if w, ok := wm.windows[ruleName]; ok {
		w.mu.RLock()
		wm.totalEvents -= len(w.events)
		w.mu.RUnlock()
		delete(wm.windows, ruleName)
	}
}

// ResetAll clears all windows' events without removing them.
func (wm *WindowManager) ResetAll() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, w := range wm.windows {
		w.mu.Lock()
		w.events = w.events[:0]
		w.mu.Unlock()
	}
	wm.totalEvents = 0
}

// DeleteAll removes all windows from the manager.
// Use this when reloading rules to prevent memory leaks.
func (wm *WindowManager) DeleteAll() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.windows = make(map[string]*SlidingWindow)
	wm.totalEvents = 0
}

// Count returns the event count for a rule.
func (wm *WindowManager) Count(ruleName string) int {
	w := wm.Get(ruleName)
	if w == nil {
		return 0
	}
	return w.Count()
}
