package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
}
