package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/agent/buffer"
	"github.com/good-yellow-bee/blazelog/internal/models"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"github.com/good-yellow-bee/blazelog/pkg/config"
)

// Config contains agent configuration.
type Config struct {
	ID            string
	Name          string
	ProjectID     string // Project this agent belongs to
	ServerAddress string
	BatchSize     int
	FlushInterval time.Duration
	Sources       []SourceConfig
	Labels        map[string]string
	Verbose       bool
	TLS           *TLSConfig // nil = insecure mode

	// Reliability settings
	BufferDir         string        // Buffer directory (default: ~/.blazelog/buffer)
	BufferMaxSize     int64         // Max buffer size in bytes (default: 100MB)
	HeartbeatInterval time.Duration // Heartbeat interval (default: 15s)
	ReconnectInitial  time.Duration // Initial reconnect delay (default: 1s)
	ReconnectMax      time.Duration // Max reconnect delay (default: 30s)
}

// Agent is the main BlazeLog agent with reliability features.
type Agent struct {
	config      *Config
	connMgr     *ConnManager
	heartbeater *Heartbeater
	buffer      buffer.Buffer
	collectors  []*Collector

	entriesChan chan *models.LogEntry
	batchBuffer []*blazelogv1.LogEntry

	// Metrics for heartbeat status
	entriesProcessed uint64
	entriesSent      uint64
	errorCount       uint64

	mu     sync.Mutex
	closed bool
}

// New creates a new agent with the given configuration.
func New(cfg *Config) (*Agent, error) {
	// Apply defaults
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.ReconnectInitial <= 0 {
		cfg.ReconnectInitial = time.Second
	}
	if cfg.ReconnectMax <= 0 {
		cfg.ReconnectMax = 30 * time.Second
	}

	// Initialize buffer
	bufCfg := buffer.DefaultConfig()
	if cfg.BufferDir != "" {
		bufCfg.Dir = cfg.BufferDir
	}
	if cfg.BufferMaxSize > 0 {
		bufCfg.MaxSize = cfg.BufferMaxSize
	}

	buf, err := buffer.NewDiskBuffer(bufCfg)
	if err != nil {
		return nil, fmt.Errorf("create buffer: %w", err)
	}

	return &Agent{
		config:      cfg,
		buffer:      buf,
		entriesChan: make(chan *models.LogEntry, 1000),
		batchBuffer: make([]*blazelogv1.LogEntry, 0, cfg.BatchSize),
	}, nil
}

// Run starts the agent and blocks until the context is canceled.
func (a *Agent) Run(ctx context.Context) error {
	defer a.buffer.Close()

	// Check for buffered entries from previous run
	if a.buffer.Len() > 0 {
		a.logf("found %d buffered entries from previous run", a.buffer.Len())
	}

	// Build agent info for registration
	agentInfo := a.buildAgentInfo()

	// Create connection manager
	connCfg := ConnManagerConfig{
		ServerAddress:  a.config.ServerAddress,
		TLS:            a.config.TLS,
		AgentInfo:      agentInfo,
		InitialBackoff: a.config.ReconnectInitial,
		MaxBackoff:     a.config.ReconnectMax,
	}
	a.connMgr = NewConnManager(connCfg)
	a.connMgr.SetVerbose(a.config.Verbose)
	a.connMgr.SetCallbacks(
		func() { a.onConnected(ctx) },
		func(err error) { a.onDisconnected(err) },
		func(state ConnState) { a.logf("connection state: %s", state) },
	)

	// Initial connect with retry
	if err := a.connMgr.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer a.connMgr.Close()

	// Start heartbeat
	hbCfg := HeartbeatConfig{
		Interval:  a.config.HeartbeatInterval,
		Timeout:   5 * time.Second,
		MaxMissed: 3,
	}
	a.heartbeater = NewHeartbeater(a.connMgr, hbCfg, a.buildStatus)
	a.heartbeater.SetVerbose(a.config.Verbose)
	hbReconnectCh := a.heartbeater.Start(ctx)

	// Start reconnection loop
	go a.connMgr.RunReconnectLoop(ctx)

	// Handle heartbeat-triggered reconnects
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hbReconnectCh:
				a.connMgr.TriggerReconnect()
			}
		}
	}()

	// Create and start collectors
	if err := a.startCollectors(ctx); err != nil {
		return fmt.Errorf("start collectors: %w", err)
	}

	// Start batch sender
	go a.batchSender(ctx)

	// Start response handler
	go a.handleResponses(ctx)

	// Merge collector entries into single channel
	go a.mergeEntries(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	a.logf("shutting down...")
	return a.Stop()
}

// buildAgentInfo creates the AgentInfo for registration.
func (a *Agent) buildAgentInfo() *blazelogv1.AgentInfo {
	hostname, _ := os.Hostname()

	sources := make([]*blazelogv1.LogSource, len(a.config.Sources))
	for i, src := range a.config.Sources {
		sources[i] = &blazelogv1.LogSource{
			Name:   src.Name,
			Path:   src.Path,
			Type:   src.Type,
			Follow: src.Follow,
		}
	}

	return &blazelogv1.AgentInfo{
		AgentId:   a.config.ID,
		Name:      a.config.Name,
		ProjectId: a.config.ProjectID,
		Hostname:  hostname,
		Version:   config.Version,
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Labels:    a.config.Labels,
		Sources:   sources,
	}
}

// buildStatus creates the AgentStatus for heartbeats.
func (a *Agent) buildStatus() *blazelogv1.AgentStatus {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &blazelogv1.AgentStatus{
		EntriesProcessed: atomic.LoadUint64(&a.entriesProcessed),
		BufferSize:       uint64(a.buffer.Len()),
		ActiveSources:    int32(len(a.collectors)),
		MemoryBytes:      memStats.Alloc,
	}
}

// onConnected is called when connection is established.
func (a *Agent) onConnected(ctx context.Context) {
	a.logf("connected, replaying %d buffered entries...", a.buffer.Len())

	// Replay buffered entries
	replayed := 0
	for a.buffer.Len() > 0 {
		entries, err := a.buffer.Read(a.config.BatchSize)
		if err != nil || len(entries) == 0 {
			break
		}

		client := a.connMgr.Client()
		if client == nil {
			// Re-buffer if no client
			a.buffer.Write(entries)
			break
		}

		if err := client.SendBatch(ctx, entries); err != nil {
			a.logf("replay failed, re-buffering: %v", err)
			a.buffer.Write(entries)
			break
		}

		replayed += len(entries)
		atomic.AddUint64(&a.entriesSent, uint64(len(entries)))
	}

	if replayed > 0 {
		a.logf("replayed %d buffered entries", replayed)
	}
}

// onDisconnected is called when connection is lost.
func (a *Agent) onDisconnected(err error) {
	atomic.AddUint64(&a.errorCount, 1)
	a.logf("disconnected: %v, buffering logs...", err)
}

// startCollectors creates and starts all log collectors.
func (a *Agent) startCollectors(ctx context.Context) error {
	for _, src := range a.config.Sources {
		collector, err := NewCollector(src, a.config.Labels)
		if err != nil {
			return fmt.Errorf("create collector for %s: %w", src.Name, err)
		}

		if err := collector.Start(ctx); err != nil {
			return fmt.Errorf("start collector for %s: %w", src.Name, err)
		}

		a.collectors = append(a.collectors, collector)
		a.logf("started collector: %s (%s)", src.Name, src.Path)
	}

	return nil
}

// mergeEntries reads from all collector channels and sends to a single channel.
func (a *Agent) mergeEntries(ctx context.Context) {
	var wg sync.WaitGroup

	for _, c := range a.collectors {
		wg.Add(1)
		go func(collector *Collector) {
			defer wg.Done()
			for entry := range collector.Entries() {
				atomic.AddUint64(&a.entriesProcessed, 1)
				select {
				case a.entriesChan <- entry:
				case <-ctx.Done():
					return
				}
			}
		}(c)
	}

	wg.Wait()
	close(a.entriesChan)
}

// batchSender batches log entries and sends them to the server.
func (a *Agent) batchSender(ctx context.Context) {
	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush
			a.flushBatch(ctx)
			return

		case entry, ok := <-a.entriesChan:
			if !ok {
				a.flushBatch(ctx)
				return
			}

			protoEntry := ToProtoLogEntry(entry)
			a.batchBuffer = append(a.batchBuffer, protoEntry)

			if len(a.batchBuffer) >= a.config.BatchSize {
				a.flushBatch(ctx)
			}

		case <-ticker.C:
			if len(a.batchBuffer) > 0 {
				a.flushBatch(ctx)
			}
		}
	}
}

// flushBatch sends the current batch to the server or buffers on failure.
func (a *Agent) flushBatch(ctx context.Context) {
	if len(a.batchBuffer) == 0 {
		return
	}

	batch := a.batchBuffer
	a.batchBuffer = make([]*blazelogv1.LogEntry, 0, a.config.BatchSize)

	// Try to send if connected
	if a.connMgr != nil && a.connMgr.IsConnected() {
		client := a.connMgr.Client()
		if client != nil {
			if err := client.SendBatch(ctx, batch); err != nil {
				a.logf("send failed, buffering %d entries: %v", len(batch), err)
				atomic.AddUint64(&a.errorCount, 1)
				// Buffer on failure
				if err := a.buffer.Write(batch); err != nil {
					a.logf("buffer write failed: %v", err)
				}
				// Trigger reconnect
				a.connMgr.TriggerReconnect()
				return
			}

			atomic.AddUint64(&a.entriesSent, uint64(len(batch)))
			a.logf("sent batch of %d entries", len(batch))
			return
		}
	}

	// Not connected: buffer entries
	if err := a.buffer.Write(batch); err != nil {
		a.logf("buffer write failed: %v", err)
	} else {
		a.logf("buffered %d entries (disconnected)", len(batch))
	}
}

// handleResponses processes responses from the server.
func (a *Agent) handleResponses(ctx context.Context) {
	for {
		client := a.connMgr.Client()
		if client == nil {
			// Wait for connection
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		responses, errs := client.ReceiveResponses(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-responses:
				if !ok {
					// Channel closed, reconnect
					goto reconnect
				}
				a.handleResponse(resp)
			case err, ok := <-errs:
				if !ok {
					goto reconnect
				}
				a.logf("stream error: %v", err)
				a.connMgr.TriggerReconnect()
				goto reconnect
			}
		}

	reconnect:
		// Wait before retrying
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// handleResponse processes a single response from the server.
func (a *Agent) handleResponse(resp *blazelogv1.StreamResponse) {
	if resp.Command != nil {
		a.handleCommand(resp.Command)
	}
}

// handleCommand handles server commands.
func (a *Agent) handleCommand(cmd *blazelogv1.ServerCommand) {
	switch cmd.Type {
	case blazelogv1.CommandType_COMMAND_TYPE_UNSPECIFIED:
		a.logf("received unspecified command")
	case blazelogv1.CommandType_COMMAND_TYPE_SHUTDOWN:
		a.logf("received shutdown command")
		// TODO: Trigger graceful shutdown
	case blazelogv1.CommandType_COMMAND_TYPE_PAUSE:
		a.logf("received pause command")
		// TODO: Pause collection
	case blazelogv1.CommandType_COMMAND_TYPE_RESUME:
		a.logf("received resume command")
		// TODO: Resume collection
	case blazelogv1.CommandType_COMMAND_TYPE_RELOAD_CONFIG:
		a.logf("received reload config command")
		// TODO: Reload configuration
	default:
		a.logf("received unknown command: %v", cmd.Type)
	}
}

// Stop stops the agent.
func (a *Agent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// Stop collectors
	for _, c := range a.collectors {
		c.Stop()
	}

	// Close connection manager
	if a.connMgr != nil {
		a.connMgr.Close()
	}

	// Close buffer (already deferred in Run, but just in case)
	if a.buffer != nil {
		a.buffer.Close()
	}

	return nil
}

// BufferLen returns the current buffer length.
func (a *Agent) BufferLen() int {
	if a.buffer == nil {
		return 0
	}
	return a.buffer.Len()
}

// Stats returns current agent statistics.
func (a *Agent) Stats() (processed, sent, errors uint64) {
	return atomic.LoadUint64(&a.entriesProcessed),
		atomic.LoadUint64(&a.entriesSent),
		atomic.LoadUint64(&a.errorCount)
}

// logf logs a message if verbose mode is enabled.
func (a *Agent) logf(format string, args ...interface{}) {
	if a.config.Verbose {
		log.Printf("[agent] "+format, args...)
	}
}
