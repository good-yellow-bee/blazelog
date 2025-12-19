package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/good-yellow-bee/blazelog/internal/web/middleware"
	"github.com/gorilla/csrf"
)

func (s *Server) Routes() chi.Router {
	r := chi.NewRouter()

	// CSRF protection options
	// Note: TrustedOrigins removed due to vulnerability GO-2025-3884
	csrfMiddleware := csrf.Protect(
		s.csrfKey,
		csrf.Secure(false),           // Set to true in production with HTTPS
		csrf.Path("/"),
		csrf.FieldName("csrf_token"), // Match the form field name
	)
	r.Use(csrfMiddleware)

	// Static files (no CSRF)
	r.Handle("/static/*", http.StripPrefix("/static/", s.StaticFS()))

	// Public routes
	r.Get("/login", s.handler.ShowLogin)
	r.Post("/login", s.handler.HandleLogin)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireSession(s.sessions))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		})
		r.Get("/dashboard", s.handler.ShowDashboard)
		r.Get("/dashboard/stats", s.handler.GetDashboardStats)
		r.Post("/logout", s.handler.HandleLogout)

		// Log viewer routes
		r.Get("/logs", s.handler.ShowLogs)
		r.Get("/logs/data", s.handler.GetLogsData)
		r.Get("/logs/export", s.handler.ExportLogs)
		r.Get("/logs/stream", s.handler.StreamLogs)

		// Settings routes
		r.Get("/settings/alerts", s.handler.ShowAlerts)

		// Admin-only settings
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(s.sessions, "admin"))
			r.Get("/settings/projects", s.handler.ShowProjects)
			r.Get("/settings/connections", s.handler.ShowConnections)
			r.Get("/settings/users", s.handler.ShowUsers)
		})
	})

	return r
}
