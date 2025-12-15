package handlers

import (
	"net/http"

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
	pages.Login(csrfToken, "").Render(r.Context(), w)
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

	// Get user from storage
	ctx := r.Context()
	user, err := h.storage.Users().GetByUsername(ctx, username)
	if err != nil || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		renderLoginError(w, r, "Invalid credentials")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		renderLoginError(w, r, "Invalid credentials")
		return
	}

	// Create session
	sess, err := h.sessions.Create(user.ID, user.Username, string(user.Role))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		renderLoginError(w, r, "Failed to create session")
		return
	}

	// Set session cookie
	rememberMe := r.FormValue("remember_me") == "on"
	maxAge := 86400 // 24 hours
	if rememberMe {
		maxAge = 2592000 // 30 days
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})

	// HTMX redirect
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
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
	components.Alert("error", message).Render(r.Context(), w)
}
