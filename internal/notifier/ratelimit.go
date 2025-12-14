package notifier

import (
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter for notifications.
type RateLimiter struct {
	mu            sync.Mutex
	maxPerWindow  int
	window        time.Duration
	timestamps    []time.Time
	dropped       int64
	enabled       bool
}

// RateLimitConfig holds rate limiter configuration.
type RateLimitConfig struct {
	MaxPerWindow int           // Maximum notifications per window (default: 10)
	Window       time.Duration // Time window (default: 1 minute)
	Enabled      bool          // Whether rate limiting is enabled (default: true)
}

// DefaultRateLimitConfig returns default rate limit settings.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxPerWindow: 10,
		Window:       time.Minute,
		Enabled:      true,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	if config.MaxPerWindow <= 0 {
		config.MaxPerWindow = 10
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}

	return &RateLimiter{
		maxPerWindow: config.MaxPerWindow,
		window:       config.Window,
		timestamps:   make([]time.Time, 0, config.MaxPerWindow),
		enabled:      config.Enabled,
	}
}

// Allow checks if a notification is allowed under the rate limit.
// Returns true if allowed, false if rate limit exceeded.
func (r *RateLimiter) Allow() bool {
	if !r.enabled {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Remove timestamps outside the window
	r.cleanup(cutoff)

	// Check if under limit
	if len(r.timestamps) >= r.maxPerWindow {
		r.dropped++
		return false
	}

	// Add current timestamp
	r.timestamps = append(r.timestamps, now)
	return true
}

// Release refunds the most recently consumed token.
// Call this when a notification attempt fails after Allow() returned true.
func (r *RateLimiter) Release() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.timestamps) > 0 {
		r.timestamps = r.timestamps[:len(r.timestamps)-1]
	}
}

// cleanup removes timestamps older than the cutoff time.
// Must be called with mutex held.
func (r *RateLimiter) cleanup(cutoff time.Time) {
	// Find first timestamp within window
	idx := 0
	for idx < len(r.timestamps) && r.timestamps[idx].Before(cutoff) {
		idx++
	}

	if idx > 0 {
		// Shift timestamps
		copy(r.timestamps, r.timestamps[idx:])
		r.timestamps = r.timestamps[:len(r.timestamps)-idx]
	}
}

// Dropped returns the number of notifications dropped due to rate limiting.
func (r *RateLimiter) Dropped() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// Stats returns rate limiter statistics.
func (r *RateLimiter) Stats() RateLimitStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	return RateLimitStats{
		Dropped:      r.dropped,
		CurrentCount: len(r.timestamps),
		MaxPerWindow: r.maxPerWindow,
		Window:       r.window,
		Enabled:      r.enabled,
	}
}

// RateLimitStats contains rate limiter statistics.
type RateLimitStats struct {
	Dropped      int64         // Total notifications dropped
	CurrentCount int           // Current count in window
	MaxPerWindow int           // Maximum allowed per window
	Window       time.Duration // Window duration
	Enabled      bool          // Whether rate limiting is enabled
}

// Reset clears the rate limiter state.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.timestamps = r.timestamps[:0]
	r.dropped = 0
}
