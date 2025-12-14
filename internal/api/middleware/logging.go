// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// RequestLogger returns a middleware that logs HTTP requests.
func RequestLogger(verbose bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := uuid.New().String()[:8]

			// Add request ID to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Wrap response writer
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log request
			duration := time.Since(start)
			if verbose || wrapped.status >= 400 {
				log.Printf("[%s] %s %s %d %d %v",
					requestID,
					r.Method,
					r.URL.Path,
					wrapped.status,
					wrapped.size,
					duration,
				)
			}
		})
	}
}
