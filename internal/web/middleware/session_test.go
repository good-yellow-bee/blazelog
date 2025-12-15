package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

func TestRequireSession_NoSession(t *testing.T) {
	store := session.NewStore(time.Hour)
	mw := RequireSession(store)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("redirect = %q, want /login", loc)
	}
}

func TestRequireSession_ValidSession(t *testing.T) {
	store := session.NewStore(time.Hour)
	sess, _ := store.Create("user-1", "admin", "admin")

	mw := RequireSession(store)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sess.ID})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRole_NoSession(t *testing.T) {
	store := session.NewStore(time.Hour)
	mw := RequireRole(store, "admin")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect)", rec.Code, http.StatusFound)
	}
}

func TestRequireRole_Hierarchy(t *testing.T) {
	store := session.NewStore(time.Hour)

	tests := []struct {
		name         string
		userRole     string
		requiredRole string
		wantAllowed  bool
	}{
		{"admin can access admin", "admin", "admin", true},
		{"admin can access operator", "admin", "operator", true},
		{"admin can access viewer", "admin", "viewer", true},
		{"operator can access operator", "operator", "operator", true},
		{"operator can access viewer", "operator", "viewer", true},
		{"operator cannot access admin", "operator", "admin", false},
		{"viewer can access viewer", "viewer", "viewer", true},
		{"viewer cannot access operator", "viewer", "operator", false},
		{"viewer cannot access admin", "viewer", "admin", false},
		{"unknown role denied", "unknown", "viewer", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, _ := store.Create("user-1", "test", tt.userRole)

			// First apply RequireSession to add session to context
			sessionMW := RequireSession(store)
			roleMW := RequireRole(store, tt.requiredRole)

			called := false
			handler := sessionMW(roleMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})))

			req := httptest.NewRequest("GET", "/test", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sess.ID})
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.wantAllowed && !called {
				t.Errorf("handler not called, want allowed")
			}
			if !tt.wantAllowed && called {
				t.Errorf("handler called, want denied")
			}
			if tt.wantAllowed && rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if !tt.wantAllowed && rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}

			// Clean up session
			store.Delete(sess.ID)
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		userRole     string
		requiredRole string
		want         bool
	}{
		{"admin", "admin", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"operator", "admin", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
		{"invalid", "viewer", false},
		{"viewer", "invalid", false},
	}

	for _, tt := range tests {
		name := tt.userRole + "_requires_" + tt.requiredRole
		t.Run(name, func(t *testing.T) {
			got := hasRole(tt.userRole, tt.requiredRole)
			if got != tt.want {
				t.Errorf("hasRole(%q, %q) = %v, want %v", tt.userRole, tt.requiredRole, got, tt.want)
			}
		})
	}
}
