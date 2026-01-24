package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// Handler handles authentication endpoints.
type Handler struct {
	storage        storage.Storage
	jwtService     *JWTService
	tokenService   *TokenService
	lockoutTracker *LockoutTracker
}

// NewHandler creates a new auth handler.
func NewHandler(store storage.Storage, jwt *JWTService, lockout *LockoutTracker, refreshTTL time.Duration) *Handler {
	return &Handler{
		storage:        store,
		jwtService:     jwt,
		tokenService:   NewTokenService(store, refreshTTL),
		lockoutTracker: lockout,
	}
}

// Response helpers (local to avoid import cycle with api package)

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

func jsonNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// LoginResponse is returned on successful login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Error codes and messages
const (
	errCodeBadRequest    = "BAD_REQUEST"
	errCodeUnauthorized  = "UNAUTHORIZED"
	errCodeAccountLocked = "ACCOUNT_LOCKED"
	errCodeInternalError = "INTERNAL_ERROR"
)

// LoginRequest is the request body for login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RefreshRequest is the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the request body for logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Login handles user login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "username and password required")
		return
	}

	// Check lockout
	if h.lockoutTracker.IsLocked(req.Username) {
		remaining := h.lockoutTracker.RemainingLockoutTime(req.Username)
		log.Printf("login blocked: account %s locked for %v", req.Username, remaining)
		jsonError(w, http.StatusTooManyRequests, errCodeAccountLocked, "account temporarily locked due to too many failed attempts")
		return
	}

	// Get user
	ctx := r.Context()
	user, err := h.storage.Users().GetByUsername(ctx, req.Username)
	if err != nil {
		log.Printf("login error: get user: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}
	if user == nil {
		h.lockoutTracker.RecordFailure(req.Username)
		log.Printf("login failed: user %s not found", req.Username)
		jsonError(w, http.StatusUnauthorized, errCodeUnauthorized, "invalid credentials")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.lockoutTracker.RecordFailure(req.Username)
		log.Printf("login failed: invalid password for user %s", req.Username)
		jsonError(w, http.StatusUnauthorized, errCodeUnauthorized, "invalid credentials")
		return
	}

	// Clear lockout on success
	h.lockoutTracker.ClearFailures(req.Username)

	// Generate access token
	accessToken, err := h.jwtService.GenerateToken(user)
	if err != nil {
		log.Printf("login error: generate access token: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Generate refresh token
	refreshToken, err := h.tokenService.CreateRefreshToken(ctx, user.ID)
	if err != nil {
		log.Printf("login error: generate refresh token: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("login success: user %s", req.Username)

	jsonOK(w, &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.jwtService.TTLSeconds(),
		TokenType:    "Bearer",
	})
}

// Refresh handles token refresh.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "refresh_token required")
		return
	}

	ctx := r.Context()

	// Validate refresh token and get user
	user, err := h.tokenService.ValidateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		log.Printf("refresh failed: %v", err)
		jsonError(w, http.StatusUnauthorized, errCodeUnauthorized, "invalid or expired token")
		return
	}

	// Generate new access token
	accessToken, err := h.jwtService.GenerateToken(user)
	if err != nil {
		log.Printf("refresh error: generate access token: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	// Rotate refresh token (revoke old, create new)
	newRefreshToken, err := h.tokenService.RotateRefreshToken(ctx, req.RefreshToken, user.ID)
	if err != nil {
		log.Printf("refresh error: rotate refresh token: %v", err)
		jsonError(w, http.StatusInternalServerError, errCodeInternalError, "internal server error")
		return
	}

	log.Printf("token refresh success: user %s", user.Username)

	jsonOK(w, &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    h.jwtService.TTLSeconds(),
		TokenType:    "Bearer",
	})
}

// Logout handles user logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		jsonError(w, http.StatusBadRequest, errCodeBadRequest, "refresh_token required")
		return
	}

	ctx := r.Context()

	// Revoke the refresh token
	if err := h.tokenService.RevokeRefreshToken(ctx, req.RefreshToken); err != nil {
		log.Printf("logout error: revoke token: %v", err)
		// Don't return error - token might already be revoked
	}

	log.Printf("logout success")

	jsonNoContent(w)
}
