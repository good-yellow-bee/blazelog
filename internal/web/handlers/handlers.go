package handlers

import (
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

type Handler struct {
	storage  storage.Storage
	sessions *session.Store
	csrfKey  string
}

func NewHandler(storage storage.Storage, sessions *session.Store, csrfKey string) *Handler {
	if sessions == nil {
		sessions = session.NewStore(24 * time.Hour)
	}
	return &Handler{
		storage:  storage,
		sessions: sessions,
		csrfKey:  csrfKey,
	}
}

// Helper to get session from context
type contextKey string

const SessionContextKey contextKey = "session"

func GetSession(r *http.Request) *session.Session {
	if s, ok := r.Context().Value(SessionContextKey).(*session.Session); ok {
		return s
	}
	return nil
}
