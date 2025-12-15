package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
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

	// TODO: Fetch real stats from storage
	stats := pages.DashboardStats{
		TotalLogs:    12543,
		ErrorCount:   87,
		WarningCount: 234,
		ActiveAlerts: 3,
	}

	pages.Dashboard(sess, stats).Render(r.Context(), w)
}

// DashboardStatsResponse contains JSON response for dashboard stats
type DashboardStatsResponse struct {
	TimeRange    string          `json:"time_range"`
	TotalLogs    int64           `json:"total_logs"`
	ErrorCount   int64           `json:"error_count"`
	WarningCount int64           `json:"warning_count"`
	FatalCount   int64           `json:"fatal_count"`
	ErrorRate    float64         `json:"error_rate"`
	ActiveAlerts int             `json:"active_alerts"`
	Volume       []VolumeData    `json:"volume,omitempty"`
	TopSources   []SourceData    `json:"top_sources,omitempty"`
	HTTPStats    *HTTPStatsData  `json:"http_stats,omitempty"`
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

	// Check if HTMX request (wants HTML partial)
	if r.Header.Get("HX-Request") == "true" {
		h.renderStatsPartial(w, r, timeRange)
		return
	}

	// Return JSON for JavaScript fetch
	h.renderStatsJSON(w, r, timeRange)
}

func (h *Handler) renderStatsJSON(w http.ResponseWriter, r *http.Request, timeRange string) {
	stats := h.fetchDashboardStats(r.Context(), timeRange)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) renderStatsPartial(w http.ResponseWriter, r *http.Request, timeRange string) {
	stats := h.fetchDashboardStats(r.Context(), timeRange)
	pageStats := pages.DashboardStats{
		TotalLogs:    int(stats.TotalLogs),
		ErrorCount:   int(stats.ErrorCount),
		WarningCount: int(stats.WarningCount),
		ActiveAlerts: stats.ActiveAlerts,
	}
	pages.StatsCards(pageStats).Render(r.Context(), w)
}

func (h *Handler) fetchDashboardStats(ctx context.Context, timeRange string) *DashboardStatsResponse {
	startTime, endTime, interval := parseTimeRange(timeRange)
	filter := &storage.AggregationFilter{
		StartTime: startTime,
		EndTime:   endTime,
	}

	response := &DashboardStatsResponse{TimeRange: timeRange}

	if h.logStorage != nil {
		logs := h.logStorage.Logs()

		// Fetch error rates
		if rates, err := logs.GetErrorRates(ctx, filter); err == nil && rates != nil {
			response.TotalLogs = rates.TotalLogs
			response.ErrorCount = rates.ErrorCount
			response.WarningCount = rates.WarningCount
			response.FatalCount = rates.FatalCount
			response.ErrorRate = rates.ErrorRate
		}

		// Fetch volume data
		if vol, err := logs.GetLogVolume(ctx, filter, interval); err == nil {
			response.Volume = convertVolume(vol)
		}

		// Fetch top sources
		if src, err := logs.GetTopSources(ctx, filter, 5); err == nil {
			response.TopSources = convertSources(src)
		}

		// Fetch HTTP stats
		if httpStats, err := logs.GetHTTPStats(ctx, filter); err == nil && httpStats != nil {
			response.HTTPStats = convertHTTPStats(httpStats)
		}
	}

	// Fetch active alerts count
	if h.storage != nil {
		if alerts, err := h.storage.Alerts().List(ctx); err == nil {
			for _, a := range alerts {
				if a.Enabled {
					response.ActiveAlerts++
				}
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
