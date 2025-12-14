// Package notifier provides notification dispatching for alerts.
package notifier

import (
	"context"
	"fmt"
	"sync"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
)

// Notifier is the interface for all notification channels.
type Notifier interface {
	// Name returns the notifier name (e.g., "email", "slack").
	Name() string
	// Send sends an alert notification.
	Send(ctx context.Context, alert *alerting.Alert) error
	// Close releases any resources.
	Close() error
}

// Dispatcher manages multiple notifiers and routes alerts.
type Dispatcher struct {
	mu          sync.RWMutex
	notifiers   map[string]Notifier
	rateLimiter *RateLimiter
}

// NewDispatcher creates a new notification dispatcher with default rate limiting.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		notifiers:   make(map[string]Notifier),
		rateLimiter: NewRateLimiter(DefaultRateLimitConfig()),
	}
}

// NewDispatcherWithRateLimit creates a dispatcher with custom rate limit configuration.
func NewDispatcherWithRateLimit(config RateLimitConfig) *Dispatcher {
	return &Dispatcher{
		notifiers:   make(map[string]Notifier),
		rateLimiter: NewRateLimiter(config),
	}
}

// Register adds a notifier to the dispatcher.
func (d *Dispatcher) Register(n Notifier) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notifiers[n.Name()] = n
}

// Unregister removes a notifier from the dispatcher.
func (d *Dispatcher) Unregister(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.notifiers, name)
}

// Get returns a notifier by name.
func (d *Dispatcher) Get(name string) (Notifier, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.notifiers[name]
	return n, ok
}

// ErrRateLimited is returned when a notification is dropped due to rate limiting.
var ErrRateLimited = fmt.Errorf("notification rate limited")

// Dispatch sends an alert to all notifiers specified in alert.Notify.
// If alert.Notify is empty, the alert is not sent to any notifier.
// Returns ErrRateLimited if the notification is dropped due to rate limiting.
func (d *Dispatcher) Dispatch(ctx context.Context, alert *alerting.Alert) error {
	if len(alert.Notify) == 0 {
		return nil
	}

	// Check rate limit
	if d.rateLimiter != nil && !d.rateLimiter.Allow() {
		return ErrRateLimited
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var errs []error
	for _, name := range alert.Notify {
		n, ok := d.notifiers[name]
		if !ok {
			continue
		}
		if err := n.Send(ctx, alert); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %v", errs)
	}
	return nil
}

// DispatchAll sends an alert to all registered notifiers regardless of alert.Notify.
// Returns ErrRateLimited if the notification is dropped due to rate limiting.
func (d *Dispatcher) DispatchAll(ctx context.Context, alert *alerting.Alert) error {
	// Check rate limit
	if d.rateLimiter != nil && !d.rateLimiter.Allow() {
		return ErrRateLimited
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var errs []error
	for name, n := range d.notifiers {
		if err := n.Send(ctx, alert); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %v", errs)
	}
	return nil
}

// RateLimitStats returns the rate limiter statistics.
func (d *Dispatcher) RateLimitStats() RateLimitStats {
	if d.rateLimiter == nil {
		return RateLimitStats{}
	}
	return d.rateLimiter.Stats()
}

// Close closes all registered notifiers.
func (d *Dispatcher) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errs []error
	for name, n := range d.notifiers {
		if err := n.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	d.notifiers = make(map[string]Notifier)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
