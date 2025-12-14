package notifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
)

// dispatcherMockNotifier is a test notifier that can be configured to fail.
type dispatcherMockNotifier struct {
	name      string
	shouldErr bool
	sendCount int
}

func (m *dispatcherMockNotifier) Name() string {
	return m.name
}

func (m *dispatcherMockNotifier) Send(ctx context.Context, alert *alerting.Alert) error {
	m.sendCount++
	if m.shouldErr {
		return errors.New("mock send error")
	}
	return nil
}

func (m *dispatcherMockNotifier) Close() error {
	return nil
}

func TestDispatcherRefundsTokenOnAllFailures(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Register a failing notifier
	failingNotifier := &dispatcherMockNotifier{name: "failing", shouldErr: true}
	dispatcher.Register(failingNotifier)

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
		Notify:    []string{"failing"},
	}

	// First dispatch - fails, should refund token
	err := dispatcher.Dispatch(context.Background(), alert)
	if err == nil {
		t.Error("expected error from failing notifier")
	}

	// Check rate limit stats - should be 0 because token was refunded
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count = %d, want 0 (token should be refunded)", stats.CurrentCount)
	}

	// Should be able to dispatch again (token was refunded)
	err = dispatcher.Dispatch(context.Background(), alert)
	if err == nil {
		t.Error("expected error from failing notifier")
	}

	// Still 0 because both failed and were refunded
	stats = dispatcher.RateLimitStats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count = %d, want 0 after second failure", stats.CurrentCount)
	}
}

func TestDispatcherKeepsTokenOnPartialSuccess(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Register one failing and one succeeding notifier
	failingNotifier := &dispatcherMockNotifier{name: "failing", shouldErr: true}
	successNotifier := &dispatcherMockNotifier{name: "success", shouldErr: false}
	dispatcher.Register(failingNotifier)
	dispatcher.Register(successNotifier)

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
		Notify:    []string{"failing", "success"},
	}

	// Dispatch - one succeeds, one fails, token should NOT be refunded
	err := dispatcher.Dispatch(context.Background(), alert)
	if err == nil {
		t.Error("expected error due to partial failure")
	}

	// Check rate limit stats - should be 1 because partial success
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 1 {
		t.Errorf("current count = %d, want 1 (token should be kept on partial success)", stats.CurrentCount)
	}
}

func TestDispatcherKeepsTokenOnFullSuccess(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Register a succeeding notifier
	successNotifier := &dispatcherMockNotifier{name: "success", shouldErr: false}
	dispatcher.Register(successNotifier)

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
		Notify:    []string{"success"},
	}

	// Dispatch - succeeds, token should be consumed
	err := dispatcher.Dispatch(context.Background(), alert)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check rate limit stats - should be 1
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 1 {
		t.Errorf("current count = %d, want 1", stats.CurrentCount)
	}
}

func TestDispatchAllRefundsTokenOnAllFailures(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Register failing notifiers
	failing1 := &dispatcherMockNotifier{name: "failing1", shouldErr: true}
	failing2 := &dispatcherMockNotifier{name: "failing2", shouldErr: true}
	dispatcher.Register(failing1)
	dispatcher.Register(failing2)

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
	}

	// DispatchAll - all fail, should refund token
	err := dispatcher.DispatchAll(context.Background(), alert)
	if err == nil {
		t.Error("expected error from failing notifiers")
	}

	// Check rate limit stats - should be 0 because token was refunded
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count = %d, want 0 (token should be refunded)", stats.CurrentCount)
	}
}

func TestDispatchAllKeepsTokenOnPartialSuccess(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Register one failing and one succeeding notifier
	failingNotifier := &dispatcherMockNotifier{name: "failing", shouldErr: true}
	successNotifier := &dispatcherMockNotifier{name: "success", shouldErr: false}
	dispatcher.Register(failingNotifier)
	dispatcher.Register(successNotifier)

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
	}

	// DispatchAll - one succeeds, one fails, token should NOT be refunded
	err := dispatcher.DispatchAll(context.Background(), alert)
	if err == nil {
		t.Error("expected error due to partial failure")
	}

	// Check rate limit stats - should be 1 because partial success
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 1 {
		t.Errorf("current count = %d, want 1 (token should be kept on partial success)", stats.CurrentCount)
	}
}

func TestDispatchRefundsWhenNoNotifiersFound(t *testing.T) {
	config := RateLimitConfig{
		MaxPerWindow: 2,
		Window:       time.Minute,
		Enabled:      true,
	}
	dispatcher := NewDispatcherWithRateLimit(config)

	// Don't register any notifiers

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
		Notify:    []string{"nonexistent"},
	}

	// Dispatch - no notifiers found, should refund token
	err := dispatcher.Dispatch(context.Background(), alert)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check rate limit stats - should be 0 because no sends happened
	stats := dispatcher.RateLimitStats()
	if stats.CurrentCount != 0 {
		t.Errorf("current count = %d, want 0 (token should be refunded when no notifiers found)", stats.CurrentCount)
	}
}

