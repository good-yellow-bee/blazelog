package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/good-yellow-bee/blazelog/internal/metrics"
)

// metricsWriter wraps http.ResponseWriter to capture status code for metrics.
type metricsWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// PrometheusMiddleware records HTTP request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Track in-flight requests
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		// Wrap response writer
		wrapped := &metricsWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Get route pattern from chi
		duration := time.Since(start).Seconds()
		path := getRoutePattern(r)

		// Record metrics
		metrics.HTTPRequestsTotal.WithLabelValues(
			r.Method,
			path,
			strconv.Itoa(wrapped.status),
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			r.Method,
			path,
		).Observe(duration)
	})
}

// getRoutePattern extracts the route pattern from chi context.
// Falls back to the request path if no pattern is available.
func getRoutePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	// Fallback for routes without patterns
	return r.URL.Path
}
