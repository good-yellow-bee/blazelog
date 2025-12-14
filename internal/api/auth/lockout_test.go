package auth

import (
	"testing"
	"time"
)

func TestLockoutTracker_Basic(t *testing.T) {
	tracker := NewLockoutTracker(3, 100*time.Millisecond)
	username := "testuser"

	// Initially not locked
	if tracker.IsLocked(username) {
		t.Error("user should not be locked initially")
	}

	// Record failures below threshold
	tracker.RecordFailure(username)
	tracker.RecordFailure(username)
	if tracker.IsLocked(username) {
		t.Error("user should not be locked after 2 failures (threshold=3)")
	}

	// Third failure should trigger lockout
	tracker.RecordFailure(username)
	if !tracker.IsLocked(username) {
		t.Error("user should be locked after 3 failures")
	}
}

func TestLockoutTracker_LockoutExpires(t *testing.T) {
	tracker := NewLockoutTracker(2, 50*time.Millisecond)
	username := "testuser"

	tracker.RecordFailure(username)
	tracker.RecordFailure(username)

	if !tracker.IsLocked(username) {
		t.Error("user should be locked")
	}

	// Wait for lockout to expire
	time.Sleep(60 * time.Millisecond)

	if tracker.IsLocked(username) {
		t.Error("lockout should have expired")
	}
}

func TestLockoutTracker_ClearFailures(t *testing.T) {
	tracker := NewLockoutTracker(3, time.Hour)
	username := "testuser"

	tracker.RecordFailure(username)
	tracker.RecordFailure(username)
	tracker.RecordFailure(username)

	if !tracker.IsLocked(username) {
		t.Error("user should be locked")
	}

	tracker.ClearFailures(username)

	if tracker.IsLocked(username) {
		t.Error("user should not be locked after clear")
	}
}

func TestLockoutTracker_RemainingTime(t *testing.T) {
	lockoutDuration := 100 * time.Millisecond
	tracker := NewLockoutTracker(1, lockoutDuration)
	username := "testuser"

	// No lockout initially
	remaining := tracker.RemainingLockoutTime(username)
	if remaining != 0 {
		t.Errorf("remaining time should be 0, got %v", remaining)
	}

	tracker.RecordFailure(username)

	remaining = tracker.RemainingLockoutTime(username)
	if remaining <= 0 {
		t.Error("remaining time should be positive after lockout")
	}
	if remaining > lockoutDuration {
		t.Errorf("remaining time %v should not exceed lockout duration %v", remaining, lockoutDuration)
	}
}

func TestLockoutTracker_IndependentUsers(t *testing.T) {
	tracker := NewLockoutTracker(2, time.Hour)
	user1 := "user1"
	user2 := "user2"

	tracker.RecordFailure(user1)
	tracker.RecordFailure(user1)

	if !tracker.IsLocked(user1) {
		t.Error("user1 should be locked")
	}
	if tracker.IsLocked(user2) {
		t.Error("user2 should not be locked")
	}
}

func TestLockoutTracker_FailureCountReset(t *testing.T) {
	tracker := NewLockoutTracker(2, 30*time.Millisecond)
	username := "testuser"

	// Record one failure, then clear
	tracker.RecordFailure(username)
	tracker.ClearFailures(username)

	// Should need 2 failures again, not 1
	tracker.RecordFailure(username)
	if tracker.IsLocked(username) {
		t.Error("user should not be locked after clear and 1 failure")
	}

	tracker.RecordFailure(username)
	if !tracker.IsLocked(username) {
		t.Error("user should be locked after 2 failures")
	}
}
