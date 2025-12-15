package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

func TestShowAlerts_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/settings/alerts", nil)
	rec := httptest.NewRecorder()
	h.ShowAlerts(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("redirect location = %s, want /login", loc)
	}
}

func TestShowAlerts_WithSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/settings/alerts", nil)
	sess := &session.Session{Username: "test", Role: "viewer"}
	ctx := context.WithValue(req.Context(), SessionContextKey, sess)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ShowAlerts(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Alerts") {
		t.Error("response body missing alerts title")
	}
}

func TestShowProjects_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/settings/projects", nil)
	rec := httptest.NewRecorder()
	h.ShowProjects(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", rec.Code, http.StatusFound)
	}
}

func TestShowProjects_RequiresAdmin(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")

	tests := []struct {
		role     string
		wantCode int
	}{
		{"viewer", http.StatusFound},   // redirect to dashboard
		{"operator", http.StatusFound}, // redirect to dashboard
		{"admin", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/settings/projects", nil)
			sess := &session.Session{Username: "test", Role: tt.role}
			ctx := context.WithValue(req.Context(), SessionContextKey, sess)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			h.ShowProjects(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("role %s: status = %d, want %d", tt.role, rec.Code, tt.wantCode)
			}
		})
	}
}

func TestShowConnections_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/settings/connections", nil)
	rec := httptest.NewRecorder()
	h.ShowConnections(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", rec.Code, http.StatusFound)
	}
}

func TestShowConnections_RequiresAdmin(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")

	tests := []struct {
		role     string
		wantCode int
	}{
		{"viewer", http.StatusFound},
		{"operator", http.StatusFound},
		{"admin", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/settings/connections", nil)
			sess := &session.Session{Username: "test", Role: tt.role}
			ctx := context.WithValue(req.Context(), SessionContextKey, sess)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			h.ShowConnections(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("role %s: status = %d, want %d", tt.role, rec.Code, tt.wantCode)
			}
		})
	}
}

func TestShowUsers_RequiresSession(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")
	req := httptest.NewRequest("GET", "/settings/users", nil)
	rec := httptest.NewRecorder()
	h.ShowUsers(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", rec.Code, http.StatusFound)
	}
}

func TestShowUsers_RequiresAdmin(t *testing.T) {
	h := NewHandler(nil, nil, nil, "csrf")

	tests := []struct {
		role     string
		wantCode int
	}{
		{"viewer", http.StatusFound},
		{"operator", http.StatusFound},
		{"admin", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/settings/users", nil)
			sess := &session.Session{Username: "test", Role: tt.role}
			ctx := context.WithValue(req.Context(), SessionContextKey, sess)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			h.ShowUsers(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("role %s: status = %d, want %d", tt.role, rec.Code, tt.wantCode)
			}
		})
	}
}
