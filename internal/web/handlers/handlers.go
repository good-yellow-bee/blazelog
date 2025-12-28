package handlers

import (
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

type Handler struct {
	storage        storage.Storage
	logStorage     storage.LogStorage
	sessions       *session.Store
	csrfKey        string
	lockoutTracker *auth.LockoutTracker
}

// HandlerConfig contains configuration for the web handler.
type HandlerConfig struct {
	Storage          storage.Storage
	LogStorage       storage.LogStorage
	Sessions         *session.Store
	CSRFKey          string
	LockoutThreshold int
	LockoutDuration  time.Duration
}

func NewHandler(storage storage.Storage, logStorage storage.LogStorage, sessions *session.Store, csrfKey string) *Handler {
	return NewHandlerWithConfig(HandlerConfig{
		Storage:          storage,
		LogStorage:       logStorage,
		Sessions:         sessions,
		CSRFKey:          csrfKey,
		LockoutThreshold: 5,                // default
		LockoutDuration:  15 * time.Minute, // default
	})
}

// NewHandlerWithConfig creates a handler with full configuration.
func NewHandlerWithConfig(cfg HandlerConfig) *Handler {
	if cfg.Sessions == nil {
		cfg.Sessions = session.NewStore(24 * time.Hour)
	}
	if cfg.LockoutThreshold == 0 {
		cfg.LockoutThreshold = 5
	}
	if cfg.LockoutDuration == 0 {
		cfg.LockoutDuration = 15 * time.Minute
	}
	return &Handler{
		storage:        cfg.Storage,
		logStorage:     cfg.LogStorage,
		sessions:       cfg.Sessions,
		csrfKey:        cfg.CSRFKey,
		lockoutTracker: auth.NewLockoutTracker(cfg.LockoutThreshold, cfg.LockoutDuration),
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
