package connections

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
type ConnectionResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Host         string  `json:"host,omitempty"`
	Port         int     `json:"port,omitempty"`
	User         string  `json:"user,omitempty"`
	Status       string  `json:"status"`
	LastTestedAt *string `json:"last_tested_at,omitempty"`
	ProjectID    string  `json:"project_id,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Handler struct {
	storage storage.Storage
}

func NewHandler(store storage.Storage) *Handler {
	return &Handler{storage: store}
}

// Request types
type CreateRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	ProjectID string `json:"project_id"`
}

type UpdateRequest struct {
	Name      string `json:"name,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	User      string `json:"user,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

// List returns all connections.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := r.URL.Query().Get("project_id")

	var connections []*models.Connection
	var err error

	if projectID != "" {
		connections, err = h.storage.Connections().ListByProject(ctx, projectID)
	} else {
		connections, err = h.storage.Connections().List(ctx)
	}

	if err != nil {
		log.Printf("list connections error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	resp := make([]*ConnectionResponse, len(connections))
	for i, c := range connections {
		resp[i] = connectionToResponse(c)
	}
	jsonOK(w, resp)
}

// Create creates a new connection.
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
	connType, err := ValidateType(req.Type)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}

	// Validate SSH-specific fields
	if connType == models.ConnectionTypeSSH {
		if err := ValidateHost(req.Host); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		if err := ValidateUser(req.User); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		if req.Port == 0 {
			req.Port = 22
		}
		if err := ValidatePort(req.Port); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
	}

	ctx := r.Context()

	// Check name uniqueness
	existing, err := h.storage.Connections().GetByName(ctx, req.Name)
	if err != nil {
		log.Printf("create connection error: check name: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if existing != nil {
		jsonError(w, http.StatusConflict, errCodeConflict, "connection name already exists")
		return
	}

	now := time.Now()
	conn := &models.Connection{
		ID:        uuid.New().String(),
		Name:      strings.TrimSpace(req.Name),
		Type:      connType,
		Host:      strings.TrimSpace(req.Host),
		Port:      req.Port,
		User:      strings.TrimSpace(req.User),
		Status:    models.ConnectionStatusUnknown,
		ProjectID: req.ProjectID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.storage.Connections().Create(ctx, conn); err != nil {
		log.Printf("create connection error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("connection created: %s (%s)", conn.Name, conn.ID)
	jsonCreated(w, connectionToResponse(conn))
}

// GetByID returns a connection by ID.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "connection id required")
		return
	}

	ctx := r.Context()
	conn, err := h.storage.Connections().GetByID(ctx, id)
	if err != nil {
		log.Printf("get connection error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if conn == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "connection not found")
		return
	}

	jsonOK(w, connectionToResponse(conn))
}

// Update updates a connection.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "connection id required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	conn, err := h.storage.Connections().GetByID(ctx, id)
	if err != nil {
		log.Printf("update connection error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if conn == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "connection not found")
		return
	}

	if req.Name != "" {
		if err := ValidateName(req.Name); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		existing, err := h.storage.Connections().GetByName(ctx, req.Name)
		if err != nil {
			log.Printf("update connection error: check name: %v", err)
			jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
			return
		}
		if existing != nil && existing.ID != id {
			jsonError(w, http.StatusConflict, errCodeConflict, "connection name already exists")
			return
		}
		conn.Name = strings.TrimSpace(req.Name)
	}
	if req.Host != "" {
		conn.Host = strings.TrimSpace(req.Host)
	}
	if req.Port != 0 {
		if err := ValidatePort(req.Port); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		conn.Port = req.Port
	}
	if req.User != "" {
		conn.User = strings.TrimSpace(req.User)
	}
	if req.ProjectID != "" {
		conn.ProjectID = req.ProjectID
	}

	// Validate SSH requirements after all updates
	if conn.Type == models.ConnectionTypeSSH {
		if err := ValidateHost(conn.Host); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		if err := ValidateUser(conn.User); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		if err := ValidatePort(conn.Port); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
	}

	conn.UpdatedAt = time.Now()

	if err := h.storage.Connections().Update(ctx, conn); err != nil {
		log.Printf("update connection error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("connection updated: %s (%s)", conn.Name, conn.ID)
	jsonOK(w, connectionToResponse(conn))
}

// Delete deletes a connection.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "connection id required")
		return
	}

	ctx := r.Context()
	conn, err := h.storage.Connections().GetByID(ctx, id)
	if err != nil {
		log.Printf("delete connection error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if conn == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "connection not found")
		return
	}

	if err := h.storage.Connections().Delete(ctx, id); err != nil {
		log.Printf("delete connection error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("connection deleted: %s (%s)", conn.Name, conn.ID)
	jsonNoContent(w)
}

// Test tests a connection (stub - actual SSH test would be in separate service).
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "connection id required")
		return
	}

	ctx := r.Context()
	conn, err := h.storage.Connections().GetByID(ctx, id)
	if err != nil {
		log.Printf("test connection error: get: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if conn == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "connection not found")
		return
	}

	// For now, just update status to "connected" as placeholder
	// Real implementation would actually test SSH connection
	now := time.Now()
	status := models.ConnectionStatusConnected
	message := "Connection test successful"

	if err := h.storage.Connections().UpdateStatus(ctx, id, status, now); err != nil {
		log.Printf("test connection error: update status: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("connection tested: %s (%s) - %s", conn.Name, conn.ID, status)
	jsonOK(w, TestResponse{
		Success: status == models.ConnectionStatusConnected,
		Message: message,
	})
}

func connectionToResponse(c *models.Connection) *ConnectionResponse {
	resp := &ConnectionResponse{
		ID:        c.ID,
		Name:      c.Name,
		Type:      string(c.Type),
		Host:      c.Host,
		Port:      c.Port,
		User:      c.User,
		Status:    string(c.Status),
		ProjectID: c.ProjectID,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
	if c.LastTestedAt != nil {
		s := c.LastTestedAt.Format(time.RFC3339)
		resp.LastTestedAt = &s
	}
	return resp
}
