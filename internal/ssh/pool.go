package ssh

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// PoolConfig holds connection pool configuration.
type PoolConfig struct {
	// MaxPerHost is the maximum number of connections per host.
	MaxPerHost int
	// IdleTimeout is how long idle connections are kept before closing.
	IdleTimeout time.Duration
	// HealthCheckInterval is how often to check connection health.
	HealthCheckInterval time.Duration
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxPerHost:          5,
		IdleTimeout:         5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Pool manages a pool of SSH connections for reuse.
type Pool struct {
	config     PoolConfig
	clients    map[string]*hostPool
	clientKeys map[*Client]string // Maps client to its pool key for O(1) release lookup
	mu         sync.Mutex
	closed     bool
	closeCh    chan struct{}
}

// hostPool manages connections to a single host.
type hostPool struct {
	connections []*pooledConnection
	mu          sync.Mutex
}

// pooledConnection wraps a Client with pool metadata.
type pooledConnection struct {
	client   *Client
	lastUsed time.Time
	inUse    bool
	key      string
}

// NewPool creates a new connection pool.
func NewPool(config PoolConfig) *Pool {
	if config.MaxPerHost <= 0 {
		config.MaxPerHost = DefaultPoolConfig().MaxPerHost
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = DefaultPoolConfig().IdleTimeout
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = DefaultPoolConfig().HealthCheckInterval
	}

	p := &Pool{
		config:     config,
		clients:    make(map[string]*hostPool),
		clientKeys: make(map[*Client]string),
		closeCh:    make(chan struct{}),
	}

	go p.cleanupLoop()

	return p
}

// Get retrieves or creates a connection for the given config.
// The caller must call Release when done with the connection.
func (p *Pool) Get(ctx context.Context, cfg *ClientConfig) (*Client, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}

	key := p.configKey(cfg)

	hp, exists := p.clients[key]
	if !exists {
		hp = &hostPool{}
		p.clients[key] = hp
	}
	p.mu.Unlock()

	return p.getFromHostPool(ctx, hp, cfg, key)
}

// Release returns a connection to the pool for reuse.
// Uses O(1) lookup via clientKeys map instead of O(n) search.
func (p *Pool) Release(client *Client) {
	if client == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		client.Close()
		return
	}

	// O(1) lookup of the host pool key
	key, ok := p.clientKeys[client]
	if !ok {
		p.mu.Unlock()
		client.Close()
		return
	}

	hp, exists := p.clients[key]
	p.mu.Unlock() // Release pool mutex before acquiring host mutex

	if !exists {
		client.Close()
		return
	}

	// Now only lock the specific host pool
	hp.mu.Lock()
	for _, conn := range hp.connections {
		if conn.client == client {
			conn.inUse = false
			conn.lastUsed = time.Now()
			hp.mu.Unlock()
			return
		}
	}
	hp.mu.Unlock()

	// Client not found in this host pool - close it
	client.Close()
}

// Close closes all pooled connections and stops the cleanup goroutine.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.closeCh)

	var firstErr error
	for _, hp := range p.clients {
		hp.mu.Lock()
		for _, conn := range hp.connections {
			if err := conn.client.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		hp.connections = nil
		hp.mu.Unlock()
	}

	p.clients = make(map[string]*hostPool)
	p.clientKeys = make(map[*Client]string)
	return firstErr
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{
		Hosts: make(map[string]HostStats),
	}

	for key, hp := range p.clients {
		hp.mu.Lock()
		hs := HostStats{}
		for _, conn := range hp.connections {
			hs.Total++
			if conn.inUse {
				hs.InUse++
			} else {
				hs.Idle++
			}
		}
		stats.Hosts[key] = hs
		stats.TotalConnections += hs.Total
		hp.mu.Unlock()
	}

	return stats
}

// PoolStats contains pool statistics.
type PoolStats struct {
	TotalConnections int
	Hosts            map[string]HostStats
}

// HostStats contains per-host statistics.
type HostStats struct {
	Total int
	InUse int
	Idle  int
}

func (p *Pool) getFromHostPool(ctx context.Context, hp *hostPool, cfg *ClientConfig, key string) (*Client, error) {
	hp.mu.Lock()

	// Collect dead clients to remove from clientKeys after releasing hp.mu
	// This avoids holding hp.mu while acquiring p.mu (lock order issue)
	var deadClients []*Client

	// Try to find an idle connection
	for _, conn := range hp.connections {
		if !conn.inUse {
			// Check if connection is still healthy
			if conn.client.IsConnected() {
				conn.inUse = true
				conn.lastUsed = time.Now()
				hp.mu.Unlock()
				// Clean up any dead clients collected before finding healthy one
				if len(deadClients) > 0 {
					p.mu.Lock()
					for _, c := range deadClients {
						delete(p.clientKeys, c)
					}
					p.mu.Unlock()
				}
				return conn.client, nil
			}
			// Connection is dead - collect for cleanup after releasing hp.mu
			deadClients = append(deadClients, conn.client)
			conn.client.Close()
			hp.connections = removeConnection(hp.connections, conn)
		}
	}

	// Check if we can create a new connection
	if len(hp.connections) >= p.config.MaxPerHost {
		hp.mu.Unlock()
		// Clean up dead clients after releasing hp.mu
		if len(deadClients) > 0 {
			p.mu.Lock()
			for _, c := range deadClients {
				delete(p.clientKeys, c)
			}
			p.mu.Unlock()
		}
		return nil, fmt.Errorf("max connections (%d) reached for host %s", p.config.MaxPerHost, cfg.Host)
	}

	// Create new connection
	client := NewClient(cfg)
	conn := &pooledConnection{
		client:   client,
		lastUsed: time.Now(),
		inUse:    true,
		key:      key,
	}
	hp.connections = append(hp.connections, conn)
	hp.mu.Unlock()

	// Clean up dead clients after releasing hp.mu
	if len(deadClients) > 0 {
		p.mu.Lock()
		for _, c := range deadClients {
			delete(p.clientKeys, c)
		}
		p.mu.Unlock()
	}

	if err := client.Connect(ctx); err != nil {
		// Remove failed connection from pool
		hp.mu.Lock()
		hp.connections = removeConnection(hp.connections, conn)
		hp.mu.Unlock()
		return nil, err
	}

	// Add client to key mapping for O(1) release lookup
	p.mu.Lock()
	p.clientKeys[client] = key
	p.mu.Unlock()

	return client, nil
}

func (p *Pool) cleanupLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.closeCh:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *Pool) cleanup() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	now := time.Now()
	// Collect clients to remove from key map
	var clientsToRemove []*Client

	for _, hp := range p.clients {
		hp.mu.Lock()
		var remaining []*pooledConnection
		for _, conn := range hp.connections {
			// Close idle connections that have exceeded timeout
			if !conn.inUse && now.Sub(conn.lastUsed) > p.config.IdleTimeout {
				clientsToRemove = append(clientsToRemove, conn.client)
				conn.client.Close()
				continue
			}
			// Close dead connections
			if !conn.inUse && !conn.client.IsConnected() {
				clientsToRemove = append(clientsToRemove, conn.client)
				conn.client.Close()
				continue
			}
			remaining = append(remaining, conn)
		}
		hp.connections = remaining
		hp.mu.Unlock()
	}

	// Remove closed clients from key map
	for _, client := range clientsToRemove {
		delete(p.clientKeys, client)
	}
	p.mu.Unlock()
}

// configKey generates a unique key for a ClientConfig.
func (p *Pool) configKey(cfg *ClientConfig) string {
	// Hash of host + user + key file to identify unique connections
	data := fmt.Sprintf("%s:%s:%s", cfg.Host, cfg.User, cfg.KeyFile)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

func removeConnection(slice []*pooledConnection, conn *pooledConnection) []*pooledConnection {
	for i, c := range slice {
		if c == conn {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
