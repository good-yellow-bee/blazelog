package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShowLogin_Success(t *testing.T) {
	h := NewHandler(nil, nil, "test-csrf-key")

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()

	h.ShowLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Sign in to BlazeLog") {
		t.Error("response body missing login title")
	}
}

func TestHandleLogin_MissingCredentials(t *testing.T) {
	h := NewHandler(nil, nil, "test-csrf-key")

	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
