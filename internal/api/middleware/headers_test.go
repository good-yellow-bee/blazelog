package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRecoverer_LogsPanicWithStackTrace(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic message")
	})

	// Wrap with Recoverer
	handler := Recoverer(panicHandler)

	// Make a test request
	req := httptest.NewRequest("GET", "/test-endpoint", nil)
	rec := httptest.NewRecorder()

	// This should recover from panic and log it
	handler.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}

	// Check log output
	logOutput := buf.String()

	// Verify log contains "PANIC recovered"
	if !strings.Contains(logOutput, "PANIC recovered") {
		t.Errorf("Log missing 'PANIC recovered': %s", logOutput)
	}

	// Verify log contains panic value
	if !strings.Contains(logOutput, "test panic message") {
		t.Errorf("Log missing panic value: %s", logOutput)
	}

	// Verify log contains request info
	if !strings.Contains(logOutput, "GET /test-endpoint") {
		t.Errorf("Log missing request info: %s", logOutput)
	}

	// Verify log contains stack trace indicator
	if !strings.Contains(logOutput, "Stack:") {
		t.Errorf("Log missing 'Stack:' header: %s", logOutput)
	}

	// Verify log contains goroutine (part of stack trace)
	if !strings.Contains(logOutput, "goroutine") {
		t.Errorf("Log missing stack trace goroutine: %s", logOutput)
	}
}

func TestRecoverer_NoPanic(t *testing.T) {
	// Handler that doesn't panic
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := Recoverer(normalHandler)

	req := httptest.NewRequest("GET", "/normal", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check security headers are set
	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("Header %s = %q, want %q", tt.header, got, tt.expected)
		}
	}

	// CSP header should be set (just check it exists)
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy header not set")
	}

	// Permissions-Policy should be set
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy header not set")
	}
}
