package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
	"github.com/gorilla/csrf"
)

// parseTimeRange converts a time range string to start/end times and aggregation interval
func parseTimeRange(rangeStr string) (start, end time.Time, interval string) {
	end = time.Now()
	switch rangeStr {
	case "15m":
		return end.Add(-15 * time.Minute), end, "minute"
	case "1h":
		return end.Add(-time.Hour), end, "minute"
	case "6h":
		return end.Add(-6 * time.Hour), end, "hour"
	case "24h":
		return end.Add(-24 * time.Hour), end, "hour"
	case "7d":
		return end.Add(-7 * 24 * time.Hour), end, "day"
	case "30d":
		return end.Add(-30 * 24 * time.Hour), end, "day"
	default:
		return end.Add(-24 * time.Hour), end, "hour"
	}
}

func (h *Handler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Fetch real stats for initial page render
	stats := pages.DashboardStats{}
	if h.logStorage != nil {
		ctx := r.Context()
		filter := &storage.AggregationFilter{
			StartTime: time.Now().Add(-24 * time.Hour),
			EndTime:   time.Now(),
		}
		if rates, err := h.logStorage.Logs().GetErrorRates(ctx, filter); err == nil && rates != nil {
			stats.TotalLogs = int(rates.TotalLogs)
			stats.ErrorCount = int(rates.ErrorCount)
			stats.WarningCount = int(rates.WarningCount)
		}
	}
	if h.storage != nil {
		if alerts, err := h.storage.Alerts().ListEnabled(r.Context()); err == nil {
			stats.ActiveAlerts = len(alerts)
		}
	}

	pages.Dashboard(sess, stats, csrf.Token(r), middleware.GetCSPNonce(r.Context())).Render(r.Context(), w)
}

// DashboardStatsResponse contains JSON response for dashboard stats
type DashboardStatsResponse struct {
	TimeRange    string         `json:"time_range"`
	TotalLogs    int64          `json:"total_logs"`
	ErrorCount   int64          `json:"error_count"`
	WarningCount int64          `json:"warning_count"`
	FatalCount   int64          `json:"fatal_count"`
	ErrorRate    float64        `json:"error_rate"`
	ActiveAlerts int            `json:"active_alerts"`
	Volume       []VolumeData   `json:"volume,omitempty"`
	TopSources   []SourceData   `json:"top_sources,omitempty"`
	HTTPStats    *HTTPStatsData `json:"http_stats,omitempty"`
}

// VolumeData represents a time-series data point for charts
type VolumeData struct {
	Timestamp  string `json:"timestamp"`
	TotalCount int64  `json:"total_count"`
	ErrorCount int64  `json:"error_count"`
}

// SourceData represents log count per source
type SourceData struct {
	Source     string `json:"source"`
	Count      int64  `json:"count"`
	ErrorCount int64  `json:"error_count"`
}

// HTTPStatsData contains HTTP status code distribution
type HTTPStatsData struct {
	Total2xx int64 `json:"total_2xx"`
	Total3xx int64 `json:"total_3xx"`
	Total4xx int64 `json:"total_4xx"`
	Total5xx int64 `json:"total_5xx"`
}

// GetDashboardStats returns dashboard statistics as JSON
func (h *Handler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}
	projectID := r.URL.Query().Get("project_id")

	// Get project access
	var access *middleware.ProjectAccess
	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		var err error
		access, err = middleware.GetProjectAccess(r.Context(), sess.UserID, role, h.storage)
		if err != nil {
			http.Error(w, "failed to get project access", http.StatusInternalServerError)
			return
		}
		// Validate requested project access
		if projectID != "" && !access.CanAccessProject(projectID) {
			http.Error(w, "no access to project", http.StatusForbidden)
			return
		}
	}

	// Check if HTMX request (wants HTML partial)
	if r.Header.Get("HX-Request") == "true" {
		h.renderStatsPartial(w, r, timeRange, projectID, access)
		return
	}

	// Return JSON for JavaScript fetch
	h.renderStatsJSON(w, r, timeRange, projectID, access)
}

func (h *Handler) renderStatsJSON(w http.ResponseWriter, r *http.Request, timeRange, projectID string, access *middleware.ProjectAccess) {
	stats := h.fetchDashboardStats(r.Context(), timeRange, projectID, access)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func (h *Handler) renderStatsPartial(w http.ResponseWriter, r *http.Request, timeRange, projectID string, access *middleware.ProjectAccess) {
	stats := h.fetchDashboardStats(r.Context(), timeRange, projectID, access)
	pageStats := pages.DashboardStats{
		TotalLogs:    int(stats.TotalLogs),
		ErrorCount:   int(stats.ErrorCount),
		WarningCount: int(stats.WarningCount),
		ActiveAlerts: stats.ActiveAlerts,
	}
	pages.StatsCards(pageStats).Render(r.Context(), w)
}

func (h *Handler) fetchDashboardStats(ctx context.Context, timeRange, projectID string, access *middleware.ProjectAccess) *DashboardStatsResponse {
	startTime, endTime, interval := parseTimeRange(timeRange)
	filter := &storage.AggregationFilter{
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Apply project access filtering (error already validated in handler)
	if access != nil {
		if err := access.ApplyToAggregationFilter(filter, projectID); err != nil {
			log.Printf("warning: aggregation filter error: %v", err)
		}
	}

	response := &DashboardStatsResponse{TimeRange: timeRange}

	if h.logStorage != nil {
		logs := h.logStorage.Logs()

		// Fetch all stats in parallel to reduce dashboard latency
		var wg sync.WaitGroup
		var (
			rates     *storage.ErrorRateResult
			vol       []*storage.VolumePoint
			src       []*storage.SourceCount
			httpStats *storage.HTTPStatsResult
		)

		wg.Add(4)
		go func() {
			defer wg.Done()
			r, err := logs.GetErrorRates(ctx, filter)
			if err == nil {
				rates = r
			}
		}()
		go func() {
			defer wg.Done()
			v, err := logs.GetLogVolume(ctx, filter, interval)
			if err == nil {
				vol = v
			}
		}()
		go func() {
			defer wg.Done()
			s, err := logs.GetTopSources(ctx, filter, 5)
			if err == nil {
				src = s
			}
		}()
		go func() {
			defer wg.Done()
			h, err := logs.GetHTTPStats(ctx, filter)
			if err == nil {
				httpStats = h
			}
		}()
		wg.Wait()

		if rates != nil {
			response.TotalLogs = rates.TotalLogs
			response.ErrorCount = rates.ErrorCount
			response.WarningCount = rates.WarningCount
			response.FatalCount = rates.FatalCount
			response.ErrorRate = rates.ErrorRate
		}
		if vol != nil {
			response.Volume = convertVolume(vol)
		}
		if src != nil {
			response.TopSources = convertSources(src)
		}
		if httpStats != nil {
			response.HTTPStats = convertHTTPStats(httpStats)
		}
	}

	// Fetch active alerts count
	if h.storage != nil {
		alertsRepo := h.storage.Alerts()
		if projectID != "" {
			if alerts, err := alertsRepo.ListByProject(ctx, projectID); err == nil {
				for _, a := range alerts {
					if a.Enabled {
						response.ActiveAlerts++
					}
				}
			}
		} else if alerts, err := alertsRepo.ListEnabled(ctx); err == nil {
			if access != nil && !access.AllProjects {
				for _, a := range alerts {
					if access.CanAccessProject(a.ProjectID) {
						response.ActiveAlerts++
					}
				}
			} else {
				response.ActiveAlerts = len(alerts)
			}
		}
	}

	return response
}

func convertVolume(vol []*storage.VolumePoint) []VolumeData {
	if vol == nil {
		return nil
	}
	result := make([]VolumeData, len(vol))
	for i, v := range vol {
		result[i] = VolumeData{
			Timestamp:  v.Timestamp.Format("15:04"),
			TotalCount: v.TotalCount,
			ErrorCount: v.ErrorCount,
		}
	}
	return result
}

func convertSources(src []*storage.SourceCount) []SourceData {
	if src == nil {
		return nil
	}
	result := make([]SourceData, len(src))
	for i, s := range src {
		result[i] = SourceData{
			Source:     s.Source,
			Count:      s.Count,
			ErrorCount: s.ErrorCount,
		}
	}
	return result
}

func convertHTTPStats(stats *storage.HTTPStatsResult) *HTTPStatsData {
	if stats == nil {
		return nil
	}
	return &HTTPStatsData{
		Total2xx: stats.Total2xx,
		Total3xx: stats.Total3xx,
		Total4xx: stats.Total4xx,
		Total5xx: stats.Total5xx,
	}
}
