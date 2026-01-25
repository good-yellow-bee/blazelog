package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// mockStorage implements storage.Storage for auth tests
type mockStorage struct {
	users *mockUserRepo
}

func (m *mockStorage) Open() error                  { return nil }
func (m *mockStorage) Close() error                 { return nil }
func (m *mockStorage) Migrate() error               { return nil }
func (m *mockStorage) EnsureAdminUser() error       { return nil }
func (m *mockStorage) Users() storage.UserRepository { return m.users }
func (m *mockStorage) Projects() storage.ProjectRepository { return nil }
func (m *mockStorage) Alerts() storage.AlertRepository { return nil }
func (m *mockStorage) Connections() storage.ConnectionRepository { return nil }
func (m *mockStorage) Tokens() storage.TokenRepository { return nil }
func (m *mockStorage) AlertHistory() storage.AlertHistoryRepository { return nil }

type mockUserRepo struct {
	user *models.User
}

func (r *mockUserRepo) Create(ctx context.Context, user *models.User) error { return nil }
func (r *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) { return r.user, nil }
func (r *mockUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) { return r.user, nil }
func (r *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) { return r.user, nil }
func (r *mockUserRepo) Update(ctx context.Context, user *models.User) error { return nil }
func (r *mockUserRepo) Delete(ctx context.Context, id string) error { return nil }
func (r *mockUserRepo) List(ctx context.Context) ([]*models.User, error) { return nil, nil }
func (r *mockUserRepo) Count(ctx context.Context) (int64, error) { return 0, nil }

func TestShowLogin_Success(t *testing.T) {
	h := NewHandler(nil, nil, nil, "test-csrf-key")

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
	h := NewHandler(nil, nil, nil, "test-csrf-key")

	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleLogin_NonHTMX_InvalidCredentials_ReturnsFullPage(t *testing.T) {
	mock := &mockStorage{users: &mockUserRepo{user: nil}} // nil user = not found
	h := NewHandler(mock, nil, nil, "test-csrf-key")

	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=test&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No HX-Request header = non-HTMX request
	rec := httptest.NewRecorder()

	h.HandleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	body := rec.Body.String()
	// Should contain full page markers
	if !strings.Contains(body, "Sign in to BlazeLog") {
		t.Error("non-HTMX response missing full page content")
	}
	// Should contain error message
	if !strings.Contains(body, "Invalid credentials") {
		t.Error("response missing error message")
	}
}

func TestHandleLogin_HTMX_InvalidCredentials_ReturnsAlertOnly(t *testing.T) {
	mock := &mockStorage{users: &mockUserRepo{user: nil}} // nil user = not found
	h := NewHandler(mock, nil, nil, "test-csrf-key")

	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=test&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true") // HTMX request
	rec := httptest.NewRecorder()

	h.HandleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	body := rec.Body.String()
	// Should contain alert with error
	if !strings.Contains(body, "Invalid credentials") {
		t.Error("HTMX response missing error message")
	}
	// Should NOT contain full page markers
	if strings.Contains(body, "Sign in to BlazeLog") {
		t.Error("HTMX response should not contain full page content")
	}
}
