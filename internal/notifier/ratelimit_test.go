package notifier

import (
	"testing"
	"time"
)

func TestRateLimiterBasic(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 3,
		Window:       time.Second,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if rl.Allow() {
		t.Error("4th request should be denied")
	}

	// Check dropped count
	if dropped := rl.Dropped(); dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       100 * time.Millisecond,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Use up the limit
	rl.Allow()
	rl.Allow()

	// Should be denied
	if rl.Allow() {
		t.Error("should be denied before window expires")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow() {
		t.Error("should be allowed after window expires")
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 1,
		Window:       time.Second,
		Enabled:      false, // Disabled
	}
	rl := NewRateLimiter(config)

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		if !rl.Allow() {
			t.Errorf("request %d should be allowed when disabled", i+1)
		}
	}

	// No drops when disabled
	if dropped := rl.Dropped(); dropped != 0 {
		t.Errorf("dropped = %d, want 0 when disabled", dropped)
	}
}

func TestRateLimiterStats(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 5,
		Window:       time.Minute,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Make some requests
	rl.Allow()
	rl.Allow()
	rl.Allow()

	stats := rl.Stats()

	if stats.CurrentCount != 3 {
		t.Errorf("current count = %d, want 3", stats.CurrentCount)
	}
	if stats.MaxPerWindow != 5 {
		t.Errorf("max per window = %d, want 5", stats.MaxPerWindow)
	}
	if stats.Window != time.Minute {
		t.Errorf("window = %v, want 1m", stats.Window)
	}
	if !stats.Enabled {
		t.Error("should be enabled")
	}
	if stats.Dropped != 0 {
		t.Errorf("dropped = %d, want 0", stats.Dropped)
	}
}

func TestRateLimiterReset(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Use up limit and get a drop
	rl.Allow()
	rl.Allow()
	rl.Allow() // This gets dropped

	if dropped := rl.Dropped(); dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}

	// Reset
	rl.Reset()

	// Stats should be cleared
	stats := rl.Stats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count after reset = %d, want 0", stats.CurrentCount)
	}
	if stats.Dropped != 0 {
		t.Errorf("dropped after reset = %d, want 0", stats.Dropped)
	}

	// Should allow again
	if !rl.Allow() {
		t.Error("should allow after reset")
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.MaxPerWindow != 10 {
		t.Errorf("default max = %d, want 10", config.MaxPerWindow)
	}
	if config.Window != time.Minute {
		t.Errorf("default window = %v, want 1m", config.Window)
	}
	if !config.Enabled {
		t.Error("should be enabled by default")
	}
}

func TestNewRateLimiterDefaults(t *testing.T) {
	// Test with zero/invalid values
	config := RateLimitConfig{
		MaxPerWindow: 0,
		Window:       0,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	stats := rl.Stats()
	if stats.MaxPerWindow != 10 {
		t.Errorf("should default to 10, got %d", stats.MaxPerWindow)
	}
	if stats.Window != time.Minute {
		t.Errorf("should default to 1m, got %v", stats.Window)
	}
}

func TestRateLimiterSlidingWindow(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 3,
		Window:       200 * time.Millisecond,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// T=0: Make 2 requests
	rl.Allow()
	rl.Allow()

	// T=100ms: Wait half the window
	time.Sleep(100 * time.Millisecond)

	// T=100ms: Make 1 more request (total 3 in window)
	if !rl.Allow() {
		t.Error("3rd request should be allowed")
	}

	// T=100ms: 4th should be denied
	if rl.Allow() {
		t.Error("4th request should be denied")
	}

	// T=200ms: Wait for first 2 requests to expire
	time.Sleep(100 * time.Millisecond)

	// T=200ms: Should be able to make 2 more requests
	if !rl.Allow() {
		t.Error("should allow after partial expiry")
	}
	if !rl.Allow() {
		t.Error("should allow 2nd after partial expiry")
	}
}

func TestRateLimiterRelease(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 3,
		Window:       time.Minute,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Use up 2 tokens
	rl.Allow()
	rl.Allow()

	stats := rl.Stats()
	if stats.CurrentCount != 2 {
		t.Errorf("current count = %d, want 2", stats.CurrentCount)
	}

	// Release one token
	rl.Release()

	stats = rl.Stats()
	if stats.CurrentCount != 1 {
		t.Errorf("current count after release = %d, want 1", stats.CurrentCount)
	}

	// Should now be able to make 2 more requests (1 remaining + 2 new = 3 max)
	if !rl.Allow() {
		t.Error("should allow after release")
	}
	if !rl.Allow() {
		t.Error("should allow 2nd after release")
	}

	// 4th should be denied (at max now)
	if rl.Allow() {
		t.Error("should deny when at max")
	}
}

func TestRateLimiterReleaseEmpty(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 3,
		Window:       time.Minute,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Release on empty should not panic
	rl.Release()

	stats := rl.Stats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count = %d, want 0", stats.CurrentCount)
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 100,
		Window:       time.Second,
		Enabled:      true,
	}
	rl := NewRateLimiter(config)

	// Run concurrent goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				rl.Allow()
			}
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 100 allowed, 100 dropped
	stats := rl.Stats()
	total := int64(stats.CurrentCount) + stats.Dropped

	// 200 total requests
	if total != 200 {
		t.Errorf("total processed = %d, want 200", total)
	}

	// Exactly 100 should be in window (allowed)
	if stats.CurrentCount != 100 {
		t.Errorf("current count = %d, want 100", stats.CurrentCount)
	}

	// Rest should be dropped
	if stats.Dropped != 100 {
		t.Errorf("dropped = %d, want 100", stats.Dropped)
	}
}
