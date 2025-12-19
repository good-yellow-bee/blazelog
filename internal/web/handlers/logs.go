package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
	"github.com/gorilla/csrf"
)

func (h *Handler) ShowLogs(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	pages.Logs(sess, csrf.Token(r)).Render(r.Context(), w)
}

// LogsDataResponse wraps paginated log list for JSON response
type LogsDataResponse struct {
	Items      []*LogItem `json:"items"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalPages int        `json:"total_pages"`
}

// LogItem represents a log entry in web responses
type LogItem struct {
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

func (h *Handler) GetLogsData(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse required start time
	startStr := q.Get("start")
	if startStr == "" {
		http.Error(w, "start time is required", http.StatusBadRequest)
		return
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		http.Error(w, "invalid start time format", http.StatusBadRequest)
		return
	}

	// Parse end time (default: now)
	endTime := time.Now()
	if endStr := q.Get("end"); endStr != "" {
		endTime, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "invalid end time format", http.StatusBadRequest)
			return
		}
	}

	// Parse pagination
	page := 1
	if pageStr := q.Get("page"); pageStr != "" {
		page, _ = strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
	}

	perPage := 50
	if ppStr := q.Get("per_page"); ppStr != "" {
		perPage, _ = strconv.Atoi(ppStr)
		if perPage < 1 || perPage > 1000 {
			perPage = 50
		}
	}

	// Parse search mode
	searchMode := storage.SearchModeToken
	if modeStr := q.Get("search_mode"); modeStr != "" {
		switch strings.ToLower(modeStr) {
		case "substring":
			searchMode = storage.SearchModeSubstring
		case "phrase":
			searchMode = storage.SearchModePhrase
		}
	}

	// Build filter
	filter := &storage.LogFilter{
		StartTime:       startTime,
		EndTime:         endTime,
		Level:           strings.ToLower(q.Get("level")),
		Type:            strings.ToLower(q.Get("type")),
		Source:          q.Get("source"),
		MessageContains: q.Get("q"),
		SearchMode:      searchMode,
		Limit:           perPage,
		Offset:          (page - 1) * perPage,
		OrderBy:         "timestamp",
		OrderDesc:       true,
	}

	// Default response for nil storage
	response := &LogsDataResponse{
		Items:      []*LogItem{},
		Total:      0,
		Page:       page,
		PerPage:    perPage,
		TotalPages: 0,
	}

	if h.logStorage != nil {
		result, err := h.logStorage.Logs().Query(ctx, filter)
		if err == nil && result != nil {
			response.Total = result.Total
			response.TotalPages = int(math.Ceil(float64(result.Total) / float64(perPage)))
			response.Items = make([]*LogItem, len(result.Entries))
			for i, entry := range result.Entries {
				response.Items[i] = recordToLogItem(entry)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func recordToLogItem(r *storage.LogRecord) *LogItem {
	item := &LogItem{
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
	if r.HTTPStatus > 0 {
		item.HTTPStatus = r.HTTPStatus
	}
	if r.HTTPMethod != "" {
		item.HTTPMethod = r.HTTPMethod
	}
	if r.URI != "" {
		item.URI = r.URI
	}
	return item
}

func (h *Handler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse start time
	startStr := q.Get("start")
	if startStr == "" {
		http.Error(w, "start time is required", http.StatusBadRequest)
		return
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		http.Error(w, "invalid start time format", http.StatusBadRequest)
		return
	}

	// Parse end time
	endTime := time.Now()
	if endStr := q.Get("end"); endStr != "" {
		endTime, _ = time.Parse(time.RFC3339, endStr)
	}

	// Parse format
	format := q.Get("format")
	if format != "json" && format != "csv" {
		format = "json"
	}

	// Parse limit
	limit := 1000
	if limitStr := q.Get("limit"); limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
		if limit < 1 || limit > 10000 {
			limit = 1000
		}
	}

	// Build filter
	filter := &storage.LogFilter{
		StartTime:       startTime,
		EndTime:         endTime,
		Level:           strings.ToLower(q.Get("level")),
		Type:            strings.ToLower(q.Get("type")),
		Source:          q.Get("source"),
		MessageContains: q.Get("q"),
		Limit:           limit,
		OrderBy:         "timestamp",
		OrderDesc:       true,
	}

	// Query logs
	var entries []*storage.LogRecord
	if h.logStorage != nil {
		result, err := h.logStorage.Logs().Query(ctx, filter)
		if err == nil && result != nil {
			entries = result.Entries
		}
	}

	// Generate filename
	filename := fmt.Sprintf("logs-export-%s.%s", time.Now().Format("2006-01-02"), format)

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		h.writeCSV(w, entries)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		h.writeJSON(w, entries)
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, entries []*storage.LogRecord) {
	items := make([]*LogItem, len(entries))
	for i, entry := range entries {
		items[i] = recordToLogItem(entry)
	}
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) writeCSV(w http.ResponseWriter, entries []*storage.LogRecord) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	writer.Write([]string{"timestamp", "level", "source", "type", "message", "file_path", "http_status", "http_method", "uri"})

	// Rows
	for _, e := range entries {
		writer.Write([]string{
			e.Timestamp.Format(time.RFC3339),
			e.Level,
			e.Source,
			e.Type,
			e.Message,
			e.FilePath,
			strconv.Itoa(e.HTTPStatus),
			e.HTTPMethod,
			e.URI,
		})
	}
}

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	q := r.URL.Query()

	// Parse start time (default: last 5 minutes)
	startTime := time.Now().Add(-5 * time.Minute)
	if startStr := q.Get("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	// Build base filter
	baseFilter := &storage.LogFilter{
		Level:           strings.ToLower(q.Get("level")),
		Type:            strings.ToLower(q.Get("type")),
		Source:          q.Get("source"),
		MessageContains: q.Get("q"),
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

	// Track last timestamp
	lastTimestamp := startTime
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	heartbeatInterval := 15 * time.Second
	lastHeartbeat := time.Now()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if h.logStorage == nil {
				continue
			}

			// Query new logs
			filter := *baseFilter
			filter.StartTime = lastTimestamp
			filter.EndTime = time.Now()

			result, err := h.logStorage.Logs().Query(ctx, &filter)
			if err != nil {
				continue
			}

			// Send new logs
			for _, entry := range result.Entries {
				if !entry.Timestamp.After(lastTimestamp) {
					continue
				}

				item := recordToLogItem(entry)
				data, _ := json.Marshal(item)
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
				flusher.Flush()

				if entry.Timestamp.After(lastTimestamp) {
					lastTimestamp = entry.Timestamp
				}
			}

			// Heartbeat
			if time.Since(lastHeartbeat) >= heartbeatInterval {
				fmt.Fprintf(w, "event: heartbeat\ndata: {\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
				flusher.Flush()
				lastHeartbeat = time.Now()
			}
		}
	}
}
