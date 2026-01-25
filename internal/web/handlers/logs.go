package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/query"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
	"github.com/gorilla/csrf"
	"golang.org/x/time/rate"
)

const maxFilterLength = 1000
const maxStreamDuration = 30 * time.Minute

// Export rate limiter: 10 requests per minute globally
// Note: rate.Limiter is already thread-safe, no mutex needed
var exportLimiter = rate.NewLimiter(rate.Every(6*time.Second), 10) // 10/min with burst of 10

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
	ProjectID  string                 `json:"project_id,omitempty"`
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

// ContextResponse wraps logs surrounding the anchor log.
type ContextResponse struct {
	Target        *LogItem   `json:"target"`
	Before        []*LogItem `json:"before"`
	After         []*LogItem `json:"after"`
	HasMoreBefore bool       `json:"has_more_before"`
	HasMoreAfter  bool       `json:"has_more_after"`
	BeforeCursor  string     `json:"before_cursor,omitempty"`
	AfterCursor   string     `json:"after_cursor,omitempty"`
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

	// Parse DSL filter expression
	var filterSQL string
	var filterArgs []any
	filterExpr := q.Get("filter")
	if len(filterExpr) > maxFilterLength {
		http.Error(w, fmt.Sprintf("filter too long (max %d chars)", maxFilterLength), http.StatusBadRequest)
		return
	}
	if filterExpr != "" {
		dsl := query.NewQueryDSL(query.DefaultFields)
		parsed, parseErr := dsl.Parse(filterExpr)
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("invalid filter: %v", parseErr), http.StatusBadRequest)
			return
		}

		builder := query.NewSQLBuilder(query.DefaultFields)
		result, buildErr := builder.Build(parsed)
		if buildErr != nil {
			http.Error(w, fmt.Sprintf("filter error: %v", buildErr), http.StatusBadRequest)
			return
		}
		filterSQL = result.SQL
		filterArgs = result.Args
	}

	// Build filter - DSL takes precedence over flat filters
	level := strings.ToLower(q.Get("level"))
	fileType := strings.ToLower(q.Get("type"))
	source := q.Get("source")
	messageContains := q.Get("q")
	projectID := q.Get("project_id")

	if filterExpr != "" {
		level = ""
		fileType = ""
		source = ""
		messageContains = ""
	}

	filter := &storage.LogFilter{
		StartTime:       startTime,
		EndTime:         endTime,
		Level:           level,
		Type:            fileType,
		Source:          source,
		MessageContains: messageContains,
		SearchMode:      searchMode,
		Limit:           perPage,
		Offset:          (page - 1) * perPage,
		OrderBy:         "timestamp",
		OrderDesc:       true,
		FilterExpr:      filterExpr,
		FilterSQL:       filterSQL,
		FilterArgs:      filterArgs,
	}

	// Apply project access filtering
	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		access, err := middleware.GetProjectAccess(ctx, sess.UserID, role, h.storage)
		if err != nil {
			http.Error(w, "failed to get project access", http.StatusInternalServerError)
			return
		}
		if err := access.ApplyToLogFilter(filter, projectID); err != nil {
			if errors.Is(err, middleware.ErrProjectAccessDenied) {
				http.Error(w, "no access to project", http.StatusForbidden)
				return
			}
			http.Error(w, "failed to apply project filter", http.StatusInternalServerError)
			return
		}
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func (h *Handler) Context(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.logStorage == nil {
		http.Error(w, "log storage not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "log id is required", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	before := parseIntDefault(q.Get("before"), 10)
	if before > 50 {
		before = 50
	}
	after := parseIntDefault(q.Get("after"), 10)
	if after > 50 {
		after = 50
	}
	beforeCursor := q.Get("before_cursor")
	afterCursor := q.Get("after_cursor")

	anchor, err := h.logStorage.Logs().GetByID(ctx, id)
	if err != nil {
		log.Printf("get log by id error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if anchor == nil {
		http.Error(w, "log not found", http.StatusNotFound)
		return
	}

	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		access, err := middleware.GetProjectAccess(ctx, sess.UserID, role, h.storage)
		if err != nil {
			log.Printf("project access error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !access.CanAccessProject(anchor.ProjectID) {
			http.Error(w, "log not found", http.StatusNotFound)
			return
		}
	}

	result, err := h.logStorage.Logs().GetContext(ctx, &storage.ContextFilter{
		TargetID:     id,
		ProjectID:    anchor.ProjectID,
		AgentID:      anchor.AgentID,
		Timestamp:    anchor.Timestamp,
		Before:       before,
		After:        after,
		BeforeCursor: beforeCursor,
		AfterCursor:  afterCursor,
	})
	if err != nil {
		log.Printf("get context error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if result == nil || result.Target == nil {
		http.Error(w, "log not found", http.StatusNotFound)
		return
	}

	resp := &ContextResponse{
		Target:        recordToLogItem(result.Target),
		Before:        make([]*LogItem, len(result.Before)),
		After:         make([]*LogItem, len(result.After)),
		HasMoreBefore: result.HasMoreBefore,
		HasMoreAfter:  result.HasMoreAfter,
		BeforeCursor:  result.BeforeCursor,
		AfterCursor:   result.AfterCursor,
	}

	for i, entry := range result.Before {
		resp.Before[i] = recordToLogItem(entry)
	}
	for i, entry := range result.After {
		resp.After[i] = recordToLogItem(entry)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func recordToLogItem(r *storage.LogRecord) *LogItem {
	item := &LogItem{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
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

// parseIntDefault parses an int from string, returning default if empty/invalid.
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}

func (h *Handler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Rate limit exports (10 per minute globally)
	if !exportLimiter.Allow() {
		http.Error(w, "export rate limit exceeded, try again later", http.StatusTooManyRequests)
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
	projectID := q.Get("project_id")
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

	// Apply project access filtering
	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		access, err := middleware.GetProjectAccess(ctx, sess.UserID, role, h.storage)
		if err != nil {
			http.Error(w, "failed to get project access", http.StatusInternalServerError)
			return
		}
		if err := access.ApplyToLogFilter(filter, projectID); err != nil {
			if errors.Is(err, middleware.ErrProjectAccessDenied) {
				http.Error(w, "no access to project", http.StatusForbidden)
				return
			}
			http.Error(w, "failed to apply project filter", http.StatusInternalServerError)
			return
		}
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
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func (h *Handler) writeCSV(w http.ResponseWriter, entries []*storage.LogRecord) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	writer.Write([]string{"timestamp", "project_id", "level", "source", "type", "message", "file_path", "http_status", "http_method", "uri"})

	// Rows
	for _, e := range entries {
		writer.Write([]string{
			e.Timestamp.Format(time.RFC3339),
			e.ProjectID,
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

// ProjectListItem represents a project for dropdown selection.
type ProjectListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetProjects returns accessible projects for the current user.
func (h *Handler) GetProjects(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	var rawProjects []*models.Project

	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		access, err := middleware.GetProjectAccess(ctx, sess.UserID, role, h.storage)
		if err != nil {
			http.Error(w, "failed to get project access", http.StatusInternalServerError)
			return
		}

		if access.AllProjects {
			rawProjects, err = h.storage.Projects().List(ctx)
		} else {
			rawProjects, err = h.storage.Projects().GetProjectsForUser(ctx, sess.UserID)
		}
		if err != nil {
			http.Error(w, "failed to get projects", http.StatusInternalServerError)
			return
		}
	}

	projects := make([]*ProjectListItem, len(rawProjects))
	for i, p := range rawProjects {
		projects[i] = &ProjectListItem{ID: p.ID, Name: p.Name}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		log.Printf("json encode error: %v", err)
		return
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
	projectID := q.Get("project_id")
	baseFilter := &storage.LogFilter{
		Level:           strings.ToLower(q.Get("level")),
		Type:            strings.ToLower(q.Get("type")),
		Source:          q.Get("source"),
		MessageContains: q.Get("q"),
		Limit:           100,
		OrderBy:         "timestamp",
		OrderDesc:       false, // ASC for streaming
	}

	// Apply project access filtering
	if h.storage != nil {
		role := models.ParseRole(sess.Role)
		access, err := middleware.GetProjectAccess(ctx, sess.UserID, role, h.storage)
		if err != nil {
			http.Error(w, "failed to get project access", http.StatusInternalServerError)
			return
		}
		if err := access.ApplyToLogFilter(baseFilter, projectID); err != nil {
			if errors.Is(err, middleware.ErrProjectAccessDenied) {
				http.Error(w, "no access to project", http.StatusForbidden)
				return
			}
			http.Error(w, "failed to apply project filter", http.StatusInternalServerError)
			return
		}
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

	// Max stream duration timeout
	streamTimeout := time.After(maxStreamDuration)

	for {
		select {
		case <-ctx.Done():
			return

		case <-streamTimeout:
			fmt.Fprintf(w, "event: timeout\ndata: {\"message\":\"stream max duration reached\"}\n\n")
			flusher.Flush()
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
				log.Printf("streaming logs query error: %v", err)
				continue
			}

			// Send new logs
			for _, entry := range result.Entries {
				if !entry.Timestamp.After(lastTimestamp) {
					continue
				}

				item := recordToLogItem(entry)
				data, err := json.Marshal(item)
				if err != nil {
					log.Printf("streaming logs marshal error: %v", err)
					continue
				}
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
