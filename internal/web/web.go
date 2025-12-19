package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/handlers"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

//go:embed static/css/*
var staticFS embed.FS

type Server struct {
	handler        *handlers.Handler
	sessions       *session.Store
	csrfKey        []byte
	trustedOrigins []string
}

func NewServer(storage storage.Storage, logStorage storage.LogStorage, csrfKey string, trustedOrigins []string) *Server {
	sessions := session.NewStore(24 * time.Hour)
	return NewServerWithSessions(storage, logStorage, csrfKey, trustedOrigins, sessions)
}

// NewServerWithSessions creates a new server with a provided session store.
// This allows sharing the session store with the API server.
func NewServerWithSessions(storage storage.Storage, logStorage storage.LogStorage, csrfKey string, trustedOrigins []string, sessions *session.Store) *Server {
	return &Server{
		handler:        handlers.NewHandler(storage, logStorage, sessions, csrfKey),
		sessions:       sessions,
		csrfKey:        []byte(csrfKey),
		trustedOrigins: trustedOrigins,
	}
}

func (s *Server) StaticFS() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Printf("Failed to create static FS: %v", err)
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}

func (s *Server) Sessions() *session.Store {
	return s.sessions
}

func (s *Server) Handler() *handlers.Handler {
	return s.handler
}

func (s *Server) CSRFKey() []byte {
	return s.csrfKey
}
