package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

func TestShowLogs_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	h.ShowLogs(rec, req)
	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", rec.Code, http.StatusFound)
	}
}

func TestShowLogs_WithSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ShowLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "fixed top-0 bottom-0 left-0") {
		t.Errorf("response missing fixed sidebar positioning classes")
	}

	infoItemsIndented := regexp.MustCompile(`\n\t{6}const infoItems = \[`)
	if !infoItemsIndented.MatchString(body) {
		t.Errorf("response missing properly indented infoItems block")
	}

	emptyAnchorBlock := regexp.MustCompile(`if \(isAnchor\) \{\s*\}`)
	if emptyAnchorBlock.MatchString(body) {
		t.Errorf("response contains empty isAnchor block")
	}
}

func TestGetLogsData_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs/data?start=2024-01-01T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	h.GetLogsData(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetLogsData_RequiresStartTime(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs/data", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.GetLogsData(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetLogsData_WithLogStorage(t *testing.T) {
	mock := &mockLogStorage{}
	h := NewHandler(nil, mock, nil, "csrf")

	req := httptest.NewRequest("GET", "/logs/data?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.GetLogsData(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %s, want application/json", ct)
	}
}

func TestExportLogs_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs/export?start=2024-01-01T00:00:00Z&format=json", nil)
	rec := httptest.NewRecorder()
	h.ExportLogs(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestExportLogs_JSON(t *testing.T) {
	mock := &mockLogStorage{}
	h := NewHandler(nil, mock, nil, "csrf")

	req := httptest.NewRequest("GET", "/logs/export?start=2024-01-01T00:00:00Z&format=json&limit=100", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ExportLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %s, want application/json", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("content-disposition = %s, want attachment", cd)
	}
}

func TestExportLogs_CSV(t *testing.T) {
	mock := &mockLogStorage{}
	h := NewHandler(nil, mock, nil, "csrf")

	req := httptest.NewRequest("GET", "/logs/export?start=2024-01-01T00:00:00Z&format=csv&limit=100", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ExportLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("content-type = %s, want text/csv", ct)
	}
}

func TestStreamLogs_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/logs/stream", nil)
	rec := httptest.NewRecorder()
	h.StreamLogs(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStreamLogs_SetsSSEHeaders(t *testing.T) {
	mock := &mockLogStorage{}
	h := NewHandler(nil, mock, nil, "csrf")

	req := httptest.NewRequest("GET", "/logs/stream?start=2024-01-01T00:00:00Z", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	// Cancel immediately to test headers
	cancel()
	h.StreamLogs(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %s, want text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("cache-control = %s, want no-cache", cc)
	}
}
