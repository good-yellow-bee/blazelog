package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/handlers"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	handler          *handlers.Handler
	sessions         *session.Store
	csrfKey          []byte
	useSecureCookies bool
}

func NewServer(storage storage.Storage, logStorage storage.LogStorage, csrfKey string, _ []string) *Server {
	sessions := session.NewStore(24 * time.Hour)
	return NewServerWithSessions(storage, logStorage, csrfKey, nil, sessions, false)
}

// NewServerWithSessions creates a new server with a provided session store.
// This allows sharing the session store with the API server.
// Note: trustedOrigins parameter is deprecated due to vulnerability GO-2025-3884.
func NewServerWithSessions(storage storage.Storage, logStorage storage.LogStorage, csrfKey string, _ []string, sessions *session.Store, useSecureCookies bool) *Server {
	return &Server{
		handler:          handlers.NewHandler(storage, logStorage, sessions, csrfKey),
		sessions:         sessions,
		csrfKey:          []byte(csrfKey),
		useSecureCookies: useSecureCookies,
	}
}

func (s *Server) StaticFS() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Unrecoverable init error - server cannot function without static assets
		panic(fmt.Sprintf("failed to create static FS: %v", err))
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})
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
