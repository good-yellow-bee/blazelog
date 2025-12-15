package projects

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
type mockProjectRepository struct {
	projects         []*models.Project
	members          []*models.ProjectMember
	getByIDError     error
	getByNameError   error
	createError      error
	updateError      error
	deleteError      error
	listError        error
	addUserError     error
	removeUserError  error
	getMembersError  error
}

func (m *mockProjectRepository) Create(ctx context.Context, project *models.Project) error {
	if m.createError != nil {
		return m.createError
	}
	m.projects = append(m.projects, project)
	return nil
}

func (m *mockProjectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	for _, p := range m.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockProjectRepository) GetByName(ctx context.Context, name string) (*models.Project, error) {
	if m.getByNameError != nil {
		return nil, m.getByNameError
	}
	for _, p := range m.projects {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockProjectRepository) Update(ctx context.Context, project *models.Project) error {
	if m.updateError != nil {
		return m.updateError
	}
	for i, p := range m.projects {
		if p.ID == project.ID {
			m.projects[i] = project
			return nil
		}
	}
	return nil
}

func (m *mockProjectRepository) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	for i, p := range m.projects {
		if p.ID == id {
			m.projects = append(m.projects[:i], m.projects[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockProjectRepository) List(ctx context.Context) ([]*models.Project, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.projects, nil
}

func (m *mockProjectRepository) AddUser(ctx context.Context, projectID, userID string, role models.Role) error {
	if m.addUserError != nil {
		return m.addUserError
	}
	return nil
}

func (m *mockProjectRepository) RemoveUser(ctx context.Context, projectID, userID string) error {
	if m.removeUserError != nil {
		return m.removeUserError
	}
	return nil
}

func (m *mockProjectRepository) GetUsers(ctx context.Context, projectID string) ([]*models.User, error) {
	return nil, nil
}

func (m *mockProjectRepository) GetProjectMembers(ctx context.Context, projectID string) ([]*models.ProjectMember, error) {
	if m.getMembersError != nil {
		return nil, m.getMembersError
	}
	// Return members for specific project (mock doesn't have ProjectID in member, filter externally)
	return m.members, nil
}

func (m *mockProjectRepository) GetProjectsForUser(ctx context.Context, userID string) ([]*models.Project, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	// For simplicity, return all projects in tests (real impl would filter)
	return m.projects, nil
}

type mockUserRepository struct {
	users        []*models.User
	getByIDError error
}

func (m *mockUserRepository) Create(ctx context.Context, user *models.User) error {
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRepository) Update(ctx context.Context, user *models.User) error {
	return nil
}

func (m *mockUserRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockUserRepository) List(ctx context.Context) ([]*models.User, error) {
	return m.users, nil
}

func (m *mockUserRepository) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

type mockStorage struct {
	projectRepo *mockProjectRepository
	userRepo    *mockUserRepository
}

func (m *mockStorage) Open() error                        { return nil }
func (m *mockStorage) Close() error                       { return nil }
func (m *mockStorage) Migrate() error                     { return nil }
func (m *mockStorage) EnsureAdminUser() error             { return nil }
func (m *mockStorage) Users() storage.UserRepository      { return m.userRepo }
func (m *mockStorage) Projects() storage.ProjectRepository { return m.projectRepo }
func (m *mockStorage) Alerts() storage.AlertRepository     { return nil }
func (m *mockStorage) Connections() storage.ConnectionRepository { return nil }
func (m *mockStorage) Tokens() storage.TokenRepository     { return nil }
func (m *mockStorage) AlertHistory() storage.AlertHistoryRepository { return nil }

func newMockStorage() (*mockStorage, *mockProjectRepository, *mockUserRepository) {
	projectRepo := &mockProjectRepository{}
	userRepo := &mockUserRepository{}
	return &mockStorage{
		projectRepo: projectRepo,
		userRepo:    userRepo,
	}, projectRepo, userRepo
}

type contextKey string

const (
	contextKeyRole   contextKey = "role"
	contextKeyUserID contextKey = "user_id"
)

func withRole(ctx context.Context, role models.Role, userID string) context.Context {
	ctx = context.WithValue(ctx, contextKeyRole, role)
	ctx = context.WithValue(ctx, contextKeyUserID, userID)
	return ctx
}

func TestList_AdminSeesAll(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Project 1", CreatedAt: now, UpdatedAt: now},
		{ID: "proj-2", Name: "Project 2", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req = req.WithContext(withRole(req.Context(), models.RoleAdmin, "admin-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("items count = %d, want 2", len(resp.Data))
	}
}

func TestList_NonAdminSeesOwn(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Project 1", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req = req.WithContext(withRole(req.Context(), models.RoleViewer, "user-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("items count = %d, want 1", len(resp.Data))
	}
}

func TestCreate_Success(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{"name": "New Project", "description": "Test description"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp struct {
		Data *ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Name != "New Project" {
		t.Errorf("name = %q, want 'New Project'", resp.Data.Name)
	}
}

func TestCreate_NameConflict(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Existing", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"name": "Existing"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestGetByID_Found(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test Project", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/projects/proj-1", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.ID != "proj-1" {
		t.Errorf("id = %q, want 'proj-1'", resp.Data.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent", nil)
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
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Original", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"name": "Updated Name"}`
	req := httptest.NewRequest("PUT", "/api/v1/projects/proj-1", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Name != "Updated Name" {
		t.Errorf("name = %q, want 'Updated Name'", resp.Data.Name)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{"name": "Updated"}`
	req := httptest.NewRequest("PUT", "/api/v1/projects/nonexistent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDelete_Success(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("DELETE", "/api/v1/projects/proj-1", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if len(mockRepo.projects) != 0 {
		t.Errorf("projects count = %d, want 0", len(mockRepo.projects))
	}
}

func TestDelete_NotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/nonexistent", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetUsers_Success(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test", CreatedAt: now, UpdatedAt: now},
	}
	mockRepo.members = []*models.ProjectMember{
		{UserID: "user-1", Username: "alice", Email: "alice@test.com", Role: models.RoleViewer},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/projects/proj-1/users", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*ProjectUserResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("members count = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].Username != "alice" {
		t.Errorf("username = %q, want 'alice'", resp.Data[0].Username)
	}
}

func TestGetUsers_ProjectNotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent/users", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUsers(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAddUser_Success(t *testing.T) {
	mockStore, mockRepo, mockUserRepo := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test", CreatedAt: now, UpdatedAt: now},
	}
	mockUserRepo.users = []*models.User{
		{ID: "user-1", Username: "alice", Email: "alice@test.com", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"user_id": "user-1", "role": "viewer"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.AddUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestAddUser_UserNotFound(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"user_id": "nonexistent", "role": "viewer"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.AddUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAddUser_ProjectNotFound(t *testing.T) {
	mockStore, _, mockUserRepo := newMockStorage()
	now := time.Now()
	mockUserRepo.users = []*models.User{
		{ID: "user-1", Username: "alice", Email: "alice@test.com", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	body := `{"user_id": "user-1", "role": "viewer"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/nonexistent/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.AddUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRemoveUser_Success(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.projects = []*models.Project{
		{ID: "proj-1", Name: "Test", CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("DELETE", "/api/v1/projects/proj-1/users/user-1", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	rctx.URLParams.Add("userId", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.RemoveUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
