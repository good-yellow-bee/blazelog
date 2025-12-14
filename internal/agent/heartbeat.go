package agent

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
)

// HeartbeatConfig configures the heartbeat runner.
type HeartbeatConfig struct {
	Interval  time.Duration // How often to send heartbeats (default: 15s)
	Timeout   time.Duration // Timeout for heartbeat RPC (default: 5s)
	MaxMissed int           // Max consecutive failures before triggering reconnect (default: 3)
}

// DefaultHeartbeatConfig returns default heartbeat configuration.
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Interval:  15 * time.Second,
		Timeout:   5 * time.Second,
		MaxMissed: 3,
	}
}

// StatusProvider provides current agent status for heartbeats.
type StatusProvider func() *blazelogv1.AgentStatus

// Heartbeater sends periodic heartbeats to the server.
type Heartbeater struct {
	config         HeartbeatConfig
	connMgr        *ConnManager
	statusProvider StatusProvider
	verbose        bool

	missedCount atomic.Int32
	mu          sync.Mutex
	running     bool
}

// NewHeartbeater creates a new heartbeat runner.
func NewHeartbeater(connMgr *ConnManager, config HeartbeatConfig, statusProvider StatusProvider) *Heartbeater {
	if config.Interval == 0 {
		config = DefaultHeartbeatConfig()
	}
	return &Heartbeater{
		config:         config,
		connMgr:        connMgr,
		statusProvider: statusProvider,
	}
}

// SetVerbose enables verbose logging.
func (h *Heartbeater) SetVerbose(v bool) {
	h.verbose = v
}

// Start begins the heartbeat loop.
// Returns a channel that signals when reconnection is needed (too many missed heartbeats).
func (h *Heartbeater) Start(ctx context.Context) <-chan struct{} {
	reconnectCh := make(chan struct{}, 1)

	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return reconnectCh
	}
	h.running = true
	h.mu.Unlock()

	go h.run(ctx, reconnectCh)
	return reconnectCh
}

// run is the main heartbeat loop.
func (h *Heartbeater) run(ctx context.Context, reconnectCh chan<- struct{}) {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	h.logf("heartbeat started, interval=%v, max_missed=%d", h.config.Interval, h.config.MaxMissed)

	for {
		select {
		case <-ctx.Done():
			h.logf("heartbeat stopped")
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
			return
		case <-ticker.C:
			h.sendHeartbeat(ctx, reconnectCh)
		}
	}
}

// sendHeartbeat sends a single heartbeat.
func (h *Heartbeater) sendHeartbeat(ctx context.Context, reconnectCh chan<- struct{}) {
	// Skip if not connected
	if !h.connMgr.IsConnected() {
		h.logf("skipping heartbeat, not connected")
		return
	}

	client := h.connMgr.Client()
	if client == nil {
		return
	}

	// Get current status
	var status *blazelogv1.AgentStatus
	if h.statusProvider != nil {
		status = h.statusProvider()
	} else {
		status = &blazelogv1.AgentStatus{}
	}

	// Send with timeout
	hbCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	resp, err := client.Heartbeat(hbCtx, status)
	if err != nil {
		missed := h.missedCount.Add(1)
		h.logf("heartbeat failed (missed=%d/%d): %v", missed, h.config.MaxMissed, err)

		if int(missed) >= h.config.MaxMissed {
			h.logf("max missed heartbeats reached, triggering reconnect")
			select {
			case reconnectCh <- struct{}{}:
			default:
			}
			h.missedCount.Store(0)
		}
		return
	}

	// Success - reset counter
	if h.missedCount.Load() > 0 {
		h.logf("heartbeat recovered after %d misses", h.missedCount.Load())
	}
	h.missedCount.Store(0)

	// Handle server command if any
	if resp.Command != nil {
		h.handleCommand(resp.Command)
	}
}

// handleCommand processes a server command from heartbeat response.
func (h *Heartbeater) handleCommand(cmd *blazelogv1.ServerCommand) {
	h.logf("received server command: %v", cmd.Type)

	switch cmd.Type {
	case blazelogv1.CommandType_COMMAND_TYPE_RELOAD_CONFIG:
		h.logf("reload config command received (not implemented)")
	case blazelogv1.CommandType_COMMAND_TYPE_PAUSE:
		h.logf("pause command received (not implemented)")
	case blazelogv1.CommandType_COMMAND_TYPE_RESUME:
		h.logf("resume command received (not implemented)")
	case blazelogv1.CommandType_COMMAND_TYPE_SHUTDOWN:
		h.logf("shutdown command received (not implemented)")
	}
}

// MissedCount returns the current missed heartbeat count.
func (h *Heartbeater) MissedCount() int {
	return int(h.missedCount.Load())
}

func (h *Heartbeater) logf(format string, args ...interface{}) {
	if h.verbose {
		log.Printf("[heartbeat] "+format, args...)
	}
}
