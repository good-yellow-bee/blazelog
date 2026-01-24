package server

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/metrics"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validation limits for AgentInfo fields.
const (
	maxAgentHostnameLen = 255
	maxAgentNameLen     = 128
	maxAgentVersionLen  = 64
)

// agentEntry tracks an agent with its last activity time.
type agentEntry struct {
	info       *blazelogv1.AgentInfo
	lastActive atomic.Value // stores time.Time
}

// Handler implements the LogServiceServer gRPC interface.
type Handler struct {
	blazelogv1.UnimplementedLogServiceServer

	processor *Processor
	agents    sync.Map // agent_id -> *agentEntry
	verbose   bool

	// Metrics
	totalBatches  uint64
	totalEntries  uint64
	activeStreams int32

	// Rate limiters
	registerLimiter *rate.Limiter // 10/sec with burst of 50

	// Cleanup
	agentTTL time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewHandler creates a new gRPC handler.
func NewHandler(processor *Processor, verbose bool) *Handler {
	h := &Handler{
		processor:       processor,
		verbose:         verbose,
		registerLimiter: rate.NewLimiter(10, 50), // 10/sec with burst of 50
		agentTTL:        30 * time.Minute,        // Agents inactive for 30 min are removed
		stopCh:          make(chan struct{}),
	}
	go h.cleanupLoop()
	return h
}

// cleanupLoop periodically removes inactive agents.
func (h *Handler) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.cleanupInactiveAgents()
		case <-h.stopCh:
			return
		}
	}
}

// cleanupInactiveAgents removes agents that haven't been active recently.
func (h *Handler) cleanupInactiveAgents() {
	cutoff := time.Now().Add(-h.agentTTL)
	removed := 0

	h.agents.Range(func(key, value any) bool {
		entry := value.(*agentEntry)
		lastActive := entry.lastActive.Load().(time.Time)
		if lastActive.Before(cutoff) {
			h.agents.Delete(key)
			removed++
		}
		return true
	})

	if removed > 0 && h.verbose {
		log.Printf("cleaned up %d inactive agents", removed)
	}
}

// Stop stops the handler's background goroutines.
func (h *Handler) Stop() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})
}

// Register handles agent registration.
func (h *Handler) Register(ctx context.Context, req *blazelogv1.RegisterRequest) (*blazelogv1.RegisterResponse, error) {
	// Rate limit registration requests
	if !h.registerLimiter.Allow() {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "registration rate limit exceeded",
		}, nil
	}

	agent := req.Agent
	if agent == nil {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "agent info is required",
		}, nil
	}

	// Validate AgentInfo fields
	if len(agent.Hostname) > maxAgentHostnameLen {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "hostname exceeds maximum length",
		}, nil
	}
	if len(agent.Name) > maxAgentNameLen {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "name exceeds maximum length",
		}, nil
	}
	if len(agent.Version) > maxAgentVersionLen {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "version exceeds maximum length",
		}, nil
	}

	// Generate agent ID if not provided
	agentID := agent.AgentId
	if agentID == "" {
		agentID = uuid.New().String()
	}

	// Store agent info with activity timestamp
	entry := &agentEntry{info: agent}
	entry.lastActive.Store(time.Now())
	h.agents.Store(agentID, entry)
	metrics.GRPCAgentsRegistered.Inc()

	projectID := agent.ProjectId
	if projectID != "" {
		log.Printf("agent registered: id=%s name=%s hostname=%s project=%s sources=%d",
			agentID, agent.Name, agent.Hostname, projectID, len(agent.Sources))
	} else {
		log.Printf("agent registered: id=%s name=%s hostname=%s sources=%d (no project)",
			agentID, agent.Name, agent.Hostname, len(agent.Sources))
	}

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

const (
	maxBatchSize      = 100
	streamIdleTimeout = 5 * time.Minute
)

// StreamLogs handles bidirectional log streaming from agents.
//
// Sequence number limitation: The server acknowledges sequence numbers from batches
// but does not track or validate ordering/gaps per agent. Agents are expected to
// handle their own retry logic based on acked sequences. This design is intentional
// as it keeps the server stateless and simpler. Logs are idempotent (UUID-based IDs)
// so duplicate delivery is safe.
func (h *Handler) StreamLogs(stream grpc.BidiStreamingServer[blazelogv1.LogBatch, blazelogv1.StreamResponse]) error {
	atomic.AddInt32(&h.activeStreams, 1)
	metrics.GRPCStreamsActive.Inc()
	defer func() {
		atomic.AddInt32(&h.activeStreams, -1)
		metrics.GRPCStreamsActive.Dec()
	}()

	if h.verbose {
		log.Printf("stream started, active streams: %d", atomic.LoadInt32(&h.activeStreams))
	}

	// Idle timeout timer
	idleTimer := time.NewTimer(streamIdleTimeout)
	defer idleTimer.Stop()

	recvCh := make(chan *blazelogv1.LogBatch, 1)
	errCh := make(chan error, 1)

	// Receive goroutine - blocks on stream.Recv() which unblocks when:
	// 1. Client sends data (normal flow)
	// 2. Client closes stream (returns io.EOF)
	// 3. Server returns from StreamLogs (gRPC framework cancels stream context)
	// On idle timeout, returning from this function triggers gRPC cleanup
	// which cancels the stream context and unblocks stream.Recv().
	go func() {
		for {
			batch, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			recvCh <- batch
		}
	}()

	for {
		select {
		case <-idleTimer.C:
			return status.Error(codes.DeadlineExceeded, "stream idle timeout")

		case err := <-errCh:
			if errors.Is(err, io.EOF) {
				if h.verbose {
					log.Printf("stream closed by client")
				}
				return nil
			}
			return err

		case batch := <-recvCh:
			// Reset idle timer
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(streamIdleTimeout)

			// Validate batch size
			if len(batch.Entries) > maxBatchSize {
				return status.Errorf(codes.InvalidArgument, "batch size %d exceeds maximum %d", len(batch.Entries), maxBatchSize)
			}

			// Process the batch
			if err := h.processor.ProcessBatch(batch); err != nil {
				log.Printf("process batch error: %v", err)
				metrics.GRPCBatchProcessErrors.Inc()
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
			metrics.GRPCBatchesTotal.Inc()
			metrics.GRPCEntriesTotal.Add(float64(len(batch.Entries)))

			// Send acknowledgement
			if err := stream.Send(&blazelogv1.StreamResponse{
				AckedSequence: batch.Sequence,
			}); err != nil {
				return err
			}
		}
	}
}

// Heartbeat handles agent heartbeat messages.
func (h *Handler) Heartbeat(ctx context.Context, req *blazelogv1.HeartbeatRequest) (*blazelogv1.HeartbeatResponse, error) {
	// Update agent's last active time
	if req.AgentId != "" {
		if entry, ok := h.agents.Load(req.AgentId); ok {
			e := entry.(*agentEntry)
			e.lastActive.Store(time.Now())
		}
	}

	if h.verbose {
		reqStatus := req.Status
		if reqStatus != nil {
			log.Printf("heartbeat from %s: processed=%d buffer=%d sources=%d",
				req.AgentId, reqStatus.EntriesProcessed, reqStatus.BufferSize, reqStatus.ActiveSources)
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
