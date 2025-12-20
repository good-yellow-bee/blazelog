package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// SecurityHeaders adds security-related HTTP headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS protection (legacy, but still useful)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - allow HTMX/Alpine.js from CDN and inline scripts/styles
		// Note: 'unsafe-eval' is required by Alpine.js for expression evaluation
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self'")

		// Permissions policy (modern replacement for Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// Recoverer recovers from panics, logs them with stack trace, and returns a 500 error.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC recovered: %v\nRequest: %s %s\nStack:\n%s",
					err, r.Method, r.URL.Path, debug.Stack())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if _, writeErr := w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`)); writeErr != nil {
					log.Printf("Failed to write error response: %v", writeErr)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}
