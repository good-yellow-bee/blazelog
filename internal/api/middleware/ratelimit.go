package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int           // max requests per window
	window   time.Duration // window duration
}

// NewRateLimiter creates a new rate limiter.
// limit is requests per minute.
func NewRateLimiter(limit int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request is allowed for the given key.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Get requests for this key
	requests := rl.requests[key]

	// Filter to only requests within the window
	var validRequests []time.Time
	for _, t := range requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	// Check if under limit
	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests

	return true
}

// cleanupLoop periodically removes old entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes old entries.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	for key, requests := range rl.requests {
		var validRequests []time.Time
		for _, t := range requests {
			if t.After(windowStart) {
				validRequests = append(validRequests, t)
			}
		}
		if len(validRequests) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = validRequests
		}
	}
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
