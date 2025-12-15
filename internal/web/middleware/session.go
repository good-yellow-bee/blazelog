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
