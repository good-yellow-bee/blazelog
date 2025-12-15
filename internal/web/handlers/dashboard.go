package handlers

import (
	"net/http"

	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
)

func (h *Handler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// TODO: Fetch real stats from storage
	stats := pages.DashboardStats{
		TotalLogs:    12543,
		ErrorCount:   87,
		WarningCount: 234,
		ActiveAlerts: 3,
	}

	pages.Dashboard(sess, stats).Render(r.Context(), w)
}
