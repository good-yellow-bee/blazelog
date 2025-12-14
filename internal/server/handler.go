package server

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
)

// Handler implements the LogServiceServer gRPC interface.
type Handler struct {
	blazelogv1.UnimplementedLogServiceServer

	processor *Processor
	agents    sync.Map // agent_id -> *blazelogv1.AgentInfo
	verbose   bool

	// Metrics
	totalBatches  uint64
	totalEntries  uint64
	activeStreams int32
}

// NewHandler creates a new gRPC handler.
func NewHandler(processor *Processor, verbose bool) *Handler {
	return &Handler{
		processor: processor,
		verbose:   verbose,
	}
}

// Register handles agent registration.
func (h *Handler) Register(ctx context.Context, req *blazelogv1.RegisterRequest) (*blazelogv1.RegisterResponse, error) {
	agent := req.Agent
	if agent == nil {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "agent info is required",
		}, nil
	}

	// Generate agent ID if not provided
	agentID := agent.AgentId
	if agentID == "" {
		agentID = uuid.New().String()
	}

	// Store agent info
	h.agents.Store(agentID, agent)

	log.Printf("agent registered: id=%s name=%s hostname=%s sources=%d",
		agentID, agent.Name, agent.Hostname, len(agent.Sources))

	return &blazelogv1.RegisterResponse{
		Success: true,
		AgentId: agentID,
		Config: &blazelogv1.StreamConfig{
			MaxBatchSize:       100,
			FlushIntervalMs:    1000,
			CompressionEnabled: false,
		},
	}, nil
}

// StreamLogs handles bidirectional log streaming from agents.
func (h *Handler) StreamLogs(stream grpc.BidiStreamingServer[blazelogv1.LogBatch, blazelogv1.StreamResponse]) error {
	atomic.AddInt32(&h.activeStreams, 1)
	defer atomic.AddInt32(&h.activeStreams, -1)

	if h.verbose {
		log.Printf("stream started, active streams: %d", atomic.LoadInt32(&h.activeStreams))
	}

	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			if h.verbose {
				log.Printf("stream closed by client")
			}
			return nil
		}
		if err != nil {
			return err
		}

		// Process the batch
		if err := h.processor.ProcessBatch(batch); err != nil {
			log.Printf("process batch error: %v", err)
			// Send error response but continue
			if sendErr := stream.Send(&blazelogv1.StreamResponse{
				AckedSequence: batch.Sequence,
				Error:         err.Error(),
			}); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Update metrics
		atomic.AddUint64(&h.totalBatches, 1)
		atomic.AddUint64(&h.totalEntries, uint64(len(batch.Entries)))

		// Send acknowledgement
		if err := stream.Send(&blazelogv1.StreamResponse{
			AckedSequence: batch.Sequence,
		}); err != nil {
			return err
		}
	}
}

// Heartbeat handles agent heartbeat messages.
func (h *Handler) Heartbeat(ctx context.Context, req *blazelogv1.HeartbeatRequest) (*blazelogv1.HeartbeatResponse, error) {
	if h.verbose {
		status := req.Status
		if status != nil {
			log.Printf("heartbeat from %s: processed=%d buffer=%d sources=%d",
				req.AgentId, status.EntriesProcessed, status.BufferSize, status.ActiveSources)
		} else {
			log.Printf("heartbeat from %s", req.AgentId)
		}
	}

	return &blazelogv1.HeartbeatResponse{
		Acknowledged: true,
	}, nil
}

// Stats returns current handler statistics.
func (h *Handler) Stats() (batches, entries uint64, streams int32) {
	return atomic.LoadUint64(&h.totalBatches),
		atomic.LoadUint64(&h.totalEntries),
		atomic.LoadInt32(&h.activeStreams)
}

// AgentCount returns the number of registered agents.
func (h *Handler) AgentCount() int {
	count := 0
	h.agents.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
