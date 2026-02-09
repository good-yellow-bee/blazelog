package api

import (
	"log"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/good-yellow-bee/blazelog/internal/api/alerts"
	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/api/connections"
	"github.com/good-yellow-bee/blazelog/internal/api/logs"
	"github.com/good-yellow-bee/blazelog/internal/api/middleware"
	"github.com/good-yellow-bee/blazelog/internal/api/projects"
	"github.com/good-yellow-bee/blazelog/internal/api/users"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/web"
)

// setupRouter creates and configures the chi router with all routes.
func (s *Server) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Configure trusted proxies for rate limiting
	if len(s.config.TrustedProxies) > 0 {
		if err := middleware.SetTrustedProxies(s.config.TrustedProxies); err != nil {
			// Log warning but continue - will fall back to direct IP
			log.Printf("warning: failed to configure trusted proxies: %v", err)
		}
	}

	// Create JWT service
	jwtService := auth.NewJWTService(s.config.JWTSecret, s.config.AccessTokenTTL)

	// Create lockout tracker
	lockoutTracker := auth.NewLockoutTracker(s.config.LockoutThreshold, s.config.LockoutDuration)

	// Create rate limiters
	ipLimiter := middleware.NewRateLimiterWithWindow(s.config.RateLimitPerIP, 15*time.Minute)
	userLimiter := middleware.NewRateLimiter(s.config.RateLimitPerUser)

	// Global middleware
	r.Use(middleware.PrometheusMiddleware)
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

		// Hybrid auth middleware that accepts both JWT and session cookies
		hybridAuth := middleware.JWTOrSessionAuth(jwtService, s.sessions)

		// User routes (protected)
		r.Route("/users", func(r chi.Router) {
			r.Use(hybridAuth)
			r.Use(middleware.RateLimitByUser(userLimiter))

			userHandler := users.NewHandler(s.storage, s.sessions)

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

				// Admin-only operations
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin))
					r.Put("/password", userHandler.ResetPassword)
					r.Delete("/", userHandler.Delete)
				})
			})
		})

		// Log routes (protected - any authenticated user can view)
		r.Route("/logs", func(r chi.Router) {
			r.Use(hybridAuth)
			r.Use(middleware.RateLimitByUser(userLimiter))

			logsHandler := logs.NewHandlerWithStorageAndConfig(s.logStorage, s.storage, logs.HandlerConfig{
				MaxQueryRange:      s.config.MaxQueryRange,
				QueryTimeout:       s.config.QueryTimeout,
				StreamMaxDuration:  s.config.StreamMaxDuration,
				StreamPollInterval: s.config.StreamPollInterval,
			})

			r.Get("/", logsHandler.Query)
			r.Get("/stats", logsHandler.Stats)
			r.Get("/stream", logsHandler.Stream)
			r.Get("/{id}/context", logsHandler.Context)
		})

		// Alert routes (protected)
		r.Route("/alerts", func(r chi.Router) {
			r.Use(hybridAuth)
			r.Use(middleware.RateLimitByUser(userLimiter))

			alertsHandler := alerts.NewHandler(s.storage)

			r.Get("/", alertsHandler.List)
			r.Get("/history", alertsHandler.History)

			// Admin/Operator can create
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(models.RoleAdmin, models.RoleOperator))
				r.Post("/", alertsHandler.Create)
			})

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", alertsHandler.GetByID)

				// Admin/Operator can update
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin, models.RoleOperator))
					r.Put("/", alertsHandler.Update)
				})

				// Admin only can delete
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin))
					r.Delete("/", alertsHandler.Delete)
				})
			})
		})

		// Project routes (protected)
		r.Route("/projects", func(r chi.Router) {
			r.Use(hybridAuth)
			r.Use(middleware.RateLimitByUser(userLimiter))

			projectsHandler := projects.NewHandler(s.storage)

			r.Get("/", projectsHandler.List)

			// Admin only for create
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(models.RoleAdmin))
				r.Post("/", projectsHandler.Create)
			})

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", projectsHandler.GetByID)
				r.Get("/users", projectsHandler.GetUsers)

				// Admin only for update/delete/user management
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin))
					r.Put("/", projectsHandler.Update)
					r.Delete("/", projectsHandler.Delete)
					r.Post("/users", projectsHandler.AddUser)
					r.Delete("/users/{userId}", projectsHandler.RemoveUser)
				})
			})
		})

		// Connection routes (protected)
		r.Route("/connections", func(r chi.Router) {
			r.Use(hybridAuth)
			r.Use(middleware.RateLimitByUser(userLimiter))

			connectionsHandler := connections.NewHandler(s.storage)

			r.Get("/", connectionsHandler.List)

			// Admin only for create
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(models.RoleAdmin))
				r.Post("/", connectionsHandler.Create)
			})

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", connectionsHandler.GetByID)

				// Admin only for update/delete/test
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(models.RoleAdmin))
					r.Put("/", connectionsHandler.Update)
					r.Delete("/", connectionsHandler.Delete)
					r.Post("/test", connectionsHandler.Test)
				})
			})
		})
	})

	// Health check endpoints (public, no rate limit)
	r.Get("/health", s.healthHandler.Health)
	r.Get("/health/live", s.healthHandler.Live)
	r.Get("/health/ready", s.healthHandler.Ready)

	// Web UI routes (mounted at root, but API routes take precedence)
	// Share the session store with the web server so sessions work across both
	if s.config.WebUIEnabled && s.config.CSRFSecret != "" {
		webServer := web.NewServerWithSessions(s.storage, s.logStorage, s.config.CSRFSecret, s.config.TrustedOrigins, s.sessions, s.config.UseSecureCookies)
		r.Mount("/", webServer.Routes())
	}

	return r
}
