package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	jwtService := auth.NewJWTService(secret, 15*time.Minute)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Role:     models.RoleAdmin,
	}

	token, err := jwtService.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Create handler that checks context values
	var gotUserID, gotUsername string
	var gotRole models.Role
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		gotUsername = GetUsername(r.Context())
		gotRole = GetRole(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrapped := JWTAuth(jwtService)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != user.ID {
		t.Errorf("UserID = %q, want %q", gotUserID, user.ID)
	}
	if gotUsername != user.Username {
		t.Errorf("Username = %q, want %q", gotUsername, user.Username)
	}
	if gotRole != user.Role {
		t.Errorf("Role = %q, want %q", gotRole, user.Role)
	}
}

func TestJWTAuth_MissingToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	jwtService := auth.NewJWTService(secret, 15*time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	wrapped := JWTAuth(jwtService)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	jwtService := auth.NewJWTService(secret, 15*time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	wrapped := JWTAuth(jwtService)(handler)

	tests := []struct {
		name   string
		header string
	}{
		{"invalid format", "NotBearer token"},
		{"invalid token", "Bearer invalid-token"},
		{"empty bearer", "Bearer "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	jwtService := auth.NewJWTService(secret, 1*time.Millisecond)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Role:     models.RoleViewer,
	}

	token, err := jwtService.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	wrapped := JWTAuth(jwtService)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestContextHelpers_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := req.Context()

	if got := GetUserID(ctx); got != "" {
		t.Errorf("GetUserID() = %q, want empty", got)
	}
	if got := GetUsername(ctx); got != "" {
		t.Errorf("GetUsername() = %q, want empty", got)
	}
	if got := GetRole(ctx); got != "" {
		t.Errorf("GetRole() = %q, want empty", got)
	}
	if got := GetClaims(ctx); got != nil {
		t.Errorf("GetClaims() = %v, want nil", got)
	}
}
