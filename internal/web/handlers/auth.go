package handlers

import (
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/components"
	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
	"github.com/gorilla/csrf"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie("session_id"); err == nil {
		if _, ok := h.sessions.Get(cookie.Value); ok {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
	}

	csrfToken := csrf.Token(r)
	pages.Login(csrfToken, "", middleware.GetCSPNonce(r.Context())).Render(r.Context(), w)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderLoginError(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		renderLoginError(w, r, "Username and password are required")
		return
	}

	// Check if account is locked out
	if h.lockoutTracker != nil && h.lockoutTracker.IsLocked(username) {
		w.WriteHeader(http.StatusTooManyRequests)
		renderLoginError(w, r, "Account temporarily locked due to too many failed attempts")
		return
	}

	// Get user from storage
	ctx := r.Context()
	user, err := h.storage.Users().GetByUsername(ctx, username)
	if err != nil || user == nil {
		// Record failed attempt
		if h.lockoutTracker != nil {
			h.lockoutTracker.RecordFailure(username)
		}
		w.WriteHeader(http.StatusUnauthorized)
		renderLoginError(w, r, "Invalid credentials")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Record failed attempt
		if h.lockoutTracker != nil {
			h.lockoutTracker.RecordFailure(username)
		}
		w.WriteHeader(http.StatusUnauthorized)
		renderLoginError(w, r, "Invalid credentials")
		return
	}

	// Clear any failed attempts on successful login
	if h.lockoutTracker != nil {
		h.lockoutTracker.ClearFailures(username)
	}

	// Invalidate any existing session to prevent session fixation
	if cookie, err := r.Cookie("session_id"); err == nil {
		h.sessions.Delete(cookie.Value)
	}

	// Determine session TTL based on remember-me
	rememberMe := r.FormValue("remember_me") == "on"
	sessionTTL := 24 * time.Hour
	maxAge := 86400 // 24 hours
	if rememberMe {
		sessionTTL = 30 * 24 * time.Hour // 30 days
		maxAge = 2592000                 // 30 days
	}

	// Create session with appropriate TTL
	sess, err := h.sessions.CreateWithTTL(user.ID, user.Username, string(user.Role), sessionTTL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		renderLoginError(w, r, "Failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   middleware.IsRequestSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Regular form submission - use HTTP redirect
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil {
		h.sessions.Delete(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusFound)
}

func renderLoginError(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html")
	if r.Header.Get("HX-Request") == "true" {
		// HTMX: return just the alert for partial replacement
		components.Alert("error", message).Render(r.Context(), w)
		return
	}
	// Non-HTMX: return full login page with error
	pages.Login(csrf.Token(r), message, middleware.GetCSPNonce(r.Context())).Render(r.Context(), w)
}
