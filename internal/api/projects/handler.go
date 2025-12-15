package projects

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Response helpers (same pattern as alerts)
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
	errCodeInternalError    = "INTERNAL_ERROR"
)

func jsonError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: errorBody{Code: code, Message: message}})
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dataResponse{Data: data})
}

func jsonCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dataResponse{Data: data})
}

func jsonNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Response types
type ProjectResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ProjectUserResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type Handler struct {
	storage storage.Storage
}

func NewHandler(store storage.Storage) *Handler {
	return &Handler{storage: store}
}

// Request types
type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type AddUserRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// List returns all projects.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	role := middleware.GetRole(ctx)
	userID := middleware.GetUserID(ctx)

	var projects []*models.Project
	var err error

	if role == models.RoleAdmin {
		projects, err = h.storage.Projects().List(ctx)
	} else {
		projects, err = h.storage.Projects().GetProjectsForUser(ctx, userID)
	}

	if err != nil {
		log.Printf("list projects error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	resp := make([]*ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = projectToResponse(p)
	}
	jsonOK(w, resp)
}

// Create creates a new project (admin only).
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

	ctx := r.Context()

	// Check name uniqueness
	existing, err := h.storage.Projects().GetByName(ctx, req.Name)
	if err != nil {
		log.Printf("create project error: check name: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if existing != nil {
		jsonError(w, http.StatusConflict, errCodeConflict, "project name already exists")
		return
	}

	now := time.Now()
	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.storage.Projects().Create(ctx, project); err != nil {
		log.Printf("create project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("project created: %s (%s)", project.Name, project.ID)
	jsonCreated(w, projectToResponse(project))
}

// GetByID returns a project by ID.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id required")
		return
	}

	ctx := r.Context()
	project, err := h.storage.Projects().GetByID(ctx, id)
	if err != nil {
		log.Printf("get project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	jsonOK(w, projectToResponse(project))
}

// Update updates a project (admin only).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	project, err := h.storage.Projects().GetByID(ctx, id)
	if err != nil {
		log.Printf("update project error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	if req.Name != "" {
		if err := ValidateName(req.Name); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		// Check uniqueness
		existing, err := h.storage.Projects().GetByName(ctx, req.Name)
		if err != nil {
			log.Printf("update project error: check name: %v", err)
			jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
			return
		}
		if existing != nil && existing.ID != id {
			jsonError(w, http.StatusConflict, errCodeConflict, "project name already exists")
			return
		}
		project.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		project.Description = strings.TrimSpace(req.Description)
	}

	project.UpdatedAt = time.Now()

	if err := h.storage.Projects().Update(ctx, project); err != nil {
		log.Printf("update project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("project updated: %s (%s)", project.Name, project.ID)
	jsonOK(w, projectToResponse(project))
}

// Delete deletes a project (admin only).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id required")
		return
	}

	ctx := r.Context()
	project, err := h.storage.Projects().GetByID(ctx, id)
	if err != nil {
		log.Printf("delete project error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	if err := h.storage.Projects().Delete(ctx, id); err != nil {
		log.Printf("delete project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("project deleted: %s (%s)", project.Name, project.ID)
	jsonNoContent(w)
}

// GetUsers returns users in a project.
func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id required")
		return
	}

	ctx := r.Context()
	project, err := h.storage.Projects().GetByID(ctx, id)
	if err != nil {
		log.Printf("get project users error: get project: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	members, err := h.storage.Projects().GetProjectMembers(ctx, id)
	if err != nil {
		log.Printf("get project users error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	resp := make([]*ProjectUserResponse, len(members))
	for i, m := range members {
		resp[i] = &ProjectUserResponse{
			UserID:   m.UserID,
			Username: m.Username,
			Email:    m.Email,
			Role:     string(m.Role),
		}
	}
	jsonOK(w, resp)
}

// AddUser adds a user to a project (admin only).
func (h *Handler) AddUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id required")
		return
	}

	var req AddUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "user_id is required")
		return
	}

	role := models.RoleViewer
	if req.Role != "" {
		switch req.Role {
		case "admin", "operator", "viewer":
			role = models.Role(req.Role)
		default:
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "role must be admin, operator, or viewer")
			return
		}
	}

	ctx := r.Context()

	// Verify project exists
	project, err := h.storage.Projects().GetByID(ctx, id)
	if err != nil {
		log.Printf("add user to project error: get project: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	// Verify user exists
	user, err := h.storage.Users().GetByID(ctx, req.UserID)
	if err != nil {
		log.Printf("add user to project error: get user: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	if err := h.storage.Projects().AddUser(ctx, id, req.UserID, role); err != nil {
		log.Printf("add user to project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("user %s added to project %s with role %s", req.UserID, id, role)
	jsonNoContent(w)
}

// RemoveUser removes a user from a project (admin only).
func (h *Handler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if projectID == "" || userID == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "project id and user id required")
		return
	}

	ctx := r.Context()

	// Verify project exists
	project, err := h.storage.Projects().GetByID(ctx, projectID)
	if err != nil {
		log.Printf("remove user from project error: get project: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "project not found")
		return
	}

	if err := h.storage.Projects().RemoveUser(ctx, projectID, userID); err != nil {
		log.Printf("remove user from project error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("user %s removed from project %s", userID, projectID)
	jsonNoContent(w)
}

func projectToResponse(p *models.Project) *ProjectResponse {
	return &ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}
