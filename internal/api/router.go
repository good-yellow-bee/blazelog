package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/api/users"
	"github.com/good-yellow-bee/blazelog/internal/models"
)

// setupRouter creates and configures the chi router with all routes.
func (s *Server) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Create JWT service
	jwtService := auth.NewJWTService(s.config.JWTSecret, s.config.AccessTokenTTL)

	// Create lockout tracker
	lockoutTracker := auth.NewLockoutTracker(s.config.LockoutThreshold, s.config.LockoutDuration)

	// Create rate limiters
	ipLimiter := middleware.NewRateLimiter(s.config.RateLimitPerIP)
	userLimiter := middleware.NewRateLimiter(s.config.RateLimitPerUser)

	// Global middleware
	r.Use(middleware.RequestLogger(s.config.Verbose))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Recoverer)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (mostly public)
		r.Route("/auth", func(r chi.Router) {
			authHandler := auth.NewHandler(
				s.storage,
				jwtService,
				lockoutTracker,
				s.config.RefreshTokenTTL,
			)

			// Public routes with IP rate limiting
			r.Group(func(r chi.Router) {
				r.Use(middleware.RateLimitByIP(ipLimiter))
				r.Post("/login", authHandler.Login)
				r.Post("/refresh", authHandler.Refresh)
			})

			// Protected routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(jwtService))
				r.Post("/logout", authHandler.Logout)
			})
		})

		// User routes (protected)
		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.JWTAuth(jwtService))
			r.Use(middleware.RateLimitByUser(userLimiter))

			userHandler := users.NewHandler(s.storage)

			// Current user endpoints (any authenticated user)
			r.Get("/me", userHandler.GetCurrentUser)
			r.Put("/me/password", userHandler.ChangePassword)

			// Admin-only endpoints
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(models.RoleAdmin))
				r.Get("/", userHandler.List)
				r.Post("/", userHandler.Create)
			})

			// Per-user endpoints (admin or self)
			r.Route("/{id}", func(r chi.Router) {
				r.Use(middleware.RequireAdminOrSelf)
				r.Get("/", userHandler.GetByID)
				r.Put("/", userHandler.Update)

				// Delete is admin-only
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin))
					r.Delete("/", userHandler.Delete)
				})
			})
		})
	})

	// Health check (public, no rate limit)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		OK(w, map[string]string{"status": "ok"})
	})

	return r
}
