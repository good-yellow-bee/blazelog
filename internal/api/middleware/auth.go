package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

// Context keys for storing user information.
type contextKey string

const (
	userIDKey   contextKey = "user_id"
	usernameKey contextKey = "username"
	roleKey     contextKey = "role"
	claimsKey   contextKey = "claims"
)

// jsonUnauthorized writes an unauthorized error response.
func jsonUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "invalid or expired token",
		},
	})
}

// jsonForbidden writes a forbidden error response.
func jsonForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "FORBIDDEN",
			"message": "access denied",
		},
	})
}

// JWTAuth returns middleware that validates JWT tokens.
func JWTAuth(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				jsonUnauthorized(w)
				return
			}

			// Parse Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				jsonUnauthorized(w)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				log.Printf("JWT auth failed for %s: %v", r.RemoteAddr, err)
				jsonUnauthorized(w)
				return
			}

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, usernameKey, claims.Username)
			ctx = context.WithValue(ctx, roleKey, claims.Role)
			ctx = context.WithValue(ctx, claimsKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWTOrSessionAuth returns middleware that validates JWT tokens or session cookies.
// This allows both API clients (using JWT) and web UI (using session) to access API endpoints.
func JWTOrSessionAuth(jwtService *auth.JWTService, sessions *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ctx context.Context

			// Try JWT first
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					claims, err := jwtService.ValidateToken(parts[1])
					if err == nil {
						ctx = r.Context()
						ctx = context.WithValue(ctx, userIDKey, claims.UserID)
						ctx = context.WithValue(ctx, usernameKey, claims.Username)
						ctx = context.WithValue(ctx, roleKey, claims.Role)
						ctx = context.WithValue(ctx, claimsKey, claims)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					log.Printf("JWT validation failed for %s: %v", r.RemoteAddr, err)
				}
			}

			// Try session cookie as fallback
			if sessions != nil {
				cookie, err := r.Cookie("session_id")
				if err == nil && cookie.Value != "" {
					sess, ok := sessions.Get(cookie.Value)
					if ok {
						ctx = r.Context()
						ctx = context.WithValue(ctx, userIDKey, sess.UserID)
						ctx = context.WithValue(ctx, usernameKey, sess.Username)
						ctx = context.WithValue(ctx, roleKey, models.Role(sess.Role))
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					log.Printf("Session not found or expired for %s: %s...", r.RemoteAddr, cookie.Value[:8])
				}
			}

			jsonUnauthorized(w)
		})
	}
}

// GetUserID returns the user ID from context.
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(userIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetUsername returns the username from context.
func GetUsername(ctx context.Context) string {
	if v := ctx.Value(usernameKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetRole returns the user role from context.
func GetRole(ctx context.Context) models.Role {
	if v := ctx.Value(roleKey); v != nil {
		if r, ok := v.(models.Role); ok {
			return r
		}
	}
	return ""
}

// GetClaims returns the JWT claims from context.
func GetClaims(ctx context.Context) *auth.Claims {
	if v := ctx.Value(claimsKey); v != nil {
		if c, ok := v.(*auth.Claims); ok {
			return c
		}
	}
	return nil
}
