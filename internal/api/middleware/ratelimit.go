package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements a token bucket rate limiter using sync.Map.
// O(1) per request instead of O(n) sliding window.
type RateLimiter struct {
	limiters sync.Map      // key -> *rateLimiterEntry
	limit    rate.Limit    // requests per second
	burst    int           // max burst size
	window   time.Duration // for cleanup (entries unused for this long are removed)
}

// rateLimiterEntry wraps a limiter with last access time for cleanup.
type rateLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess int64 // unix nano, updated on each access
}

// NewRateLimiter creates a new rate limiter.
// limit is requests per minute.
func NewRateLimiter(limit int) *RateLimiter {
	rl := &RateLimiter{
		limit:  rate.Limit(float64(limit) / 60.0), // convert per-minute to per-second
		burst:  limit,                              // allow burst up to full limit
		window: time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request is allowed for the given key.
// O(1) operation using token bucket algorithm.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now().UnixNano()

	// Load or create limiter for this key
	entry, loaded := rl.limiters.Load(key)
	if !loaded {
		newEntry := &rateLimiterEntry{
			limiter:    rate.NewLimiter(rl.limit, rl.burst),
			lastAccess: now,
		}
		entry, _ = rl.limiters.LoadOrStore(key, newEntry)
	}

	e := entry.(*rateLimiterEntry)
	e.lastAccess = now // Update access time (benign race, approximate is fine)

	return e.limiter.Allow()
}

// cleanupLoop periodically removes stale entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes entries not accessed within the window.
func (rl *RateLimiter) cleanup() {
	cutoff := time.Now().Add(-rl.window).UnixNano()

	rl.limiters.Range(func(key, value any) bool {
		entry := value.(*rateLimiterEntry)
		if entry.lastAccess < cutoff {
			rl.limiters.Delete(key)
		}
		return true
	})
}

// jsonRateLimited writes a rate limited error response.
func jsonRateLimited(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "RATE_LIMITED",
			"message": "too many requests",
		},
	})
}

// RateLimitByIP returns middleware that rate limits by client IP.
func RateLimitByIP(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.Allow(ip) {
				jsonRateLimited(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByUser returns middleware that rate limits by authenticated user.
func RateLimitByUser(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				// Fall back to IP if no user
				userID = getClientIP(r)
			}

			if !limiter.Allow(userID) {
				jsonRateLimited(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
