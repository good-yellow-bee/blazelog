package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
)

// cspNonceKey stores the request-specific CSP nonce in context.
const cspNonceKey contextKey = "csp_nonce"

// GetCSPNonce returns the per-request CSP nonce from context.
func GetCSPNonce(ctx context.Context) string {
	if v := ctx.Value(cspNonceKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func generateCSPNonce() (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(nonceBytes), nil
}

func buildCSPHeader(nonce string) string {
	scriptSrc := []string{"'self'", "https://cdn.jsdelivr.net"}
	if nonce != "" {
		scriptSrc = append([]string{"'self'", "'nonce-" + nonce + "'"}, "https://cdn.jsdelivr.net")
	} else {
		// Fallback for nonce generation failure to avoid breaking web UI.
		scriptSrc = append([]string{"'self'", "'unsafe-inline'"}, "https://cdn.jsdelivr.net")
	}

	return "default-src 'self'; " +
		"script-src " + strings.Join(scriptSrc, " ") + "; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src 'self' https://fonts.gstatic.com; " +
		"img-src 'self' data:; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'"
}

// SecurityHeaders adds security-related HTTP headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := generateCSPNonce()
		if err != nil {
			log.Printf("warning: failed to generate CSP nonce: %v", err)
		} else {
			r = r.WithContext(context.WithValue(r.Context(), cspNonceKey, nonce))
		}

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS protection (legacy, but still useful)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy with per-request script nonce.
		w.Header().Set("Content-Security-Policy", buildCSPHeader(nonce))

		// HSTS only for secure requests
		if IsRequestSecure(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

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
