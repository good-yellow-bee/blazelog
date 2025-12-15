package handlers

import (
	"net/http"

	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
)

// ShowAlerts renders the alerts management page
// Accessible to all authenticated users (view), create/edit for operator+, delete for admin
func (h *Handler) ShowAlerts(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	pages.SettingsAlerts(sess).Render(r.Context(), w)
}

// ShowProjects renders the projects management page (admin only)
func (h *Handler) ShowProjects(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Double-check admin role (middleware handles this, but belt-and-suspenders)
	if sess.Role != "admin" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	pages.SettingsProjects(sess).Render(r.Context(), w)
}

// ShowConnections renders the connections management page (admin only)
func (h *Handler) ShowConnections(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if sess.Role != "admin" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	pages.SettingsConnections(sess).Render(r.Context(), w)
}

// ShowUsers renders the users management page (admin only)
func (h *Handler) ShowUsers(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if sess.Role != "admin" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	pages.SettingsUsers(sess).Render(r.Context(), w)
}
