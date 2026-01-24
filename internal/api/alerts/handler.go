package alerts

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Response helpers
type errorResponse struct {
	Error errorBody `json:"error"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type dataResponse struct {
	Data any `json:"data"`
}

const (
	errCodeBadRequest       = "BAD_REQUEST"
	errCodeValidationFailed = "VALIDATION_FAILED"
	errCodeNotFound         = "NOT_FOUND"
	errCodeConflict         = "CONFLICT"
	errCodeForbidden        = "FORBIDDEN"
	errCodeInternalError    = "INTERNAL_ERROR"
)

func jsonError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorResponse{Error: errorBody{Code: code, Message: message}}); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(dataResponse{Data: data}); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func jsonCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(dataResponse{Data: data}); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func jsonNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Response types
type AlertResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Condition   string   `json:"condition"`
	Severity    string   `json:"severity"`
	Window      string   `json:"window"`
	Cooldown    string   `json:"cooldown"`
	Notify      []string `json:"notify"`
	Enabled     bool     `json:"enabled"`
	ProjectID   string   `json:"project_id,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type AlertHistoryResponse struct {
	ID          string `json:"id"`
	AlertID     string `json:"alert_id"`
	AlertName   string `json:"alert_name"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	MatchedLogs int    `json:"matched_logs"`
	NotifiedAt  string `json:"notified_at"`
	ProjectID   string `json:"project_id,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type HistoryListResponse struct {
	Items   []*AlertHistoryResponse `json:"items"`
	Total   int64                   `json:"total"`
	Page    int                     `json:"page"`
	PerPage int                     `json:"per_page"`
}

// Handler handles alert endpoints.
type Handler struct {
	storage storage.Storage
}

func NewHandler(store storage.Storage) *Handler {
	return &Handler{storage: store}
}

// Request types
type CreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Condition   string   `json:"condition"`
	Severity    string   `json:"severity"`
	Window      string   `json:"window"`
	Cooldown    string   `json:"cooldown"`
	Notify      []string `json:"notify"`
	Enabled     bool     `json:"enabled"`
	ProjectID   string   `json:"project_id"`
}

type UpdateRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Condition   string   `json:"condition,omitempty"`
	Severity    string   `json:"severity,omitempty"`
	Window      string   `json:"window,omitempty"`
	Cooldown    string   `json:"cooldown,omitempty"`
	Notify      []string `json:"notify,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
}

// List returns all alerts.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := r.URL.Query().Get("project_id")
	userID := middleware.GetUserID(ctx)
	role := middleware.GetRole(ctx)

	access, err := middleware.GetProjectAccess(ctx, userID, role, h.storage)
	if err != nil {
		log.Printf("list alerts error: get access: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	if projectID != "" && !access.CanAccessProject(projectID) {
		jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
		return
	}

	var alerts []*models.AlertRule

	if projectID != "" {
		alerts, err = h.storage.Alerts().ListByProject(ctx, projectID)
	} else if access.AllProjects {
		alerts, err = h.storage.Alerts().List(ctx)
	} else {
		// Filter to user's accessible projects
		alerts = []*models.AlertRule{}
		for _, pid := range access.ProjectIDs {
			projectAlerts, pErr := h.storage.Alerts().ListByProject(ctx, pid)
			if pErr != nil {
				err = pErr
				break
			}
			alerts = append(alerts, projectAlerts...)
		}
		if access.IncludeUnassigned {
			unassigned, pErr := h.storage.Alerts().ListByProject(ctx, "")
			if pErr != nil && err == nil {
				err = pErr
			} else if pErr == nil {
				alerts = append(alerts, unassigned...)
			}
		}
	}

	if err != nil {
		log.Printf("list alerts error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	resp := make([]*AlertResponse, len(alerts))
	for i, a := range alerts {
		resp[i] = alertToResponse(a)
	}
	jsonOK(w, resp)
}

// Create creates a new alert.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	if err := ValidateName(req.Name); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}
	alertType, err := ValidateType(req.Type)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}
	severity, err := ValidateSeverity(req.Severity)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}
	if err := ValidateCondition(req.Condition); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}

	window, err := time.ParseDuration(req.Window)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "invalid window duration")
		return
	}
	cooldown, err := time.ParseDuration(req.Cooldown)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "invalid cooldown duration")
		return
	}

	ctx := r.Context()

	// Validate project exists if specified
	if req.ProjectID != "" {
		project, err := h.storage.Projects().GetByID(ctx, req.ProjectID)
		if err != nil {
			log.Printf("create alert error: check project: %v", err)
			jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
			return
		}
		if project == nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "project not found")
			return
		}
	}

	now := time.Now()
	alert := &models.AlertRule{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Type:        alertType,
		Condition:   req.Condition,
		Severity:    severity,
		Window:      window,
		Cooldown:    cooldown,
		Notify:      req.Notify,
		Enabled:     req.Enabled,
		ProjectID:   req.ProjectID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if alert.Notify == nil {
		alert.Notify = []string{}
	}

	if err := h.storage.Alerts().Create(ctx, alert); err != nil {
		log.Printf("create alert error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("alert created: %s (%s)", alert.Name, alert.ID)
	jsonCreated(w, alertToResponse(alert))
}

// GetByID returns an alert by ID.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "alert id required")
		return
	}

	ctx := r.Context()
	alert, err := h.storage.Alerts().GetByID(ctx, id)
	if err != nil {
		log.Printf("get alert error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if alert == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "alert not found")
		return
	}

	userID := middleware.GetUserID(ctx)
	role := middleware.GetRole(ctx)
	access, err := middleware.GetProjectAccess(ctx, userID, role, h.storage)
	if err != nil {
		log.Printf("get alert error: get access: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if !access.CanAccessProject(alert.ProjectID) {
		jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
		return
	}

	jsonOK(w, alertToResponse(alert))
}

// Update updates an alert.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "alert id required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	alert, err := h.storage.Alerts().GetByID(ctx, id)
	if err != nil {
		log.Printf("update alert error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if alert == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "alert not found")
		return
	}

	userID := middleware.GetUserID(ctx)
	role := middleware.GetRole(ctx)
	access, err := middleware.GetProjectAccess(ctx, userID, role, h.storage)
	if err != nil {
		log.Printf("update alert error: get access: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if !access.CanAccessProject(alert.ProjectID) {
		jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
		return
	}

	// Update fields if provided
	if req.Name != "" {
		if err := ValidateName(req.Name); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		alert.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		alert.Description = strings.TrimSpace(req.Description)
	}
	if req.Type != "" {
		alertType, err := ValidateType(req.Type)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		alert.Type = alertType
	}
	if req.Condition != "" {
		if err := ValidateCondition(req.Condition); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		alert.Condition = req.Condition
	}
	if req.Severity != "" {
		severity, err := ValidateSeverity(req.Severity)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		alert.Severity = severity
	}
	if req.Window != "" {
		window, err := time.ParseDuration(req.Window)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "invalid window duration")
			return
		}
		alert.Window = window
	}
	if req.Cooldown != "" {
		cooldown, err := time.ParseDuration(req.Cooldown)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "invalid cooldown duration")
			return
		}
		alert.Cooldown = cooldown
	}
	if req.Notify != nil {
		alert.Notify = req.Notify
	}
	if req.Enabled != nil {
		alert.Enabled = *req.Enabled
	}
	if req.ProjectID != "" {
		alert.ProjectID = req.ProjectID
	}

	alert.UpdatedAt = time.Now()

	if err := h.storage.Alerts().Update(ctx, alert); err != nil {
		log.Printf("update alert error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("alert updated: %s (%s)", alert.Name, alert.ID)
	jsonOK(w, alertToResponse(alert))
}

// Delete deletes an alert.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "alert id required")
		return
	}

	ctx := r.Context()
	alert, err := h.storage.Alerts().GetByID(ctx, id)
	if err != nil {
		log.Printf("delete alert error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if alert == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "alert not found")
		return
	}

	userID := middleware.GetUserID(ctx)
	role := middleware.GetRole(ctx)
	access, err := middleware.GetProjectAccess(ctx, userID, role, h.storage)
	if err != nil {
		log.Printf("delete alert error: get access: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if !access.CanAccessProject(alert.ProjectID) {
		jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
		return
	}

	if err := h.storage.Alerts().Delete(ctx, id); err != nil {
		log.Printf("delete alert error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("alert deleted: %s (%s)", alert.Name, alert.ID)
	jsonNoContent(w)
}

// History returns alert history with pagination.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertID := r.URL.Query().Get("alert_id")
	projectID := r.URL.Query().Get("project_id")
	userID := middleware.GetUserID(ctx)
	role := middleware.GetRole(ctx)

	access, err := middleware.GetProjectAccess(ctx, userID, role, h.storage)
	if err != nil {
		log.Printf("list alert history error: get access: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	if projectID != "" && !access.CanAccessProject(projectID) {
		jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
		return
	}

	// If filtering by alert_id, verify access to the alert's project
	if alertID != "" {
		alert, aErr := h.storage.Alerts().GetByID(ctx, alertID)
		if aErr != nil {
			log.Printf("list alert history error: get alert: %v", aErr)
			jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
			return
		}
		if alert == nil {
			jsonError(w, http.StatusNotFound, errCodeNotFound, "alert not found")
			return
		}
		if !access.CanAccessProject(alert.ProjectID) {
			jsonError(w, http.StatusForbidden, errCodeForbidden, "no access to project")
			return
		}
	}

	page := 1
	perPage := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
	}

	offset := (page - 1) * perPage
	var histories []*models.AlertHistory
	var total int64

	if alertID != "" {
		histories, total, err = h.storage.AlertHistory().ListByAlert(ctx, alertID, perPage, offset)
	} else if projectID != "" {
		histories, total, err = h.storage.AlertHistory().ListByProject(ctx, projectID, perPage, offset)
	} else if access.AllProjects {
		histories, total, err = h.storage.AlertHistory().List(ctx, perPage, offset)
	} else {
		// Non-admin without specific project filter: return empty or filter by accessible projects
		// For simplicity, require project_id filter for non-admins
		histories = []*models.AlertHistory{}
		total = 0
	}

	if err != nil {
		log.Printf("list alert history error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	items := make([]*AlertHistoryResponse, len(histories))
	for i, hist := range histories {
		items[i] = historyToResponse(hist)
	}

	jsonOK(w, HistoryListResponse{
		Items:   items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func alertToResponse(a *models.AlertRule) *AlertResponse {
	return &AlertResponse{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Type:        string(a.Type),
		Condition:   a.Condition,
		Severity:    string(a.Severity),
		Window:      a.Window.String(),
		Cooldown:    a.Cooldown.String(),
		Notify:      a.Notify,
		Enabled:     a.Enabled,
		ProjectID:   a.ProjectID,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
	}
}

func historyToResponse(h *models.AlertHistory) *AlertHistoryResponse {
	return &AlertHistoryResponse{
		ID:          h.ID,
		AlertID:     h.AlertID,
		AlertName:   h.AlertName,
		Severity:    string(h.Severity),
		Message:     h.Message,
		MatchedLogs: h.MatchedLogs,
		NotifiedAt:  h.NotifiedAt.Format(time.RFC3339),
		ProjectID:   h.ProjectID,
		CreatedAt:   h.CreatedAt.Format(time.RFC3339),
	}
}
