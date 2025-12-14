package users

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Response helpers (local to avoid import cycle)

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

// Error codes
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

// UserResponse is a user without sensitive fields.
type UserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Handler handles user management endpoints.
type Handler struct {
	storage storage.Storage
}

// NewHandler creates a new user handler.
func NewHandler(store storage.Storage) *Handler {
	return &Handler{storage: store}
}

// CreateRequest is the request body for creating a user.
type CreateRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateRequest is the request body for updating a user.
type UpdateRequest struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
}

// ChangePasswordRequest is the request body for changing password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// List returns all users (admin only).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	users, err := h.storage.Users().List(ctx)
	if err != nil {
		log.Printf("list users error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Convert to response format
	resp := make([]*UserResponse, len(users))
	for i, u := range users {
		resp[i] = userToResponse(u)
	}

	jsonOK(w, resp)
}

// Create creates a new user (admin only).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	// Validate fields
	if err := ValidateUsername(req.Username); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}
	if err := auth.ValidatePasswordOrError(req.Password); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}

	role, err := ValidateRole(req.Role)
	if err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}

	ctx := r.Context()

	// Check username uniqueness
	existing, err := h.storage.Users().GetByUsername(ctx, req.Username)
	if err != nil {
		log.Printf("create user error: check username: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if existing != nil {
		jsonError(w, http.StatusConflict, errCodeConflict, "username already exists")
		return
	}

	// Check email uniqueness
	existing, err = h.storage.Users().GetByEmail(ctx, req.Email)
	if err != nil {
		log.Printf("create user error: check email: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if existing != nil {
		jsonError(w, http.StatusConflict, errCodeConflict, "email already exists")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("create user error: hash password: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Create user
	now := time.Now()
	user := &models.User{
		ID:           uuid.New().String(),
		Username:     strings.TrimSpace(req.Username),
		Email:        strings.TrimSpace(req.Email),
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.storage.Users().Create(ctx, user); err != nil {
		log.Printf("create user error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("user created: %s (%s)", user.Username, user.ID)

	jsonCreated(w, userToResponse(user))
}

// GetByID returns a user by ID (admin or self).
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "user id required")
		return
	}

	ctx := r.Context()

	user, err := h.storage.Users().GetByID(ctx, userID)
	if err != nil {
		log.Printf("get user error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	jsonOK(w, userToResponse(user))
}

// Update updates a user (admin or self, role change admin only).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "user id required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	currentUserRole := middleware.GetRole(ctx)
	currentUserID := middleware.GetUserID(ctx)

	// Get existing user
	user, err := h.storage.Users().GetByID(ctx, userID)
	if err != nil {
		log.Printf("update user error: get user: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	// Update email if provided
	if req.Email != "" {
		if err := ValidateEmail(req.Email); err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}

		// Check email uniqueness
		existing, err := h.storage.Users().GetByEmail(ctx, req.Email)
		if err != nil {
			log.Printf("update user error: check email: %v", err)
			jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
			return
		}
		if existing != nil && existing.ID != userID {
			jsonError(w, http.StatusConflict, errCodeConflict, "email already exists")
			return
		}

		user.Email = strings.TrimSpace(req.Email)
	}

	// Update role if provided (admin only)
	if req.Role != "" {
		// Only admin can change roles
		if currentUserRole != models.RoleAdmin {
			jsonError(w, http.StatusForbidden, errCodeForbidden, "access denied")
			return
		}

		// Prevent admin from demoting themselves
		if userID == currentUserID && req.Role != "admin" {
			jsonError(w, http.StatusBadRequest, errCodeBadRequest, "cannot change own role")
			return
		}

		role, err := ValidateRole(req.Role)
		if err != nil {
			jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
			return
		}
		user.Role = role
	}

	user.UpdatedAt = time.Now()

	if err := h.storage.Users().Update(ctx, user); err != nil {
		log.Printf("update user error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("user updated: %s (%s)", user.Username, user.ID)

	jsonOK(w, userToResponse(user))
}

// Delete deletes a user (admin only).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "user id required")
		return
	}

	ctx := r.Context()
	currentUserID := middleware.GetUserID(ctx)

	// Prevent self-deletion
	if userID == currentUserID {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "cannot delete own account")
		return
	}

	// Check if user exists
	user, err := h.storage.Users().GetByID(ctx, userID)
	if err != nil {
		log.Printf("delete user error: get user: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	// Delete user
	if err := h.storage.Users().Delete(ctx, userID); err != nil {
		log.Printf("delete user error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("user deleted: %s (%s)", user.Username, user.ID)

	jsonNoContent(w)
}

// GetCurrentUser returns the current authenticated user.
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	user, err := h.storage.Users().GetByID(ctx, userID)
	if err != nil {
		log.Printf("get current user error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	jsonOK(w, userToResponse(user))
}

// ChangePassword changes the current user's password.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "current_password is required")
		return
	}
	if err := auth.ValidatePasswordOrError(req.NewPassword); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, err.Error())
		return
	}

	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	// Get user
	user, err := h.storage.Users().GetByID(ctx, userID)
	if err != nil {
		log.Printf("change password error: get user: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, errCodeNotFound, "user not found")
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeValidationFailed, "current password is incorrect")
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("change password error: hash password: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now()

	if err := h.storage.Users().Update(ctx, user); err != nil {
		log.Printf("change password error: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Revoke all refresh tokens (force re-login on other devices)
	if err := h.storage.Tokens().RevokeAllForUser(ctx, userID); err != nil {
		log.Printf("change password warning: revoke tokens: %v", err)
		// Don't fail the request, password was already changed
	}

	log.Printf("password changed: user %s", user.Username)

	jsonNoContent(w)
}

// userToResponse converts a User to UserResponse.
func userToResponse(u *models.User) *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
	}
}
