package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// testServer creates a test server with in-memory SQLite
func testServer(t *testing.T) (*Server, storage.Storage, func()) {
	t.Helper()

	// Create temp DB
	tmpFile, err := os.CreateTemp("", "blazelog-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	masterKey := []byte("test-master-key-32-bytes-long!!")
	dbKey := []byte("test-db-key-32-bytes-long!!!!!")
	store := storage.NewSQLiteStorage(tmpFile.Name(), masterKey, dbKey)
	if err := store.Open(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("open storage: %v", err)
	}
	if err := store.Migrate(); err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("migrate storage: %v", err)
	}

	cfg := &Config{
		Address:          ":0",
		JWTSecret:        []byte("test-jwt-secret-32-bytes-long!!"),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  24 * time.Hour,
		RateLimitPerIP:   100,
		RateLimitPerUser: 100,
		LockoutThreshold: 5,
		LockoutDuration:  30 * time.Minute,
		Verbose:          false,
	}

	srv, err := New(cfg, store, nil) // nil logStorage - ClickHouse not used in tests
	if err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("create server: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return srv, store, cleanup
}

// createTestUser creates a user in the database for testing
func createTestUser(t *testing.T, store storage.Storage, username, password string, role models.Role) *models.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	now := time.Now()
	user := &models.User{
		ID:           "test-" + username,
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.Users().Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

// handler returns the HTTP handler for the server
func handler(srv *Server) http.Handler {
	return srv.server.Handler
}

func TestHealthEndpoint(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogin_Success(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "testuser", "TestPassword123!", models.RoleViewer)

	body := `{"username":"testuser","password":"TestPassword123!"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.Data.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
	if resp.Data.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", resp.Data.TokenType)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "testuser", "TestPassword123!", models.RoleViewer)

	body := `{"username":"testuser","password":"wrongpassword"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	body := `{"username":"nonexistent","password":"password"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_Success(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "testuser", "TestPassword123!", models.RoleViewer)

	// First login
	loginBody := `{"username":"testuser","password":"TestPassword123!"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Refresh
	refreshBody := `{"refresh_token":"` + loginResp.Data.RefreshToken + `"}`
	refreshReq := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", refreshRec.Code, http.StatusOK, refreshRec.Body.String())
	}
}

func TestProtectedEndpoint_NoToken(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProtectedEndpoint_WithToken(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "testuser", "TestPassword123!", models.RoleViewer)

	// Login
	loginBody := `{"username":"testuser","password":"TestPassword123!"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Access protected endpoint
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAdminEndpoint_NonAdmin(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "viewer", "TestPassword123!", models.RoleViewer)

	// Login as viewer
	loginBody := `{"username":"viewer","password":"TestPassword123!"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Try to access admin endpoint
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdminEndpoint_Admin(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "admin", "TestPassword123!", models.RoleAdmin)

	// Login as admin
	loginBody := `{"username":"admin","password":"TestPassword123!"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Access admin endpoint
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	rec := httptest.NewRecorder()

	handler(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestLogout(t *testing.T) {
	srv, store, cleanup := testServer(t)
	defer cleanup()

	createTestUser(t, store, "testuser", "TestPassword123!", models.RoleViewer)

	// Login
	loginBody := `{"username":"testuser","password":"TestPassword123!"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Logout
	logoutBody := `{"refresh_token":"` + loginResp.Data.RefreshToken + `"}`
	logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", bytes.NewBufferString(logoutBody))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	logoutRec := httptest.NewRecorder()

	handler(srv).ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", logoutRec.Code, http.StatusNoContent)
	}

	// Try to refresh with revoked token
	refreshBody := `{"refresh_token":"` + loginResp.Data.RefreshToken + `"}`
	refreshReq := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()
	handler(srv).ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want %d", refreshRec.Code, http.StatusUnauthorized)
	}
}
