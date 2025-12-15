// Package health provides health check endpoints for the API.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Checker defines the interface for health checkers.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Handler manages health check endpoints.
type Handler struct {
	mu       sync.RWMutex
	checkers []Checker
}

// NewHandler creates a new health handler.
func NewHandler() *Handler {
	return &Handler{
		checkers: make([]Checker, 0),
	}
}

// RegisterChecker adds a dependency checker.
func (h *Handler) RegisterChecker(c Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, c)
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Health returns basic health status.
// This endpoint is for simple "is the process running" checks.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// Live returns liveness probe status.
// Returns 200 if the process is running.
// Use for Kubernetes liveness probes.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{Status: "live"})
}

// Ready returns readiness probe status.
// Checks all registered dependencies and returns 200 only if all are healthy.
// Use for Kubernetes readiness probes.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	h.mu.RLock()
	checkers := make([]Checker, len(h.checkers))
	copy(checkers, h.checkers)
	h.mu.RUnlock()

	results := make(map[string]string)
	allHealthy := true

	for _, checker := range checkers {
		if err := checker.Check(ctx); err != nil {
			results[checker.Name()] = err.Error()
			allHealthy = false
		} else {
			results[checker.Name()] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")

	resp := HealthResponse{
		Status: "ready",
		Checks: results,
	}

	if !allHealthy {
		resp.Status = "not_ready"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}
