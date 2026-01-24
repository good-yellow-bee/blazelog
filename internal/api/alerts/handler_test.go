package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Mock repositories
type mockAlertRepository struct {
	alerts       []*models.AlertRule
	getByIDError error
	createError  error
	updateError  error
	deleteError  error
	listError    error
}

func (m *mockAlertRepository) Create(ctx context.Context, alert *models.AlertRule) error {
	if m.createError != nil {
		return m.createError
	}
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *mockAlertRepository) GetByID(ctx context.Context, id string) (*models.AlertRule, error) {
	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	for _, a := range m.alerts {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAlertRepository) Update(ctx context.Context, alert *models.AlertRule) error {
	if m.updateError != nil {
		return m.updateError
	}
	for i, a := range m.alerts {
		if a.ID == alert.ID {
			m.alerts[i] = alert
			return nil
		}
	}
	return nil
}

func (m *mockAlertRepository) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	for i, a := range m.alerts {
		if a.ID == id {
			m.alerts = append(m.alerts[:i], m.alerts[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockAlertRepository) List(ctx context.Context) ([]*models.AlertRule, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.alerts, nil
}

func (m *mockAlertRepository) ListByProject(ctx context.Context, projectID string) ([]*models.AlertRule, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	var result []*models.AlertRule
	for _, a := range m.alerts {
		if a.ProjectID == projectID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockAlertRepository) ListEnabled(ctx context.Context) ([]*models.AlertRule, error) {
	return nil, nil
}

func (m *mockAlertRepository) SetEnabled(ctx context.Context, id string, enabled bool) error {
	return nil
}

type mockAlertHistoryRepository struct {
	histories []*models.AlertHistory
	total     int64
	listError error
}

func (m *mockAlertHistoryRepository) Create(ctx context.Context, history *models.AlertHistory) error {
	return nil
}

func (m *mockAlertHistoryRepository) List(ctx context.Context, limit, offset int) ([]*models.AlertHistory, int64, error) {
	if m.listError != nil {
		return nil, 0, m.listError
	}
	return m.histories, m.total, nil
}

func (m *mockAlertHistoryRepository) ListByAlert(ctx context.Context, alertID string, limit, offset int) ([]*models.AlertHistory, int64, error) {
	if m.listError != nil {
		return nil, 0, m.listError
	}
	var result []*models.AlertHistory
	for _, h := range m.histories {
		if h.AlertID == alertID {
			result = append(result, h)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockAlertHistoryRepository) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]*models.AlertHistory, int64, error) {
	if m.listError != nil {
		return nil, 0, m.listError
	}
	var result []*models.AlertHistory
	for _, h := range m.histories {
		if h.ProjectID == projectID {
			result = append(result, h)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockAlertHistoryRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

type mockProjectRepository struct{}

func (m *mockProjectRepository) Create(ctx context.Context, project *models.Project) error { return nil }
func (m *mockProjectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectRepository) GetByName(ctx context.Context, name string) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectRepository) List(ctx context.Context) ([]*models.Project, error) { return nil, nil }
func (m *mockProjectRepository) Update(ctx context.Context, project *models.Project) error { return nil }
func (m *mockProjectRepository) Delete(ctx context.Context, id string) error              { return nil }
func (m *mockProjectRepository) AddUser(ctx context.Context, projectID, userID string, role models.Role) error {
	return nil
}
func (m *mockProjectRepository) RemoveUser(ctx context.Context, projectID, userID string) error {
	return nil
}
func (m *mockProjectRepository) GetProjectsForUser(ctx context.Context, userID string) ([]*models.Project, error) {
	return []*models.Project{}, nil
}
func (m *mockProjectRepository) GetProjectMembers(ctx context.Context, projectID string) ([]*models.ProjectMember, error) {
	return nil, nil
}
func (m *mockProjectRepository) GetUsers(ctx context.Context, projectID string) ([]*models.User, error) {
	return nil, nil
}

type mockStorage struct {
	alertRepo        *mockAlertRepository
	alertHistoryRepo *mockAlertHistoryRepository
	projectRepo      *mockProjectRepository
}

func (m *mockStorage) Open() error                                    { return nil }
func (m *mockStorage) Close() error                                   { return nil }
func (m *mockStorage) Migrate() error                                 { return nil }
func (m *mockStorage) EnsureAdminUser() error                         { return nil }
func (m *mockStorage) Users() storage.UserRepository                  { return nil }
func (m *mockStorage) Projects() storage.ProjectRepository            { return m.projectRepo }
func (m *mockStorage) Alerts() storage.AlertRepository                { return m.alertRepo }
func (m *mockStorage) Connections() storage.ConnectionRepository      { return nil }
func (m *mockStorage) Tokens() storage.TokenRepository                { return nil }
func (m *mockStorage) AlertHistory() storage.AlertHistoryRepository   { return m.alertHistoryRepo }

func newMockStorage() (*mockStorage, *mockAlertRepository, *mockAlertHistoryRepository) {
	alertRepo := &mockAlertRepository{}
	historyRepo := &mockAlertHistoryRepository{}
	return &mockStorage{
		alertRepo:        alertRepo,
		alertHistoryRepo: historyRepo,
		projectRepo:      &mockProjectRepository{},
	}, alertRepo, historyRepo
}

func withAdminContext(r *http.Request) *http.Request {
	ctx := middleware.WithUserContext(r.Context(), "admin-user", "admin", models.RoleAdmin)
	return r.WithContext(ctx)
}

func TestList_Empty(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("GET", "/api/v1/alerts", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*AlertResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 0 {
		t.Errorf("items count = %d, want 0", len(resp.Data))
	}
}

func TestList_WithResults(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{
			ID:        "alert-1",
			Name:      "High Error Rate",
			Type:      models.AlertTypeThreshold,
			Condition: "error_rate > 5",
			Severity:  models.SeverityCritical,
			Window:    5 * time.Minute,
			Cooldown:  10 * time.Minute,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*AlertResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("items count = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].Name != "High Error Rate" {
		t.Errorf("name = %q, want 'High Error Rate'", resp.Data[0].Name)
	}
}

func TestList_WithProjectFilter(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{ID: "alert-1", Name: "Alert 1", ProjectID: "proj-1", Type: models.AlertTypeThreshold, Severity: models.SeverityMedium, Window: 5 * time.Minute, Cooldown: 10 * time.Minute, CreatedAt: now, UpdatedAt: now},
		{ID: "alert-2", Name: "Alert 2", ProjectID: "proj-2", Type: models.AlertTypeThreshold, Severity: models.SeverityMedium, Window: 5 * time.Minute, Cooldown: 10 * time.Minute, CreatedAt: now, UpdatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts?project_id=proj-1", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []*AlertResponse `json:"data"`
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

func TestCreate_Success(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"name": "Test Alert",
		"type": "threshold",
		"condition": "error_rate > 10",
		"severity": "medium",
		"window": "5m",
		"cooldown": "10m",
		"enabled": true
	}`

	req := httptest.NewRequest("POST", "/api/v1/alerts", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp struct {
		Data *AlertResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.Name != "Test Alert" {
		t.Errorf("name = %q, want 'Test Alert'", resp.Data.Name)
	}
	if resp.Data.Type != "threshold" {
		t.Errorf("type = %q, want 'threshold'", resp.Data.Type)
	}
}

func TestCreate_MissingName(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"type": "threshold",
		"condition": "error_rate > 10",
		"severity": "warning",
		"window": "5m",
		"cooldown": "10m"
	}`

	req := httptest.NewRequest("POST", "/api/v1/alerts", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreate_InvalidType(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"name": "Test Alert",
		"type": "invalid",
		"condition": "test",
		"severity": "medium",
		"window": "5m",
		"cooldown": "10m"
	}`

	req := httptest.NewRequest("POST", "/api/v1/alerts", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreate_InvalidWindow(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	body := `{
		"name": "Test Alert",
		"type": "threshold",
		"condition": "error_rate > 10",
		"severity": "medium",
		"window": "invalid",
		"cooldown": "10m"
	}`

	req := httptest.NewRequest("POST", "/api/v1/alerts", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetByID_Found(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{
			ID:        "alert-1",
			Name:      "Test Alert",
			Type:      models.AlertTypeThreshold,
			Severity:  models.SeverityMedium,
			Window:    5 * time.Minute,
			Cooldown:  10 * time.Minute,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts/alert-1", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "alert-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *AlertResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Data.ID != "alert-1" {
		t.Errorf("id = %q, want 'alert-1'", resp.Data.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("GET", "/api/v1/alerts/nonexistent", nil)
	req = withAdminContext(req)
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
	mockRepo.alerts = []*models.AlertRule{
		{
			ID:        "alert-1",
			Name:      "Original Name",
			Type:      models.AlertTypeThreshold,
			Severity:  models.SeverityMedium,
			Window:    5 * time.Minute,
			Cooldown:  10 * time.Minute,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	handler := NewHandler(mockStore)
	body := `{"name": "Updated Name"}`
	req := httptest.NewRequest("PUT", "/api/v1/alerts/alert-1", strings.NewReader(body))
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "alert-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data *AlertResponse `json:"data"`
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

	body := `{"name": "Updated Name"}`
	req := httptest.NewRequest("PUT", "/api/v1/alerts/nonexistent", strings.NewReader(body))
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdate_ValidationError(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{
			ID:        "alert-1",
			Name:      "Original",
			Type:      models.AlertTypeThreshold,
			Severity:  models.SeverityMedium,
			Window:    5 * time.Minute,
			Cooldown:  10 * time.Minute,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	handler := NewHandler(mockStore)
	// Send name with only spaces which becomes empty after trim
	body := `{"name": "   "}`
	req := httptest.NewRequest("PUT", "/api/v1/alerts/alert-1", strings.NewReader(body))
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "alert-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDelete_Success(t *testing.T) {
	mockStore, mockRepo, _ := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{
			ID:        "alert-1",
			Name:      "Test",
			Type:      models.AlertTypeThreshold,
			Severity:  models.SeverityMedium,
			Window:    5 * time.Minute,
			Cooldown:  10 * time.Minute,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("DELETE", "/api/v1/alerts/alert-1", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "alert-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if len(mockRepo.alerts) != 0 {
		t.Errorf("alerts count = %d, want 0", len(mockRepo.alerts))
	}
}

func TestDelete_NotFound(t *testing.T) {
	mockStore, _, _ := newMockStorage()
	handler := NewHandler(mockStore)

	req := httptest.NewRequest("DELETE", "/api/v1/alerts/nonexistent", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHistory_Pagination(t *testing.T) {
	mockStore, _, mockHistoryRepo := newMockStorage()
	now := time.Now()
	mockHistoryRepo.histories = []*models.AlertHistory{
		{ID: "h1", AlertID: "alert-1", AlertName: "Alert 1", Severity: models.SeverityMedium, Message: "Test", MatchedLogs: 10, NotifiedAt: now, CreatedAt: now},
		{ID: "h2", AlertID: "alert-1", AlertName: "Alert 1", Severity: models.SeverityMedium, Message: "Test", MatchedLogs: 15, NotifiedAt: now, CreatedAt: now},
	}
	mockHistoryRepo.total = 2

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts/history?page=1&per_page=10", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.History(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *HistoryListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data.Items) != 2 {
		t.Errorf("items count = %d, want 2", len(resp.Data.Items))
	}
	if resp.Data.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Data.Total)
	}
}

func TestHistory_AlertFilter(t *testing.T) {
	mockStore, mockRepo, mockHistoryRepo := newMockStorage()
	now := time.Now()
	mockRepo.alerts = []*models.AlertRule{
		{ID: "alert-1", Name: "Alert 1", Type: models.AlertTypeThreshold, Severity: models.SeverityMedium, Window: 5 * time.Minute, Cooldown: 10 * time.Minute, CreatedAt: now, UpdatedAt: now},
	}
	mockHistoryRepo.histories = []*models.AlertHistory{
		{ID: "h1", AlertID: "alert-1", AlertName: "Alert 1", Severity: models.SeverityMedium, Message: "Test", NotifiedAt: now, CreatedAt: now},
		{ID: "h2", AlertID: "alert-2", AlertName: "Alert 2", Severity: models.SeverityMedium, Message: "Test", NotifiedAt: now, CreatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts/history?alert_id=alert-1", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.History(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *HistoryListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data.Items) != 1 {
		t.Errorf("items count = %d, want 1", len(resp.Data.Items))
	}
	if resp.Data.Items[0].AlertID != "alert-1" {
		t.Errorf("alert_id = %q, want 'alert-1'", resp.Data.Items[0].AlertID)
	}
}

func TestHistory_ProjectFilter(t *testing.T) {
	mockStore, _, mockHistoryRepo := newMockStorage()
	now := time.Now()
	mockHistoryRepo.histories = []*models.AlertHistory{
		{ID: "h1", AlertID: "alert-1", ProjectID: "proj-1", AlertName: "Alert 1", Severity: models.SeverityMedium, Message: "Test", NotifiedAt: now, CreatedAt: now},
		{ID: "h2", AlertID: "alert-2", ProjectID: "proj-2", AlertName: "Alert 2", Severity: models.SeverityMedium, Message: "Test", NotifiedAt: now, CreatedAt: now},
	}

	handler := NewHandler(mockStore)
	req := httptest.NewRequest("GET", "/api/v1/alerts/history?project_id=proj-1", nil)
	req = withAdminContext(req)
	rec := httptest.NewRecorder()

	handler.History(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data *HistoryListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data.Items) != 1 {
		t.Errorf("items count = %d, want 1", len(resp.Data.Items))
	}
	if resp.Data.Items[0].ProjectID != "proj-1" {
		t.Errorf("project_id = %q, want 'proj-1'", resp.Data.Items[0].ProjectID)
	}
}
