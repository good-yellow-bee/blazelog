package auth

import (
	"sync"
	"time"
)

// lockoutEntry tracks failed login attempts for an account.
type lockoutEntry struct {
	failures  int
	lockedAt  time.Time
	expiresAt time.Time
}

// LockoutTracker tracks failed login attempts and account lockouts.
//
// Persistence limitation: Lockout state is stored in memory only and will be lost
// on server restart. This is acceptable for single-instance deployments where a
// restart provides a natural cooldown period. For clustered deployments requiring
// persistent lockout state, consider using Redis or database-backed storage.
type LockoutTracker struct {
	mu              sync.RWMutex
	entries         map[string]*lockoutEntry // keyed by username or IP
	threshold       int                      // number of failures before lockout
	lockoutDuration time.Duration
}

// NewLockoutTracker creates a new lockout tracker.
func NewLockoutTracker(threshold int, duration time.Duration) *LockoutTracker {
	tracker := &LockoutTracker{
		entries:         make(map[string]*lockoutEntry),
		threshold:       threshold,
		lockoutDuration: duration,
	}

	// Start cleanup goroutine
	go tracker.cleanupLoop()

	return tracker
}

// RecordFailure records a failed login attempt.
// Returns true if the account is now locked.
func (t *LockoutTracker) RecordFailure(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, exists := t.entries[key]
	if !exists {
		entry = &lockoutEntry{}
		t.entries[key] = entry
	}

	// If already locked and not expired, don't increment
	if entry.lockedAt.After(time.Time{}) && time.Now().Before(entry.expiresAt) {
		return true
	}

	// If lockout expired, reset
	if entry.lockedAt.After(time.Time{}) && time.Now().After(entry.expiresAt) {
		entry.failures = 0
		entry.lockedAt = time.Time{}
		entry.expiresAt = time.Time{}
	}

	entry.failures++

	// Check if we should lock
	if entry.failures >= t.threshold {
		now := time.Now()
		entry.lockedAt = now
		entry.expiresAt = now.Add(t.lockoutDuration)
		return true
	}

	return false
}

// IsLocked returns true if the account is currently locked.
func (t *LockoutTracker) IsLocked(key string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.entries[key]
	if !exists {
		return false
	}

	// Not locked
	if entry.lockedAt.IsZero() {
		return false
	}

	// Check if lockout expired
	if time.Now().After(entry.expiresAt) {
		return false
	}

	return true
}

// RemainingLockoutTime returns how long until the lockout expires.
func (t *LockoutTracker) RemainingLockoutTime(key string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.entries[key]
	if !exists {
		return 0
	}

	if entry.lockedAt.IsZero() {
		return 0
	}

	remaining := time.Until(entry.expiresAt)
	if remaining < 0 {
		return 0
	}

	return remaining
}

// ClearFailures clears failed attempts on successful login.
func (t *LockoutTracker) ClearFailures(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, key)
}

// cleanupLoop periodically removes expired entries.
func (t *LockoutTracker) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.cleanup()
	}
}

// cleanup removes expired entries.
func (t *LockoutTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for key, entry := range t.entries {
		// Remove entries with expired lockouts or no failures
		if entry.failures == 0 || (entry.lockedAt.After(time.Time{}) && now.After(entry.expiresAt)) {
			delete(t.entries, key)
		}
	}
}
