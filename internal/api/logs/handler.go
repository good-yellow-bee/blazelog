// Package logs provides HTTP handlers for log query and streaming endpoints.
package logs

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/query"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"golang.org/x/sync/errgroup"
)

// Response helpers (local to avoid import cycle with api package)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

type apiResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error *apiError   `json:"error,omitempty"`
}

const (
	errCodeBadRequest    = "BAD_REQUEST"
	errCodeInternalError = "INTERNAL_ERROR"
	maxFilterLength      = 1000
)

func jsonError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{Error: &apiError{Code: code, Message: message}})
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiResponse{Data: data})
}

// Handler handles log query and streaming endpoints.
type Handler struct {
	logStorage storage.LogStorage
}

// NewHandler creates a new logs handler.
func NewHandler(logStore storage.LogStorage) *Handler {
	return &Handler{logStorage: logStore}
}

// LogResponse represents a log entry in API responses.
type LogResponse struct {
	ID         string                 `json:"id"`
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Source     string                 `json:"source,omitempty"`
	Type       string                 `json:"type,omitempty"`
	AgentID    string                 `json:"agent_id,omitempty"`
	FilePath   string                 `json:"file_path,omitempty"`
	LineNumber int64                  `json:"line_number,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	HTTPStatus int                    `json:"http_status,omitempty"`
	HTTPMethod string                 `json:"http_method,omitempty"`
	URI        string                 `json:"uri,omitempty"`
}

// LogsResponse wraps a paginated list of logs.
type LogsResponse struct {
	Items      []*LogResponse `json:"items"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

// StatsResponse contains aggregated log statistics.
type StatsResponse struct {
	ErrorRates *ErrorRatesResponse `json:"error_rates"`
	TopSources []*SourceResponse   `json:"top_sources"`
	Volume     []*VolumeResponse   `json:"volume"`
	HTTPStats  *HTTPStatsResponse  `json:"http_stats,omitempty"`
}

// ErrorRatesResponse contains error rate statistics.
type ErrorRatesResponse struct {
	TotalLogs    int64   `json:"total_logs"`
	ErrorCount   int64   `json:"error_count"`
	WarningCount int64   `json:"warning_count"`
	FatalCount   int64   `json:"fatal_count"`
	ErrorRate    float64 `json:"error_rate"`
}

// SourceResponse represents log count per source.
type SourceResponse struct {
	Source     string `json:"source"`
	Count      int64  `json:"count"`
	ErrorCount int64  `json:"error_count"`
}

// VolumeResponse represents a time-series data point.
type VolumeResponse struct {
	Timestamp  string `json:"timestamp"`
	TotalCount int64  `json:"total_count"`
	ErrorCount int64  `json:"error_count"`
}

// HTTPStatsResponse contains HTTP status distribution.
type HTTPStatsResponse struct {
	Total2xx int64          `json:"total_2xx"`
	Total3xx int64          `json:"total_3xx"`
	Total4xx int64          `json:"total_4xx"`
	Total5xx int64          `json:"total_5xx"`
	TopURIs  []*URIResponse `json:"top_uris,omitempty"`
}

// URIResponse represents request count per URI.
type URIResponse struct {
	URI   string `json:"uri"`
	Count int64  `json:"count"`
}

// Query handles GET /api/v1/logs - query logs with filters and pagination.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	if h.logStorage == nil {
		jsonError(w, http.StatusServiceUnavailable, errCodeInternalError, "log storage not configured")
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse required start time
	startStr := q.Get("start")
	if startStr == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "start time is required")
		return
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid start time format (use RFC3339)")
		return
	}

	// Parse end time (default: now)
	endTime := time.Now()
	if endStr := q.Get("end"); endStr != "" {
		endTime, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid end time format (use RFC3339)")
			return
		}
	}

	// Validate time range
	if startTime.After(endTime) {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "start time must be before end time")
		return
	}

	// Parse pagination
	page := 1
	if pageStr := q.Get("page"); pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid page number")
			return
		}
	}

	perPage := 50
	if perPageStr := q.Get("per_page"); perPageStr != "" {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil || perPage < 1 || perPage > 1000 {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "per_page must be between 1 and 1000")
			return
		}
	}

	// Parse search mode
	searchMode := storage.SearchModeToken
	if modeStr := q.Get("search_mode"); modeStr != "" {
		switch strings.ToLower(modeStr) {
		case "token":
			searchMode = storage.SearchModeToken
		case "substring":
			searchMode = storage.SearchModeSubstring
		case "phrase":
			searchMode = storage.SearchModePhrase
		default:
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "search_mode must be token, substring, or phrase")
			return
		}
	}

	// Parse order
	orderBy := "timestamp"
	if ob := q.Get("order"); ob != "" {
		if ob != "timestamp" && ob != "level" {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "order must be timestamp or level")
			return
		}
		orderBy = ob
	}

	orderDesc := true
	if od := q.Get("order_dir"); od != "" {
		switch strings.ToLower(od) {
		case "desc":
			orderDesc = true
		case "asc":
			orderDesc = false
		default:
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "order_dir must be asc or desc")
			return
		}
	}

	// Parse levels
	var levels []string
	if levelsStr := q.Get("levels"); levelsStr != "" {
		levels = strings.Split(levelsStr, ",")
		for i := range levels {
			levels[i] = strings.TrimSpace(strings.ToLower(levels[i]))
		}
	}

	// Parse DSL filter expression (takes precedence over flat filters)
	var filterSQL string
	var filterArgs []any
	filterExpr := q.Get("filter")
	if len(filterExpr) > maxFilterLength {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, fmt.Sprintf("filter expression too long (max %d chars)", maxFilterLength))
		return
	}
	if filterExpr != "" {
		dsl := query.NewQueryDSL(query.DefaultFields)
		parsed, err := dsl.Parse(filterExpr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, fmt.Sprintf("invalid filter expression: %v", err))
			return
		}

		builder := query.NewSQLBuilder(query.DefaultFields)
		result, err := builder.Build(parsed)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, fmt.Sprintf("filter conversion error: %v", err))
			return
		}
		filterSQL = result.SQL
		filterArgs = result.Args
	}

	// Build filter - DSL takes precedence over flat filters
	agentID := q.Get("agent_id")
	level := strings.ToLower(q.Get("level"))
	fileType := strings.ToLower(q.Get("type"))
	source := q.Get("source")
	filePath := q.Get("file_path")
	messageContains := q.Get("q")

	if filterExpr != "" {
		agentID = ""
		level = ""
		levels = nil
		fileType = ""
		source = ""
		filePath = ""
		messageContains = ""
	}

	filter := &storage.LogFilter{
		StartTime:       startTime,
		EndTime:         endTime,
		AgentID:         agentID,
		Level:           level,
		Levels:          levels,
		Type:            fileType,
		Source:          source,
		FilePath:        filePath,
		MessageContains: messageContains,
		SearchMode:      searchMode,
		Limit:           perPage,
		Offset:          (page - 1) * perPage,
		OrderBy:         orderBy,
		OrderDesc:       orderDesc,
		FilterExpr:      filterExpr,
		FilterSQL:       filterSQL,
		FilterArgs:      filterArgs,
	}

	// Execute query
	result, err := h.logStorage.Logs().Query(ctx, filter)
	if err != nil {
		log.Printf("log query error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Convert to response
	items := make([]*LogResponse, len(result.Entries))
	for i, entry := range result.Entries {
		items[i] = recordToResponse(entry)
	}

	// Calculate total pages
	totalPages := 0
	if result.Total > 0 {
		totalPages = int(math.Ceil(float64(result.Total) / float64(perPage)))
	}

	jsonOK(w, &LogsResponse{
		Items:      items,
		Total:      result.Total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

// Stats handles GET /api/v1/logs/stats - aggregated statistics.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if h.logStorage == nil {
		jsonError(w, http.StatusServiceUnavailable, errCodeInternalError, "log storage not configured")
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse required start time
	startStr := q.Get("start")
	if startStr == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "start time is required")
		return
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid start time format (use RFC3339)")
		return
	}

	// Parse end time (default: now)
	endTime := time.Now()
	if endStr := q.Get("end"); endStr != "" {
		endTime, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid end time format (use RFC3339)")
			return
		}
	}

	// Parse interval
	interval := "hour"
	if iv := q.Get("interval"); iv != "" {
		if iv != "minute" && iv != "hour" && iv != "day" {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "interval must be minute, hour, or day")
			return
		}
		interval = iv
	}

	// Build aggregation filter
	aggFilter := &storage.AggregationFilter{
		StartTime: startTime,
		EndTime:   endTime,
		AgentID:   q.Get("agent_id"),
		Type:      q.Get("type"),
	}

	// Execute all 4 queries in parallel for ~4x latency improvement
	var (
		errorRates *storage.ErrorRateResult
		topSources []*storage.SourceCount
		volume     []*storage.VolumePoint
		httpStats  *storage.HTTPStatsResult
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		errorRates, err = h.logStorage.Logs().GetErrorRates(gCtx, aggFilter)
		if err != nil {
			log.Printf("error rates query error: %v", err)
		}
		return err
	})

	g.Go(func() error {
		var err error
		topSources, err = h.logStorage.Logs().GetTopSources(gCtx, aggFilter, 10)
		if err != nil {
			log.Printf("top sources query error: %v", err)
		}
		return err
	})

	g.Go(func() error {
		var err error
		volume, err = h.logStorage.Logs().GetLogVolume(gCtx, aggFilter, interval)
		if err != nil {
			log.Printf("log volume query error: %v", err)
		}
		return err
	})

	g.Go(func() error {
		var err error
		httpStats, err = h.logStorage.Logs().GetHTTPStats(gCtx, aggFilter)
		if err != nil {
			log.Printf("http stats query error: %v", err)
		}
		return err
	})

	if err := g.Wait(); err != nil {
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Build response
	resp := &StatsResponse{
		ErrorRates: &ErrorRatesResponse{
			TotalLogs:    errorRates.TotalLogs,
			ErrorCount:   errorRates.ErrorCount,
			WarningCount: errorRates.WarningCount,
			FatalCount:   errorRates.FatalCount,
			ErrorRate:    errorRates.ErrorRate,
		},
		TopSources: make([]*SourceResponse, len(topSources)),
		Volume:     make([]*VolumeResponse, len(volume)),
	}

	for i, src := range topSources {
		resp.TopSources[i] = &SourceResponse{
			Source:     src.Source,
			Count:      src.Count,
			ErrorCount: src.ErrorCount,
		}
	}

	for i, vol := range volume {
		resp.Volume[i] = &VolumeResponse{
			Timestamp:  vol.Timestamp.Format(time.RFC3339),
			TotalCount: vol.TotalCount,
			ErrorCount: vol.ErrorCount,
		}
	}

	// Add HTTP stats if available
	if httpStats != nil {
		resp.HTTPStats = &HTTPStatsResponse{
			Total2xx: httpStats.Total2xx,
			Total3xx: httpStats.Total3xx,
			Total4xx: httpStats.Total4xx,
			Total5xx: httpStats.Total5xx,
		}
		if len(httpStats.TopURIs) > 0 {
			resp.HTTPStats.TopURIs = make([]*URIResponse, len(httpStats.TopURIs))
			for i, uri := range httpStats.TopURIs {
				resp.HTTPStats.TopURIs[i] = &URIResponse{
					URI:   uri.URI,
					Count: uri.Count,
				}
			}
		}
	}

	jsonOK(w, resp)
}

// Stream handles GET /api/v1/logs/stream - SSE streaming of logs.
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	if h.logStorage == nil {
		jsonError(w, http.StatusServiceUnavailable, errCodeInternalError, "log storage not configured")
		return
	}

	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "streaming not supported")
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse start time (default: last 5 minutes)
	startTime := time.Now().Add(-5 * time.Minute)
	if startStr := q.Get("start"); startStr != "" {
		var err error
		startTime, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid start time format (use RFC3339)")
			return
		}
	}

	// Parse search mode
	searchMode := storage.SearchModeToken
	if modeStr := q.Get("search_mode"); modeStr != "" {
		switch strings.ToLower(modeStr) {
		case "token":
			searchMode = storage.SearchModeToken
		case "substring":
			searchMode = storage.SearchModeSubstring
		case "phrase":
			searchMode = storage.SearchModePhrase
		default:
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "search_mode must be token, substring, or phrase")
			return
		}
	}

	// Parse levels
	var levels []string
	if levelsStr := q.Get("levels"); levelsStr != "" {
		levels = strings.Split(levelsStr, ",")
		for i := range levels {
			levels[i] = strings.TrimSpace(strings.ToLower(levels[i]))
		}
	}

	// Build base filter
	baseFilter := &storage.LogFilter{
		AgentID:         q.Get("agent_id"),
		Level:           strings.ToLower(q.Get("level")),
		Levels:          levels,
		Type:            strings.ToLower(q.Get("type")),
		Source:          q.Get("source"),
		MessageContains: q.Get("q"),
		SearchMode:      searchMode,
		Limit:           100,
		OrderBy:         "timestamp",
		OrderDesc:       false, // ASC for streaming
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Create SSE writer
	sse := NewSSEWriter(w, flusher)

	// Track last seen timestamp
	lastTimestamp := startTime

	// Polling interval
	pollInterval := time.Second

	// Heartbeat interval
	heartbeatInterval := 15 * time.Second
	lastHeartbeat := time.Now()

	// Stream timeout (30 minutes)
	streamTimeout := 30 * time.Minute
	deadline := time.Now().Add(streamTimeout)

	// Main loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return

		case <-ticker.C:
			// Check timeout
			if time.Now().After(deadline) {
				sse.SendEvent("close", `{"reason":"timeout"}`)
				return
			}

			// Build filter with time range
			filter := *baseFilter
			filter.StartTime = lastTimestamp
			filter.EndTime = time.Now()

			// Query for new logs
			result, err := h.logStorage.Logs().Query(ctx, &filter)
			if err != nil {
				log.Printf("stream query error: %v", err)
				continue
			}

			// Send new logs
			for _, entry := range result.Entries {
				// Skip if we've already sent this timestamp
				if !entry.Timestamp.After(lastTimestamp) && entry.Timestamp.Equal(lastTimestamp) {
					continue
				}

				resp := recordToResponse(entry)
				data, _ := json.Marshal(resp)
				if err := sse.SendEvent("log", string(data)); err != nil {
					return // Client disconnected
				}

				// Update last timestamp
				if entry.Timestamp.After(lastTimestamp) {
					lastTimestamp = entry.Timestamp
				}
			}

			// Send heartbeat if needed
			if time.Since(lastHeartbeat) >= heartbeatInterval {
				sse.SendEvent("heartbeat", `{"timestamp":"`+time.Now().Format(time.RFC3339)+`"}`)
				lastHeartbeat = time.Now()
			}
		}
	}
}

// recordToResponse converts a LogRecord to LogResponse.
func recordToResponse(r *storage.LogRecord) *LogResponse {
	resp := &LogResponse{
		ID:         r.ID,
		Timestamp:  r.Timestamp.Format(time.RFC3339),
		Level:      r.Level,
		Message:    r.Message,
		Source:     r.Source,
		Type:       r.Type,
		AgentID:    r.AgentID,
		FilePath:   r.FilePath,
		LineNumber: r.LineNumber,
		Fields:     r.Fields,
		Labels:     r.Labels,
	}

	// Only include HTTP fields if they have values
	if r.HTTPStatus > 0 {
		resp.HTTPStatus = r.HTTPStatus
	}
	if r.HTTPMethod != "" {
		resp.HTTPMethod = r.HTTPMethod
	}
	if r.URI != "" {
		resp.URI = r.URI
	}

	return resp
}
