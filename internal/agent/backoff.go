package agent

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// Backoff implements exponential backoff with jitter for retry logic.
type Backoff struct {
	Initial    time.Duration // Initial delay (default: 1s)
	Max        time.Duration // Maximum delay (default: 30s)
	Multiplier float64       // Multiplier per attempt (default: 2.0)
	Jitter     float64       // Jitter factor 0-1 (default: 0.1 = 10%)

	attempt int
	mu      sync.Mutex
}

// NewBackoff creates a new Backoff with sensible defaults.
func NewBackoff() *Backoff {
	return &Backoff{
		Initial:    1 * time.Second,
		Max:        30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
		attempt:    0,
	}
}

// NewBackoffWithConfig creates a Backoff with custom configuration.
func NewBackoffWithConfig(initial, max time.Duration, multiplier, jitter float64) *Backoff {
	return &Backoff{
		Initial:    initial,
		Max:        max,
		Multiplier: multiplier,
		Jitter:     jitter,
		attempt:    0,
	}
}

// Next returns the next backoff duration and increments the attempt counter.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate base delay: initial * multiplier^attempt
	delay := float64(b.Initial) * math.Pow(b.Multiplier, float64(b.attempt))

	// Cap at maximum
	if delay > float64(b.Max) {
		delay = float64(b.Max)
	}

	// Apply jitter: delay * (1 + random(-jitter, +jitter))
	if b.Jitter > 0 {
		jitterRange := delay * b.Jitter
		delay = delay + (rand.Float64()*2-1)*jitterRange
	}

	// Ensure non-negative
	if delay < 0 {
		delay = float64(b.Initial)
	}

	b.attempt++
	return time.Duration(delay)
}

// Reset resets the attempt counter to zero.
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempt = 0
}

// Attempt returns the current attempt number.
func (b *Backoff) Attempt() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempt
}
