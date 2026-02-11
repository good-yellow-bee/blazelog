package logs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// mockLogRepository implements storage.LogRepository for testing.
type mockLogRepository struct {
	entries       []*storage.LogRecord
	total         int64
	errorRates    *storage.ErrorRateResult
	topSources    []*storage.SourceCount
	volume        []*storage.VolumePoint
	httpStats     *storage.HTTPStatsResult
	queryError    error
	countError    error
	statsError    error
	lastFilter    *storage.LogFilter
	lastAggFilter *storage.AggregationFilter
	mu            sync.Mutex // protects lastAggFilter for concurrent Stats calls
}

func (m *mockLogRepository) InsertBatch(ctx context.Context, entries []*storage.LogRecord) error {
	return nil
}

func (m *mockLogRepository) Query(ctx context.Context, filter *storage.LogFilter) (*storage.LogQueryResult, error) {
	m.lastFilter = filter
	if m.queryError != nil {
		return nil, m.queryError
	}
	return &storage.LogQueryResult{
		Entries: m.entries,
		Total:   m.total,
		HasMore: int64(len(m.entries)) < m.total,
	}, nil
}

func (m *mockLogRepository) Count(ctx context.Context, filter *storage.LogFilter) (int64, error) {
	if m.countError != nil {
		return 0, m.countError
	}
	return m.total, nil
}

func (m *mockLogRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func (m *mockLogRepository) GetErrorRates(ctx context.Context, filter *storage.AggregationFilter) (*storage.ErrorRateResult, error) {
	m.mu.Lock()
	m.lastAggFilter = filter
	m.mu.Unlock()
	if m.statsError != nil {
		return nil, m.statsError
	}
	if m.errorRates == nil {
		return &storage.ErrorRateResult{}, nil
	}
	return m.errorRates, nil
}

func (m *mockLogRepository) GetTopSources(ctx context.Context, filter *storage.AggregationFilter, limit int) ([]*storage.SourceCount, error) {
	m.mu.Lock()
	m.lastAggFilter = filter
	m.mu.Unlock()
	if m.statsError != nil {
		return nil, m.statsError
	}
	return m.topSources, nil
}

func (m *mockLogRepository) GetLogVolume(ctx context.Context, filter *storage.AggregationFilter, interval string) ([]*storage.VolumePoint, error) {
	m.mu.Lock()
	m.lastAggFilter = filter
	m.mu.Unlock()
	if m.statsError != nil {
		return nil, m.statsError
	}
	return m.volume, nil
}

func (m *mockLogRepository) GetHTTPStats(ctx context.Context, filter *storage.AggregationFilter) (*storage.HTTPStatsResult, error) {
	m.mu.Lock()
	m.lastAggFilter = filter
	m.mu.Unlock()
	if m.statsError != nil {
		return nil, m.statsError
	}
	return m.httpStats, nil
}

func (m *mockLogRepository) GetByID(ctx context.Context, id string) (*storage.LogRecord, error) {
	return nil, nil
}

func (m *mockLogRepository) GetContext(ctx context.Context, filter *storage.ContextFilter) (*storage.ContextResult, error) {
	return &storage.ContextResult{}, nil
}

// mockLogStorage implements storage.LogStorage for testing.
type mockLogStorage struct {
	repo *mockLogRepository
}

func (m *mockLogStorage) Open() error                    { return nil }
func (m *mockLogStorage) Close() error                   { return nil }
func (m *mockLogStorage) Migrate() error                 { return nil }
func (m *mockLogStorage) Ping(ctx context.Context) error { return nil }
func (m *mockLogStorage) Logs() storage.LogRepository    { return m.repo }

func newMockLogStorage() (*mockLogStorage, *mockLogRepository) {
	repo := &mockLogRepository{}
	return &mockLogStorage{repo: repo}, repo
}

func TestQuery_Success(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	now := time.Now()

	mockRepo.entries = []*storage.LogRecord{
		{
			ID:        "log-1",
			Timestamp: now.Add(-time.Minute),
			Level:     "error",
			Message:   "Test error message",
			Source:    "nginx-access",
			Type:      "nginx",
			AgentID:   "agent-001",
			FilePath:  "/var/log/nginx/error.log",
			Fields:    map[string]interface{}{"status": float64(500)},
			Labels:    map[string]string{"env": "prod"},
		},
		{
			ID:        "log-2",
			Timestamp: now.Add(-2 * time.Minute),
			Level:     "info",
			Message:   "Request completed",
			Source:    "nginx-access",
			Type:      "nginx",
		},
	}
	mockRepo.total = 2

	handler := NewHandler(mockStorage)

	startTime := now.Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *ListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data == nil {
		t.Fatal("response data is nil")
	}
	if len(resp.Data.Items) != 2 {
		t.Errorf("items count = %d, want 2", len(resp.Data.Items))
	}
	if resp.Data.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Data.Total)
	}
	if resp.Data.Page != 1 {
		t.Errorf("page = %d, want 1", resp.Data.Page)
	}
	if resp.Data.PerPage != 50 {
		t.Errorf("per_page = %d, want 50", resp.Data.PerPage)
	}
}

func TestQuery_WithFilters(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.entries = []*storage.LogRecord{}
	mockRepo.total = 0

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)
	reqURL := "/api/v1/logs?start=" + url.QueryEscape(startTime) + "&end=" + url.QueryEscape(endTime) +
		"&level=error&type=nginx&source=web&agent_id=agent-1&q=database&search_mode=phrase" +
		"&page=2&per_page=25&order=level&order_dir=asc"

	req := httptest.NewRequest("GET", reqURL, nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify filter was passed correctly
	if mockRepo.lastFilter == nil {
		t.Fatal("filter was not set")
	}
	if mockRepo.lastFilter.Level != "error" {
		t.Errorf("filter.Level = %q, want %q", mockRepo.lastFilter.Level, "error")
	}
	if mockRepo.lastFilter.Type != "nginx" {
		t.Errorf("filter.Type = %q, want %q", mockRepo.lastFilter.Type, "nginx")
	}
	if mockRepo.lastFilter.Source != "web" {
		t.Errorf("filter.Source = %q, want %q", mockRepo.lastFilter.Source, "web")
	}
	if mockRepo.lastFilter.AgentID != "agent-1" {
		t.Errorf("filter.AgentID = %q, want %q", mockRepo.lastFilter.AgentID, "agent-1")
	}
	if mockRepo.lastFilter.MessageContains != "database" {
		t.Errorf("filter.MessageContains = %q, want %q", mockRepo.lastFilter.MessageContains, "database")
	}
	if mockRepo.lastFilter.SearchMode != storage.SearchModePhrase {
		t.Errorf("filter.SearchMode = %d, want %d", mockRepo.lastFilter.SearchMode, storage.SearchModePhrase)
	}
	if mockRepo.lastFilter.Limit != 25 {
		t.Errorf("filter.Limit = %d, want 25", mockRepo.lastFilter.Limit)
	}
	if mockRepo.lastFilter.Offset != 25 { // page 2 with per_page 25
		t.Errorf("filter.Offset = %d, want 25", mockRepo.lastFilter.Offset)
	}
	if mockRepo.lastFilter.OrderBy != "level" {
		t.Errorf("filter.OrderBy = %q, want %q", mockRepo.lastFilter.OrderBy, "level")
	}
	if mockRepo.lastFilter.OrderDesc {
		t.Error("filter.OrderDesc = true, want false")
	}
}

func TestQuery_MissingStartTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	req := httptest.NewRequest("GET", "/api/v1/logs", nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_InvalidStartTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	req := httptest.NewRequest("GET", "/api/v1/logs?start=invalid", nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_InvalidTimeRange(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	// End time before start time
	start := time.Now().Format(time.RFC3339)
	end := time.Now().Add(-time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest("GET", "/api/v1/logs?start="+start+"&end="+end, nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_InvalidPagination(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)
	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)

	tests := []struct {
		name  string
		query string
	}{
		{"invalid page", "page=abc"},
		{"page zero", "page=0"},
		{"negative page", "page=-1"},
		{"per_page zero", "per_page=0"},
		{"per_page too large", "per_page=2000"},
		{"invalid per_page", "per_page=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime)+"&"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler.Query(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestQuery_InvalidSearchMode(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)
	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime)+"&search_mode=invalid", nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_NoLogStorage(t *testing.T) {
	handler := NewHandler(nil)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestStats_Success(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	now := time.Now()

	mockRepo.errorRates = &storage.ErrorRateResult{
		TotalLogs:    1000,
		ErrorCount:   50,
		WarningCount: 100,
		FatalCount:   5,
		ErrorRate:    5.5,
	}
	mockRepo.topSources = []*storage.SourceCount{
		{Source: "nginx-access", Count: 500, ErrorCount: 20},
		{Source: "app-logs", Count: 300, ErrorCount: 30},
	}
	mockRepo.volume = []*storage.VolumePoint{
		{Timestamp: now.Add(-2 * time.Hour), TotalCount: 400, ErrorCount: 15},
		{Timestamp: now.Add(-time.Hour), TotalCount: 600, ErrorCount: 35},
	}
	mockRepo.httpStats = &storage.HTTPStatsResult{
		Total2xx: 800,
		Total3xx: 50,
		Total4xx: 100,
		Total5xx: 50,
		TopURIs:  []*storage.URICount{{URI: "/api/products", Count: 200}},
	}

	handler := NewHandler(mockStorage)

	startTime := now.Add(-23 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *StatsResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data == nil {
		t.Fatal("response data is nil")
	}
	if resp.Data.ErrorRates == nil {
		t.Fatal("error_rates is nil")
	}
	if resp.Data.ErrorRates.TotalLogs != 1000 {
		t.Errorf("total_logs = %d, want 1000", resp.Data.ErrorRates.TotalLogs)
	}
	if len(resp.Data.TopSources) != 2 {
		t.Errorf("top_sources count = %d, want 2", len(resp.Data.TopSources))
	}
	if len(resp.Data.Volume) != 2 {
		t.Errorf("volume count = %d, want 2", len(resp.Data.Volume))
	}
	if resp.Data.HTTPStats == nil {
		t.Fatal("http_stats is nil")
	}
	if resp.Data.HTTPStats.Total2xx != 800 {
		t.Errorf("total_2xx = %d, want 800", resp.Data.HTTPStats.Total2xx)
	}
}

func TestStats_MissingStartTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	req := httptest.NewRequest("GET", "/api/v1/logs/stats", nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStats_InvalidInterval(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime)+"&interval=invalid", nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStats_ExceedsMaxQueryRange(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.errorRates = &storage.ErrorRateResult{}

	handler := NewHandlerWithStorageAndConfig(mockStorage, nil, HandlerConfig{
		MaxQueryRange: 2 * time.Hour,
		QueryTimeout:  5 * time.Second,
	})

	startTime := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime)+"&end="+url.QueryEscape(endTime), nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStats_NoLogStorage(t *testing.T) {
	handler := NewHandler(nil)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestStream_NoLogStorage(t *testing.T) {
	handler := NewHandler(nil)

	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil)
	rec := httptest.NewRecorder()

	handler.Stream(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestStream_InvalidSearchMode(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	req := httptest.NewRequest("GET", "/api/v1/logs/stream?search_mode=invalid", nil)
	rec := httptest.NewRecorder()

	handler.Stream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStream_StartTooOld(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandlerWithStorageAndConfig(mockStorage, nil, HandlerConfig{
		MaxQueryRange: 10 * time.Minute,
	})

	start := time.Now().Add(-11 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stream?start="+url.QueryEscape(start), nil)
	rec := httptest.NewRecorder()

	handler.Stream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStream_SSEHeaders(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.entries = []*storage.LogRecord{}
	mockRepo.total = 0

	handler := NewHandler(mockStorage)

	// Create a request with context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	// Cancel context immediately to stop the stream
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	handler.Stream(rec, req)

	// Check SSE headers
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}
	if conn := rec.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want %q", conn, "keep-alive")
	}
}

func TestRecordToResponse(t *testing.T) {
	now := time.Now()
	record := &storage.LogRecord{
		ID:         "test-id",
		Timestamp:  now,
		Level:      "error",
		Message:    "Test message",
		Source:     "test-source",
		Type:       "nginx",
		AgentID:    "agent-001",
		FilePath:   "/var/log/test.log",
		LineNumber: 42,
		Fields:     map[string]interface{}{"key": "value"},
		Labels:     map[string]string{"env": "prod"},
		HTTPStatus: 500,
		HTTPMethod: "GET",
		URI:        "/api/test",
	}

	resp := recordToResponse(record)

	if resp.ID != "test-id" {
		t.Errorf("ID = %q, want %q", resp.ID, "test-id")
	}
	if resp.Timestamp != now.Format(time.RFC3339) {
		t.Errorf("Timestamp = %q, want %q", resp.Timestamp, now.Format(time.RFC3339))
	}
	if resp.Level != "error" {
		t.Errorf("Level = %q, want %q", resp.Level, "error")
	}
	if resp.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", resp.HTTPStatus)
	}
	if resp.HTTPMethod != "GET" {
		t.Errorf("HTTPMethod = %q, want %q", resp.HTTPMethod, "GET")
	}
	if resp.URI != "/api/test" {
		t.Errorf("URI = %q, want %q", resp.URI, "/api/test")
	}
}

func TestRecordToResponse_OmitsEmptyHTTPFields(t *testing.T) {
	record := &storage.LogRecord{
		ID:        "test-id",
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "Test",
		// HTTP fields intentionally empty
	}

	resp := recordToResponse(record)

	if resp.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0", resp.HTTPStatus)
	}
	if resp.HTTPMethod != "" {
		t.Errorf("HTTPMethod = %q, want empty", resp.HTTPMethod)
	}
	if resp.URI != "" {
		t.Errorf("URI = %q, want empty", resp.URI)
	}
}

func TestSSEWriter_SendEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	sse := NewSSEWriter(rec, rec)

	err := sse.SendEvent("log", `{"id":"test"}`)
	if err != nil {
		t.Errorf("SendEvent error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: log\n") {
		t.Errorf("body missing event line: %s", body)
	}
	if !strings.Contains(body, `data: {"id":"test"}`) {
		t.Errorf("body missing data line: %s", body)
	}
}

func TestSSEWriter_SendData(t *testing.T) {
	rec := httptest.NewRecorder()
	sse := NewSSEWriter(rec, rec)

	err := sse.SendData(`{"message":"test"}`)
	if err != nil {
		t.Errorf("SendData error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `data: {"message":"test"}`) {
		t.Errorf("body missing data: %s", body)
	}
}

func TestSSEWriter_SendComment(t *testing.T) {
	rec := httptest.NewRecorder()
	sse := NewSSEWriter(rec, rec)

	err := sse.SendComment("keepalive")
	if err != nil {
		t.Errorf("SendComment error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, ": keepalive\n") {
		t.Errorf("body missing comment: %s", body)
	}
}

func TestSSEWriter_SendRetry(t *testing.T) {
	rec := httptest.NewRecorder()
	sse := NewSSEWriter(rec, rec)

	err := sse.SendRetry(5000)
	if err != nil {
		t.Errorf("SendRetry error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "retry: 5000\n") {
		t.Errorf("body missing retry: %s", body)
	}
}

func TestQuery_StorageError(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.queryError = errors.New("storage unavailable")

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestStats_StorageError(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.statsError = errors.New("stats failed")

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestQuery_Timeout(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.queryError = context.DeadlineExceeded

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

func TestQuery_ExceedsMaxQueryRange(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.entries = []*storage.LogRecord{}
	mockRepo.total = 0

	handler := NewHandlerWithStorageAndConfig(mockStorage, nil, HandlerConfig{
		MaxQueryRange: 2 * time.Hour,
		QueryTimeout:  5 * time.Second,
	})

	startTime := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime)+"&end="+url.QueryEscape(endTime), nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStats_Timeout(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.statsError = context.DeadlineExceeded

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime), nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

func TestQuery_SearchModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode storage.SearchMode
	}{
		{"token mode", "token", storage.SearchModeToken},
		{"substring mode", "substring", storage.SearchModeSubstring},
		{"phrase mode", "phrase", storage.SearchModePhrase},
		{"empty defaults to token", "", storage.SearchModeToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage, mockRepo := newMockLogStorage()
			mockRepo.entries = []*storage.LogRecord{}
			mockRepo.total = 0

			handler := NewHandler(mockStorage)

			startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
			reqURL := "/api/v1/logs?start=" + url.QueryEscape(startTime)
			if tt.mode != "" {
				reqURL += "&search_mode=" + tt.mode
			}
			req := httptest.NewRequest("GET", reqURL, nil)
			rec := httptest.NewRecorder()

			handler.Query(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if mockRepo.lastFilter != nil && mockRepo.lastFilter.SearchMode != tt.wantMode {
				t.Errorf("SearchMode = %d, want %d", mockRepo.lastFilter.SearchMode, tt.wantMode)
			}
		})
	}
}

func TestQuery_OrderOptions(t *testing.T) {
	tests := []struct {
		name     string
		order    string
		orderDir string
	}{
		{"default order", "", ""},
		{"order by level asc", "level", "asc"},
		{"order by timestamp desc", "timestamp", "desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage, mockRepo := newMockLogStorage()
			mockRepo.entries = []*storage.LogRecord{}
			mockRepo.total = 0

			handler := NewHandler(mockStorage)

			startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
			reqURL := "/api/v1/logs?start=" + url.QueryEscape(startTime)
			if tt.order != "" {
				reqURL += "&order=" + tt.order
			}
			if tt.orderDir != "" {
				reqURL += "&order_dir=" + tt.orderDir
			}
			req := httptest.NewRequest("GET", reqURL, nil)
			rec := httptest.NewRecorder()

			handler.Query(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
		})
	}
}

func TestStats_Intervals(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{"minute interval", "minute"},
		{"hour interval", "hour"},
		{"day interval", "day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage, mockRepo := newMockLogStorage()
			mockRepo.errorRates = &storage.ErrorRateResult{TotalLogs: 100}
			mockRepo.topSources = []*storage.SourceCount{}
			mockRepo.volume = []*storage.VolumePoint{}
			mockRepo.httpStats = &storage.HTTPStatsResult{}

			handler := NewHandler(mockStorage)

			startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
			req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime)+"&interval="+tt.interval, nil)
			rec := httptest.NewRecorder()

			handler.Stats(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
		})
	}
}

func TestStream_WithStartTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stream?start="+url.QueryEscape(startTime), nil)

	// Create cancellable context
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Cancel quickly
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	handler.Stream(rec, req)

	// Should set SSE headers
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", rec.Header().Get("Content-Type"))
	}
}

func TestStream_WithFilters(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stream?start="+url.QueryEscape(startTime)+
		"&level=error&type=nginx&source=web&agent_id=agent-1&q=database&levels=error,warning", nil)

	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	handler.Stream(rec, req)

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", rec.Header().Get("Content-Type"))
	}
}

func TestStream_SearchModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"token", "token"},
		{"substring", "substring"},
		{"phrase", "phrase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage, _ := newMockLogStorage()
			handler := NewHandler(mockStorage)

			req := httptest.NewRequest("GET", "/api/v1/logs/stream?search_mode="+tt.mode, nil)

			ctx, cancel := context.WithCancel(req.Context())
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			handler.Stream(rec, req)

			if rec.Header().Get("Content-Type") != "text/event-stream" {
				t.Errorf("Content-Type = %q, want text/event-stream", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestStream_InvalidStartTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	req := httptest.NewRequest("GET", "/api/v1/logs/stream?start=invalid-time", nil)
	rec := httptest.NewRecorder()

	handler.Stream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStats_WithFilters(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.errorRates = &storage.ErrorRateResult{TotalLogs: 100}
	mockRepo.topSources = []*storage.SourceCount{}
	mockRepo.volume = []*storage.VolumePoint{}
	mockRepo.httpStats = &storage.HTTPStatsResult{}

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime)+
		"&end="+url.QueryEscape(endTime)+"&agent_id=agent-1&type=nginx", nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if mockRepo.lastAggFilter == nil {
		t.Fatal("aggregation filter was not set")
	}
	if mockRepo.lastAggFilter.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want agent-1", mockRepo.lastAggFilter.AgentID)
	}
	if mockRepo.lastAggFilter.Type != "nginx" {
		t.Errorf("Type = %q, want nginx", mockRepo.lastAggFilter.Type)
	}
}

func TestStats_InvalidEndTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs/stats?start="+url.QueryEscape(startTime)+"&end=invalid-time", nil)
	rec := httptest.NewRecorder()

	handler.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_InvalidEndTime(t *testing.T) {
	mockStorage, _ := newMockLogStorage()
	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime)+"&end=invalid-time", nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQuery_MultipleLevels(t *testing.T) {
	mockStorage, mockRepo := newMockLogStorage()
	mockRepo.entries = []*storage.LogRecord{}
	mockRepo.total = 0

	handler := NewHandler(mockStorage)

	startTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/logs?start="+url.QueryEscape(startTime)+"&levels=error,warning,fatal", nil)
	rec := httptest.NewRecorder()

	handler.Query(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if mockRepo.lastFilter == nil {
		t.Fatal("filter was not set")
	}
	if len(mockRepo.lastFilter.Levels) != 3 {
		t.Errorf("Levels count = %d, want 3", len(mockRepo.lastFilter.Levels))
	}
}
