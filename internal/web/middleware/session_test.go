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
