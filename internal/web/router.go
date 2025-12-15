package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/good-yellow-bee/blazelog/internal/web/middleware"
	"github.com/gorilla/csrf"
)

func (s *Server) Routes() chi.Router {
	r := chi.NewRouter()

	// CSRF protection
	csrfMiddleware := csrf.Protect(
		s.csrfKey,
		csrf.Secure(false), // Set to true in production
		csrf.Path("/"),
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
		r.Post("/logout", s.handler.HandleLogout)

		// Placeholder routes for future milestones
		r.Get("/logs", s.handler.ShowDashboard)     // TODO: Milestone 27
		r.Get("/alerts", s.handler.ShowDashboard)   // TODO: Milestone 28
		r.Get("/projects", s.handler.ShowDashboard) // TODO: Milestone 28
		r.Get("/users", s.handler.ShowDashboard)    // TODO: Milestone 28
	})

	return r
}
