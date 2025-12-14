package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func setAuthContext(r *http.Request, userID string, role models.Role) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, roleKey, role)
	return r.WithContext(ctx)
}

func TestRequireRole_Allowed(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name     string
		role     models.Role
		allowed  []models.Role
		wantCode int
	}{
		{"exact match", models.RoleAdmin, []models.Role{models.RoleAdmin}, http.StatusOK},
		{"one of many", models.RoleOperator, []models.Role{models.RoleAdmin, models.RoleOperator}, http.StatusOK},
		{"admin bypass", models.RoleAdmin, []models.Role{models.RoleViewer}, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := RequireRole(tc.allowed...)(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req = setAuthContext(req, "user-123", tc.role)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestRequireRole_Denied(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	tests := []struct {
		name    string
		role    models.Role
		allowed []models.Role
	}{
		{"viewer not admin", models.RoleViewer, []models.Role{models.RoleAdmin}},
		{"operator not admin", models.RoleOperator, []models.Role{models.RoleAdmin}},
		{"viewer not operator", models.RoleViewer, []models.Role{models.RoleOperator}},
		{"empty role", "", []models.Role{models.RoleViewer}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := RequireRole(tc.allowed...)(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			if tc.role != "" {
				req = setAuthContext(req, "user-123", tc.role)
			}
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		role     models.Role
		wantCode int
	}{
		{models.RoleAdmin, http.StatusOK},
		{models.RoleOperator, http.StatusForbidden},
		{models.RoleViewer, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			wrapped := RequireAdmin(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req = setAuthContext(req, "user-123", tc.role)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestRequireAdminOrSelf(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		userID     string
		role       models.Role
		resourceID string
		wantCode   int
	}{
		{"admin accessing other", "admin-1", models.RoleAdmin, "user-2", http.StatusOK},
		{"user accessing self", "user-1", models.RoleViewer, "user-1", http.StatusOK},
		{"operator accessing self", "user-1", models.RoleOperator, "user-1", http.StatusOK},
		{"viewer accessing other", "user-1", models.RoleViewer, "user-2", http.StatusForbidden},
		{"operator accessing other", "user-1", models.RoleOperator, "user-2", http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := RequireAdminOrSelf(handler)

			// Use chi router to set URL param
			router := chi.NewRouter()
			router.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r = setAuthContext(r, tc.userID, tc.role)
					next.ServeHTTP(w, r)
				})
			}).Get("/users/{id}", wrapped.ServeHTTP)

			req := httptest.NewRequest("GET", "/users/"+tc.resourceID, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestRequireCanWrite(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		role     models.Role
		wantCode int
	}{
		{models.RoleAdmin, http.StatusOK},
		{models.RoleOperator, http.StatusOK},
		{models.RoleViewer, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			wrapped := RequireCanWrite(handler)

			req := httptest.NewRequest("POST", "/test", nil)
			req = setAuthContext(req, "user-123", tc.role)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}
