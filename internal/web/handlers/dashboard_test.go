package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

// mockLogStorage implements storage.LogStorage for testing
type mockLogStorage struct {
	errorRates *storage.ErrorRateResult
	topSources []*storage.SourceCount
	volume     []*storage.VolumePoint
	httpStats  *storage.HTTPStatsResult
}

func (m *mockLogStorage) Open() error                         { return nil }
func (m *mockLogStorage) Close() error                        { return nil }
func (m *mockLogStorage) Migrate() error                      { return nil }
func (m *mockLogStorage) Ping(ctx context.Context) error      { return nil }
func (m *mockLogStorage) Logs() storage.LogRepository         { return &mockLogRepo{mock: m} }

type mockLogRepo struct {
	mock *mockLogStorage
}

func (r *mockLogRepo) InsertBatch(ctx context.Context, entries []*storage.LogRecord) error {
	return nil
}

func (r *mockLogRepo) Query(ctx context.Context, filter *storage.LogFilter) (*storage.LogQueryResult, error) {
	return &storage.LogQueryResult{}, nil
}

func (r *mockLogRepo) Count(ctx context.Context, filter *storage.LogFilter) (int64, error) {
	return 0, nil
}

func (r *mockLogRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func (r *mockLogRepo) GetErrorRates(ctx context.Context, filter *storage.AggregationFilter) (*storage.ErrorRateResult, error) {
	if r.mock.errorRates != nil {
		return r.mock.errorRates, nil
	}
	return &storage.ErrorRateResult{}, nil
}

func (r *mockLogRepo) GetTopSources(ctx context.Context, filter *storage.AggregationFilter, limit int) ([]*storage.SourceCount, error) {
	return r.mock.topSources, nil
}

func (r *mockLogRepo) GetLogVolume(ctx context.Context, filter *storage.AggregationFilter, interval string) ([]*storage.VolumePoint, error) {
	return r.mock.volume, nil
}

func (r *mockLogRepo) GetHTTPStats(ctx context.Context, filter *storage.AggregationFilter) (*storage.HTTPStatsResult, error) {
	return r.mock.httpStats, nil
}

func TestHandler_HasLogStorage(t *testing.T) {
	// Test that handler can be created with nil logStorage
	h := NewHandler(nil, nil, nil, "csrf")
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestHandler_WithLogStorage(t *testing.T) {
	mock := &mockLogStorage{
		errorRates: &storage.ErrorRateResult{
			TotalLogs:    1000,
			ErrorCount:   50,
			WarningCount: 100,
			ErrorRate:    0.05,
		},
	}
	h := NewHandler(nil, mock, nil, "csrf")
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestGetDashboardStats_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	h.GetDashboardStats(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetDashboardStats_WithSession(t *testing.T) {
	mock := &mockLogStorage{
		errorRates: &storage.ErrorRateResult{
			TotalLogs:    1000,
			ErrorCount:   50,
			WarningCount: 100,
			ErrorRate:    0.05,
		},
	}
	h := NewHandler(nil, mock, nil, "csrf")

	req := httptest.NewRequest("GET", "/dashboard/stats?range=24h", nil)
	sess := &session.Session{Username: "test", Role: "admin"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.GetDashboardStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %s, want application/json", ct)
	}
}

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		input       string
		wantDur     time.Duration
		wantInterval string
	}{
		{"15m", 15 * time.Minute, "minute"},
		{"1h", time.Hour, "minute"},
		{"6h", 6 * time.Hour, "hour"},
		{"24h", 24 * time.Hour, "hour"},
		{"7d", 7 * 24 * time.Hour, "day"},
		{"30d", 30 * 24 * time.Hour, "day"},
		{"invalid", 24 * time.Hour, "hour"}, // default
		{"", 24 * time.Hour, "hour"},        // empty default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end, interval := parseTimeRange(tt.input)
			gotDur := end.Sub(start)

			// Allow 2 second tolerance for timing
			if gotDur < tt.wantDur-2*time.Second || gotDur > tt.wantDur+2*time.Second {
				t.Errorf("duration = %v, want %v", gotDur, tt.wantDur)
			}
			if interval != tt.wantInterval {
				t.Errorf("interval = %s, want %s", interval, tt.wantInterval)
			}
		})
	}
}
