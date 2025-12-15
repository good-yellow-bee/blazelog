package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

// benchServer creates a test server for benchmarking
func benchServer(b *testing.B) (*Server, storage.Storage, func()) {
	b.Helper()

	// Create temp DB
	tmpFile, err := os.CreateTemp("", "blazelog-bench-*.db")
	if err != nil {
		b.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	masterKey := []byte("test-master-key-32-bytes-long!!")
	store := storage.NewSQLiteStorage(tmpFile.Name(), masterKey)
	if err := store.Open(); err != nil {
		os.Remove(tmpFile.Name())
		b.Fatalf("open storage: %v", err)
	}
	if err := store.Migrate(); err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		b.Fatalf("migrate storage: %v", err)
	}

	cfg := &Config{
		Address:          ":0",
		JWTSecret:        []byte("test-jwt-secret-32-bytes-long!!"),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  24 * time.Hour,
		RateLimitPerIP:   10000, // High limit for benchmarks
		RateLimitPerUser: 10000,
		LockoutThreshold: 1000,
		LockoutDuration:  15 * time.Minute,
		Verbose:          false,
	}

	srv, err := New(cfg, store, nil)
	if err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		b.Fatalf("create server: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return srv, store, cleanup
}

// createBenchUser creates a user for benchmarking
func createBenchUser(b *testing.B, store storage.Storage, username, password string, role models.Role) *models.User {
	b.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		b.Fatalf("hash password: %v", err)
	}

	now := time.Now()
	user := &models.User{
		ID:           "bench-" + username,
		Username:     username,
		Email:        username + "@bench.com",
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.Users().Create(context.Background(), user); err != nil {
		b.Fatalf("create user: %v", err)
	}

	return user
}

// getAuthTokenBench gets a JWT token for benchmarking
func getAuthTokenBench(b *testing.B, ts *httptest.Server) string {
	b.Helper()

	loginBody := `{"username":"benchadmin","password":"benchpassword"}`
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		b.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Fatalf("login failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		b.Fatalf("decode response: %v", err)
	}

	return result.Data.AccessToken
}

// BenchmarkAPI_Health benchmarks the health endpoint
func BenchmarkAPI_Health(b *testing.B) {
	srv, _, cleanup := benchServer(b)
	defer cleanup()

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/health", nil)
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_HealthReady benchmarks the readiness endpoint
func BenchmarkAPI_HealthReady(b *testing.B) {
	srv, _, cleanup := benchServer(b)
	defer cleanup()

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/health/ready", nil)
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_Login benchmarks the login endpoint
func BenchmarkAPI_Login(b *testing.B) {
	srv, store, cleanup := benchServer(b)
	defer cleanup()

	// Create test user
	createBenchUser(b, store, "loginbench", "loginpassword", models.RoleAdmin)

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()
	loginBody := `{"username":"loginbench","password":"loginpassword"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/api/v1/auth/login", bytes.NewBufferString(loginBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_UsersList benchmarks the users list endpoint
func BenchmarkAPI_UsersList(b *testing.B) {
	srv, store, cleanup := benchServer(b)
	defer cleanup()

	// Create admin user
	createBenchUser(b, store, "benchadmin", "benchpassword", models.RoleAdmin)

	// Create test users
	for i := 0; i < 100; i++ {
		createBenchUser(b, store, fmt.Sprintf("user%d", i), "password", models.RoleViewer)
	}

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()
	token := getAuthTokenBench(b, ts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_ProjectsList benchmarks the projects list endpoint
func BenchmarkAPI_ProjectsList(b *testing.B) {
	srv, store, cleanup := benchServer(b)
	defer cleanup()

	// Create admin user
	createBenchUser(b, store, "benchadmin", "benchpassword", models.RoleAdmin)

	// Create test projects
	for i := 0; i < 50; i++ {
		project := &models.Project{
			ID:          fmt.Sprintf("proj-%d", i),
			Name:        fmt.Sprintf("project-%d", i),
			Description: fmt.Sprintf("Test project %d", i),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		store.Projects().Create(context.Background(), project)
	}

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()
	token := getAuthTokenBench(b, ts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_ConnectionsList benchmarks the connections list endpoint
func BenchmarkAPI_ConnectionsList(b *testing.B) {
	srv, store, cleanup := benchServer(b)
	defer cleanup()

	// Create admin user
	createBenchUser(b, store, "benchadmin", "benchpassword", models.RoleAdmin)

	// Create a project first
	project := &models.Project{
		ID:        "bench-project",
		Name:      "Benchmark Project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Projects().Create(context.Background(), project)

	// Create test connections
	for i := 0; i < 20; i++ {
		conn := &models.Connection{
			ID:        fmt.Sprintf("conn-%d", i),
			Name:      fmt.Sprintf("connection-%d", i),
			Type:      models.ConnectionTypeSSH,
			Host:      fmt.Sprintf("host%d.example.com", i),
			Port:      22,
			User:      "blazelog",
			ProjectID: project.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Connections().Create(context.Background(), conn)
	}

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()
	token := getAuthTokenBench(b, ts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/connections", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_Parallel tests parallel request handling
func BenchmarkAPI_Parallel(b *testing.B) {
	srv, store, cleanup := benchServer(b)
	defer cleanup()

	// Create admin user
	createBenchUser(b, store, "benchadmin", "benchpassword", models.RoleAdmin)

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	client := ts.Client()
	token := getAuthTokenBench(b, ts)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/health", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
