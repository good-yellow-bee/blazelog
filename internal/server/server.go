package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"github.com/good-yellow-bee/blazelog/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds server configuration.
type Config struct {
	GRPCAddress string
	Verbose     bool
	TLS         *TLSConfig // nil = insecure mode
	LogBuffer   LogBuffer  // nil = no ClickHouse storage
}

// LogBuffer interface for log buffering (implemented by storage.LogBuffer).
type LogBuffer interface {
	AddBatch(entries []*LogRecord) error
	Close() error
}

// LogRecord represents a log entry for storage.
type LogRecord struct {
	ID         string
	Timestamp  time.Time
	Level      string
	Message    string
	Source     string
	Type       string
	Raw        string
	AgentID    string
	FilePath   string
	LineNumber int64
	Fields     map[string]interface{}
	Labels     map[string]string
	HTTPStatus int
	HTTPMethod string
	URI        string
}

// TLSConfig holds TLS configuration for the server.
type TLSConfig struct {
	CertFile     string
	KeyFile      string
	ClientCAFile string
}

// Server is the BlazeLog gRPC server.
type Server struct {
	config     *Config
	grpcServer *grpc.Server
	handler    *Handler
	processor  *Processor
}

// New creates a new BlazeLog server.
func New(cfg *Config) (*Server, error) {
	processor := NewProcessor(cfg.Verbose, cfg.LogBuffer)
	handler := NewHandler(processor, cfg.Verbose)

	var opts []grpc.ServerOption

	// Configure TLS if enabled
	if cfg.TLS != nil {
		tlsCfg := &security.ServerTLSConfig{
			CertFile:     cfg.TLS.CertFile,
			KeyFile:      cfg.TLS.KeyFile,
			ClientCAFile: cfg.TLS.ClientCAFile,
		}
		creds, err := security.LoadServerTLS(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("load server TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
		log.Printf("mTLS enabled for gRPC server")
	} else {
		opts = append(opts, grpc.Creds(insecure.NewCredentials()))
		log.Printf("WARNING: gRPC server running in insecure mode (no TLS)")
	}

	grpcServer := grpc.NewServer(opts...)
	blazelogv1.RegisterLogServiceServer(grpcServer, handler)

	return &Server{
		config:     cfg,
		grpcServer: grpcServer,
		handler:    handler,
		processor:  processor,
	}, nil
}

// Run starts the server and blocks until context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.GRPCAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.config.GRPCAddress, err)
	}

	log.Printf("gRPC server listening on %s", s.config.GRPCAddress)

	// Handle shutdown
	go func() {
		<-ctx.Done()
		log.Printf("shutting down gRPC server...")
		s.grpcServer.GracefulStop()
	}()

	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	s.grpcServer.GracefulStop()
}

// Stats returns current server statistics.
func (s *Server) Stats() (batches, entries uint64, streams int32, agents int) {
	batches, entries, streams = s.handler.Stats()
	agents = s.handler.AgentCount()
	return
}
