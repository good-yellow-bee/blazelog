package middleware

import (
	"context"
	"net/http"

	"github.com/good-yellow-bee/blazelog/internal/web/handlers"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

func RequireSession(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			sess, ok := store.Get(cookie.Value)
			if !ok {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:   "session_id",
					Value:  "",
					Path:   "/",
					MaxAge: -1,
				})
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Add session to context
			ctx := context.WithValue(r.Context(), handlers.SessionContextKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole middleware ensures the user has the required role
// Must be used after RequireSession middleware
func RequireRole(store *session.Store, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := r.Context().Value(handlers.SessionContextKey).(*session.Session)
			if !ok || sess == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Check if user has required role
			if !hasRole(sess.Role, role) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasRole checks if userRole meets or exceeds requiredRole
// Role hierarchy: admin > operator > viewer
func hasRole(userRole, requiredRole string) bool {
	roleLevel := map[string]int{
		"viewer":   1,
		"operator": 2,
		"admin":    3,
	}

	userLevel, ok := roleLevel[userRole]
	if !ok {
		return false
	}

	requiredLevel, ok := roleLevel[requiredRole]
	if !ok {
		return false
	}

	return userLevel >= requiredLevel
}
