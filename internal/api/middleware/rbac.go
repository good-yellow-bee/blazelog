package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// RequireRole returns middleware that requires specific roles.
func RequireRole(allowedRoles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetRole(r.Context())
			if userRole == "" {
				jsonForbidden(w)
				return
			}

			// Check if user has any of the allowed roles
			for _, role := range allowedRoles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Admin always has access
			if userRole == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			jsonForbidden(w)
		})
	}
}

// RequireAdmin is shorthand for RequireRole(RoleAdmin).
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)(next)
}

// RequireOperator is shorthand for RequireRole(RoleOperator, RoleAdmin).
func RequireOperator(next http.Handler) http.Handler {
	return RequireRole(models.RoleOperator, models.RoleAdmin)(next)
}

// RequireAdminOrSelf allows access if user is admin or accessing their own resource.
// Expects {id} URL parameter.
func RequireAdminOrSelf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		userRole := GetRole(r.Context())

		// Admin has access to everything
		if userRole == models.RoleAdmin {
			next.ServeHTTP(w, r)
			return
		}

		// Check if accessing own resource
		resourceID := chi.URLParam(r, "id")
		if resourceID != "" && resourceID == userID {
			next.ServeHTTP(w, r)
			return
		}

		jsonForbidden(w)
	})
}

// RequireCanWrite allows access to admin and operator roles.
func RequireCanWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userRole := GetRole(r.Context())

		if userRole == models.RoleAdmin || userRole == models.RoleOperator {
			next.ServeHTTP(w, r)
			return
		}

		jsonForbidden(w)
	})
}
