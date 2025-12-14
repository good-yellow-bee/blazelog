package agent

import (
	"testing"
	"time"
)

func TestBackoff_Next(t *testing.T) {
	b := NewBackoff()

	// First attempt should be around Initial (1s) +/- jitter
	d1 := b.Next()
	if d1 < 800*time.Millisecond || d1 > 1200*time.Millisecond {
		t.Errorf("first delay %v not within expected range [800ms, 1200ms]", d1)
	}

	// Second attempt should be around 2s +/- jitter
	d2 := b.Next()
	if d2 < 1600*time.Millisecond || d2 > 2400*time.Millisecond {
		t.Errorf("second delay %v not within expected range [1.6s, 2.4s]", d2)
	}

	// Third attempt should be around 4s +/- jitter
	d3 := b.Next()
	if d3 < 3200*time.Millisecond || d3 > 4800*time.Millisecond {
		t.Errorf("third delay %v not within expected range [3.2s, 4.8s]", d3)
	}
}

func TestBackoff_Max(t *testing.T) {
	b := NewBackoffWithConfig(1*time.Second, 5*time.Second, 2.0, 0)

	// Run many iterations
	for i := 0; i < 10; i++ {
		d := b.Next()
		if d > 5*time.Second {
			t.Errorf("delay %v exceeded max 5s", d)
		}
	}
}

func TestBackoff_Reset(t *testing.T) {
	b := NewBackoff()

	// Advance a few attempts
	b.Next()
	b.Next()
	b.Next()

	if b.Attempt() != 3 {
		t.Errorf("expected attempt 3, got %d", b.Attempt())
	}

	b.Reset()

	if b.Attempt() != 0 {
		t.Errorf("expected attempt 0 after reset, got %d", b.Attempt())
	}

	// First delay after reset should be around Initial
	d := b.Next()
	if d < 800*time.Millisecond || d > 1200*time.Millisecond {
		t.Errorf("delay after reset %v not within expected range", d)
	}
}

func TestBackoff_NoJitter(t *testing.T) {
	b := NewBackoffWithConfig(100*time.Millisecond, 1*time.Second, 2.0, 0)

	// Without jitter, delays should be exact
	delays := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1000 * time.Millisecond, // capped at max
	}

	for i, expected := range delays {
		got := b.Next()
		if got != expected {
			t.Errorf("attempt %d: expected %v, got %v", i, expected, got)
		}
	}
}
