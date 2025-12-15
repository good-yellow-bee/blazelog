package web

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/storage"
	"github.com/good-yellow-bee/blazelog/internal/web/handlers"
	"github.com/good-yellow-bee/blazelog/internal/web/session"
)

//go:embed static/css/*
var staticFS embed.FS

type Server struct {
	handler  *handlers.Handler
	sessions *session.Store
	csrfKey  []byte
}

func NewServer(storage storage.Storage, logStorage storage.LogStorage, csrfKey string) *Server {
	sessions := session.NewStore(24 * time.Hour)
	return &Server{
		handler:  handlers.NewHandler(storage, logStorage, sessions, csrfKey),
		sessions: sessions,
		csrfKey:  []byte(csrfKey),
	}
}

func (s *Server) StaticFS() http.Handler {
	sub, _ := fs.Sub(staticFS, "static")
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
