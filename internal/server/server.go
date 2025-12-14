package server

import (
	"context"
	"fmt"
	"log"
	"net"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds server configuration.
type Config struct {
	GRPCAddress string
	Verbose     bool
}

// Server is the BlazeLog gRPC server.
type Server struct {
	config     *Config
	grpcServer *grpc.Server
	handler    *Handler
	processor  *Processor
}

// New creates a new BlazeLog server.
func New(cfg *Config) *Server {
	processor := NewProcessor(cfg.Verbose)
	handler := NewHandler(processor, cfg.Verbose)

	grpcServer := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()),
	)
	blazelogv1.RegisterLogServiceServer(grpcServer, handler)

	return &Server{
		config:     cfg,
		grpcServer: grpcServer,
		handler:    handler,
		processor:  processor,
	}
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
