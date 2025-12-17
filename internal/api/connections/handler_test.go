package connections

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Mock repositories
type mockConnectionRepository struct {
	connections       []*models.Connection
	getByIDError      error
	getByNameError    error
	createError       error
	updateError       error
	deleteError       error
	listError         error
	updateStatusError error
}

func (m *mockConnectionRepository) Create(ctx context.Context, conn *models.Connection) error {
	if m.createError != nil {
		return m.createError
	}
	m.connections = append(m.connections, conn)
	return nil
}

func (m *mockConnectionRepository) GetByID(ctx context.Context, id string) (*models.Connection, error) {
	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	for _, c := range m.connections {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockConnectionRepository) GetByName(ctx context.Context, name string) (*models.Connection, error) {
	if m.getByNameError != nil {
		return nil, m.getByNameError
	}
	for _, c := range m.connections {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockConnectionRepository) Update(ctx context.Context, conn *models.Connection) error {
	if m.updateError != nil {
		return m.updateError
	}
	for i, c := range m.connections {
		if c.ID == conn.ID {
			m.connections[i] = conn
			return nil
		}
	}
	return nil
}

func (m *mockConnectionRepository) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	for i, c := range m.connections {
		if c.ID == id {
			m.connections = append(m.connections[:i], m.connections[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockConnectionRepository) List(ctx context.Context) ([]*models.Connection, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.connections, nil
}

func (m *mockConnectionRepository) ListByProject(ctx context.Context, projectID string) ([]*models.Connection, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	var result []*models.Connection
	for _, c := range m.connections {
		if c.ProjectID == projectID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockConnectionRepository) UpdateStatus(ctx context.Context, id string, status models.ConnectionStatus, testedAt time.Time) error {
	if m.updateStatusError != nil {
		return m.updateStatusError
	}
	for _, c := range m.connections {
		if c.ID == id {
			c.Status = status
			c.LastTestedAt = &testedAt
			return nil
		}
	}
	return nil
}

func (m *mockConnectionRepository) EncryptCredentials(plaintext []byte) ([]byte, error) {
	return plaintext, nil // mock: return plaintext unchanged
}

func (m *mockConnectionRepository) DecryptCredentials(encrypted []byte) ([]byte, error) {
	return encrypted, nil // mock: return encrypted unchanged
}

type mockStorage struct {
	connRepo *mockConnectionRepository
}

func (m *mockStorage) Open() error                        { return nil }
func (m *mockStorage) Close() error                       { return nil }
func (m *mockStorage) Migrate() error                     { return nil }
func (m *mockStorage) EnsureAdminUser() error             { return nil }
func (m *mockStorage) Users() storage.UserRepository      { return nil }
func (m *mockStorage) Projects() storage.ProjectRepository { return nil }
func (m *mockStorage) Alerts() storage.AlertRepository     { return nil }
func (m *mockStorage) Connections() storage.ConnectionRepository { return m.connRepo }
func (m *mockStorage) Tokens() storage.TokenRepository     { return nil }
func (m *mockStorage) AlertHistory() storage.AlertHistoryRepository { return nil }

func newMockStorage() (*mockStorage, *mockConnectionRepository) {
	connRepo := &mockConnectionRepository{}
	return &mockStorage{connRepo: connRepo}, connRepo
}

func TestList_All(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "SSH Server 1", Type: models.ConnectionTypeSSH, Host: "server1.example.com", Port: 22, User: "admin", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
		{ID: "conn-2", Name: "Local Agent", Type: models.ConnectionTypeLocal, Status: models.ConnectionStatusConnected, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/connections", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("items count = %d, want 2", len(resp.Data))
	}
}

func TestList_WithProjectFilter(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Conn 1", ProjectID: "proj-1", Type: models.ConnectionTypeSSH, Host: "host1", Port: 22, User: "user", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
		{ID: "conn-2", Name: "Conn 2", ProjectID: "proj-2", Type: models.ConnectionTypeSSH, Host: "host2", Port: 22, User: "user", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/connections?project_id=proj-1", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("items count = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].ProjectID != "proj-1" {
		t.Errorf("project_id = %q, want 'proj-1'", resp.Data[0].ProjectID)
	}
}

func TestCreate_SSHConnection(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"name": "Production Server",
		"type": "ssh",
		"host": "prod.example.com",
		"port": 22,
		"user": "deploy"
	}`

	req := httptest.NewRequest("POST", "/api/v1/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp struct {
		Data *ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Name != "Production Server" {
		t.Errorf("name = %q, want 'Production Server'", resp.Data.Name)
	}
	if resp.Data.Type != "ssh" {
		t.Errorf("type = %q, want 'ssh'", resp.Data.Type)
	}
	if resp.Data.Host != "prod.example.com" {
		t.Errorf("host = %q, want 'prod.example.com'", resp.Data.Host)
	}
}

func TestCreate_LocalConnection(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"name": "Local Agent",
		"type": "local"
	}`

	req := httptest.NewRequest("POST", "/api/v1/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp struct {
		Data *ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Type != "local" {
		t.Errorf("type = %q, want 'local'", resp.Data.Type)
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"type": "ssh", "host": "server.com", "user": "admin"}`},
		{"invalid type", `{"name": "Test", "type": "invalid"}`},
		{"SSH missing host", `{"name": "Test", "type": "ssh", "user": "admin"}`},
		{"SSH missing user", `{"name": "Test", "type": "ssh", "host": "server.com"}`},
		{"invalid port", `{"name": "Test", "type": "ssh", "host": "server.com", "user": "admin", "port": 99999}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore, _ := newMockStorage()
			handler := NewHandler(mockStore)

			req := httptest.NewRequest("POST", "/api/v1/connections", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestCreate_NameConflict(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Existing", Type: models.ConnectionTypeLocal, Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"name": "Existing", "type": "local"}`
	req := httptest.NewRequest("POST", "/api/v1/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestGetByID_Found(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Test Connection", Type: models.ConnectionTypeSSH, Host: "server.com", Port: 22, User: "admin", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/connections/conn-1", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "conn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.ID != "conn-1" {
		t.Errorf("id = %q, want 'conn-1'", resp.Data.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("GET", "/api/v1/connections/nonexistent", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdate_Success(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Original", Type: models.ConnectionTypeSSH, Host: "old.com", Port: 22, User: "admin", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"name": "Updated Name", "host": "new.com"}`
	req := httptest.NewRequest("PUT", "/api/v1/connections/conn-1", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "conn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *ConnectionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Name != "Updated Name" {
		t.Errorf("name = %q, want 'Updated Name'", resp.Data.Name)
	}
	if resp.Data.Host != "new.com" {
		t.Errorf("host = %q, want 'new.com'", resp.Data.Host)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{"name": "Updated"}`
	req := httptest.NewRequest("PUT", "/api/v1/connections/nonexistent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdate_SSHValidationAfterUpdate(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "SSH", Type: models.ConnectionTypeSSH, Host: "server.com", Port: 22, User: "admin", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	// Update with invalid port, which should fail validation
	body := `{"port": 99999}`
	req := httptest.NewRequest("PUT", "/api/v1/connections/conn-1", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "conn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDelete_Success(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Test", Type: models.ConnectionTypeLocal, Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("DELETE", "/api/v1/connections/conn-1", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "conn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if len(mockRepo.connections) != 0 {
		t.Errorf("connections count = %d, want 0", len(mockRepo.connections))
	}
}

func TestDelete_NotFound(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("DELETE", "/api/v1/connections/nonexistent", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTest_Success(t *testing.T) {
	mockStore, mockRepo := newMockStorage()
	now := time.Now()
	mockRepo.connections = []*models.Connection{
		{ID: "conn-1", Name: "Test", Type: models.ConnectionTypeSSH, Host: "server.com", Port: 22, User: "admin", Status: models.ConnectionStatusUnknown, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("POST", "/api/v1/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "conn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Test(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *TestResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.Data.Success {
		t.Errorf("success = false, want true")
	}
	if resp.Data.Message == "" {
		t.Error("message is empty")
	}
}

func TestTest_NotFound(t *testing.T) {
	mockStore, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("POST", "/api/v1/connections/nonexistent/test", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Test(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
