package handlers

import (
	"net/http"

	"github.com/good-yellow-bee/blazelog/internal/web/templates/pages"
)

func (h *Handler) ShowLogs(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	pages.Logs(sess).Render(r.Context(), w)
}
