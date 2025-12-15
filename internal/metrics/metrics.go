// Package metrics provides Prometheus metrics for BlazeLog.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "blazelog"
)

// HTTP metrics
var (
	// HTTPRequestsTotal counts HTTP requests by method, path, and status.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks HTTP request latency.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight tracks concurrent HTTP requests.
	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)
)

// gRPC metrics
var (
	// GRPCStreamsActive tracks active gRPC streams.
	GRPCStreamsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "streams_active",
			Help:      "Number of active gRPC log streams",
		},
	)

	// GRPCBatchesTotal counts received log batches.
	GRPCBatchesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "batches_total",
			Help:      "Total log batches received via gRPC",
		},
	)

	// GRPCEntriesTotal counts received log entries.
	GRPCEntriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "entries_total",
			Help:      "Total log entries received via gRPC",
		},
	)

	// GRPCAgentsRegistered tracks registered agents.
	GRPCAgentsRegistered = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "agents_registered",
			Help:      "Number of registered agents",
		},
	)

	// GRPCBatchProcessErrors counts batch processing errors.
	GRPCBatchProcessErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "batch_process_errors_total",
			Help:      "Total batch processing errors",
		},
	)
)

// Buffer metrics
var (
	// BufferPending tracks entries waiting to be flushed.
	BufferPending = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "buffer",
			Name:      "pending_entries",
			Help:      "Log entries waiting to be flushed to storage",
		},
	)

	// BufferDroppedTotal counts dropped entries due to backpressure.
	BufferDroppedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "buffer",
			Name:      "dropped_total",
			Help:      "Total entries dropped due to buffer overflow",
		},
	)

	// BufferFlushesTotal counts flush operations.
	BufferFlushesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "buffer",
			Name:      "flushes_total",
			Help:      "Total buffer flush operations",
		},
	)

	// BufferInsertedTotal counts successfully inserted entries.
	BufferInsertedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "buffer",
			Name:      "inserted_total",
			Help:      "Total entries inserted to storage",
		},
	)

	// BufferFlushErrors counts flush errors.
	BufferFlushErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "buffer",
			Name:      "flush_errors_total",
			Help:      "Total buffer flush errors",
		},
	)
)

// Storage metrics
var (
	// StorageQueryDuration tracks query latency.
	StorageQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "storage",
			Name:      "query_duration_seconds",
			Help:      "Storage query latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "backend"},
	)

	// StorageErrors counts storage operation errors.
	StorageErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "storage",
			Name:      "errors_total",
			Help:      "Total storage operation errors",
		},
		[]string{"operation", "backend"},
	)
)

// Auth metrics
var (
	// AuthAttemptsTotal counts authentication attempts.
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "attempts_total",
			Help:      "Total authentication attempts",
		},
		[]string{"result"}, // success, failure, locked
	)

	// AuthTokensIssued counts issued tokens.
	AuthTokensIssued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "auth",
			Name:      "tokens_issued_total",
			Help:      "Total tokens issued",
		},
		[]string{"type"}, // access, refresh
	)
)

// Info metric
var (
	// BuildInfo exposes build information.
	BuildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_info",
			Help:      "Build information",
		},
		[]string{"version", "commit", "build_time"},
	)
)

// SetBuildInfo sets the build info metric.
func SetBuildInfo(version, commit, buildTime string) {
	BuildInfo.WithLabelValues(version, commit, buildTime).Set(1)
}
