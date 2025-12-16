package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
)

// ConnState represents the connection state.
type ConnState int32

const (
	ConnStateDisconnected ConnState = iota
	ConnStateConnecting
	ConnStateRegistering
	ConnStateConnected
	ConnStateReconnecting
)

func (s ConnState) String() string {
	switch s {
	case ConnStateDisconnected:
		return "disconnected"
	case ConnStateConnecting:
		return "connecting"
	case ConnStateRegistering:
		return "registering"
	case ConnStateConnected:
		return "connected"
	case ConnStateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ConnManagerConfig configures the connection manager.
type ConnManagerConfig struct {
	ServerAddress string
	TLS           *TLSConfig
	AgentInfo     *blazelogv1.AgentInfo

	// Retry settings
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	MaxRetries     int // 0 = infinite retries
}

// DefaultConnManagerConfig returns default configuration.
func DefaultConnManagerConfig() ConnManagerConfig {
	return ConnManagerConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		MaxRetries:     0, // Infinite
	}
}

// ConnManager manages the connection lifecycle with automatic reconnection.
type ConnManager struct {
	config  ConnManagerConfig
	client  *Client
	backoff *Backoff
	state   atomic.Int32
	agentID string
	verbose bool

	// Callbacks
	onConnected    func()
	onDisconnected func(error)
	onStateChange  func(ConnState)

	// Internal
	mu          sync.Mutex
	reconnectCh chan struct{}
	stopCh      chan struct{}
}

// NewConnManager creates a new connection manager.
func NewConnManager(config ConnManagerConfig) *ConnManager {
	cm := &ConnManager{
		config: config,
		backoff: NewBackoffWithConfig(
			config.InitialBackoff,
			config.MaxBackoff,
			2.0,
			0.1,
		),
		reconnectCh: make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}
	cm.state.Store(int32(ConnStateDisconnected))
	return cm
}

// SetVerbose enables verbose logging.
func (cm *ConnManager) SetVerbose(v bool) {
	cm.verbose = v
}

// SetCallbacks sets the connection lifecycle callbacks.
func (cm *ConnManager) SetCallbacks(onConnected func(), onDisconnected func(error), onStateChange func(ConnState)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onConnected = onConnected
	cm.onDisconnected = onDisconnected
	cm.onStateChange = onStateChange
}

// State returns the current connection state.
func (cm *ConnManager) State() ConnState {
	return ConnState(cm.state.Load())
}

// IsConnected returns true if currently connected.
func (cm *ConnManager) IsConnected() bool {
	return cm.State() == ConnStateConnected
}

// Client returns the underlying gRPC client (may be nil).
func (cm *ConnManager) Client() *Client {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.client
}

// AgentID returns the registered agent ID.
func (cm *ConnManager) AgentID() string {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.agentID
}

// Connect establishes the initial connection with retry.
func (cm *ConnManager) Connect(ctx context.Context) error {
	cm.logf("connecting to %s", cm.config.ServerAddress)
	cm.setState(ConnStateConnecting)

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			cm.setState(ConnStateDisconnected)
			return ctx.Err()
		default:
		}

		err := cm.doConnect(ctx)
		if err == nil {
			cm.backoff.Reset()
			cm.setState(ConnStateConnected)
			cm.logf("connected successfully, agent_id=%s", cm.agentID)
			if cm.onConnected != nil {
				cm.onConnected()
			}
			return nil
		}

		attempts++
		if cm.config.MaxRetries > 0 && attempts >= cm.config.MaxRetries {
			cm.setState(ConnStateDisconnected)
			return fmt.Errorf("max retries (%d) exceeded: %w", cm.config.MaxRetries, err)
		}

		delay := cm.backoff.Next()
		cm.logf("connect failed (attempt %d): %v, retrying in %v", attempts, err, delay)

		select {
		case <-ctx.Done():
			cm.setState(ConnStateDisconnected)
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// TriggerReconnect signals that a reconnection is needed.
func (cm *ConnManager) TriggerReconnect() {
	select {
	case cm.reconnectCh <- struct{}{}:
	default:
		// Already pending
	}
}

// RunReconnectLoop runs the reconnection loop until context is canceled.
func (cm *ConnManager) RunReconnectLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopCh:
			return
		case <-cm.reconnectCh:
			cm.handleReconnect(ctx)
		}
	}
}

// handleReconnect performs the reconnection.
func (cm *ConnManager) handleReconnect(ctx context.Context) {
	cm.logf("reconnecting...")
	cm.setState(ConnStateReconnecting)

	// Close existing client
	cm.mu.Lock()
	if cm.client != nil {
		cm.client.Close()
		cm.client = nil
	}
	cm.mu.Unlock()

	// Notify disconnected
	if cm.onDisconnected != nil {
		cm.onDisconnected(fmt.Errorf("reconnecting"))
	}

	// Reconnect
	if err := cm.Connect(ctx); err != nil {
		cm.logf("reconnect failed: %v", err)
		// Will retry on next trigger
	}
}

// doConnect performs the actual connection and registration.
func (cm *ConnManager) doConnect(ctx context.Context) error {
	// Create new client
	client, err := NewClient(cm.config.ServerAddress, cm.config.TLS)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	// Register
	cm.setState(ConnStateRegistering)
	resp, err := client.Register(ctx, cm.config.AgentInfo)
	if err != nil {
		client.Close()
		return fmt.Errorf("register: %w", err)
	}

	// Start stream
	if err := client.StartStream(ctx); err != nil {
		client.Close()
		return fmt.Errorf("start stream: %w", err)
	}

	cm.mu.Lock()
	cm.client = client
	cm.agentID = resp.AgentId
	cm.mu.Unlock()

	return nil
}

// Close stops the connection manager and closes the client.
func (cm *ConnManager) Close() error {
	close(cm.stopCh)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.setState(ConnStateDisconnected)
	if cm.client != nil {
		return cm.client.Close()
	}
	return nil
}

// setState updates the state and notifies callback.
func (cm *ConnManager) setState(state ConnState) {
	old := ConnState(cm.state.Swap(int32(state)))
	if old != state && cm.onStateChange != nil {
		cm.onStateChange(state)
	}
}

func (cm *ConnManager) logf(format string, args ...interface{}) {
	if cm.verbose {
		log.Printf("[connmgr] "+format, args...)
	}
}
