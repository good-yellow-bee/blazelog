package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"github.com/good-yellow-bee/blazelog/pkg/config"
)

// AgentConfig contains agent configuration.
type AgentConfig struct {
	ID            string
	Name          string
	ServerAddress string
	BatchSize     int
	FlushInterval time.Duration
	Sources       []SourceConfig
	Labels        map[string]string
	Verbose       bool
	TLS           *TLSConfig // nil = insecure mode
}

// Agent is the main BlazeLog agent.
type Agent struct {
	config     *AgentConfig
	client     *Client
	collectors []*Collector

	entriesChan chan *models.LogEntry
	batchBuffer []*blazelogv1.LogEntry

	mu     sync.Mutex
	closed bool
}

// New creates a new agent with the given configuration.
func New(cfg *AgentConfig) (*Agent, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}

	return &Agent{
		config:      cfg,
		entriesChan: make(chan *models.LogEntry, 1000),
		batchBuffer: make([]*blazelogv1.LogEntry, 0, cfg.BatchSize),
	}, nil
}

// Run starts the agent and blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	// Connect to server
	client, err := NewClient(a.config.ServerAddress, a.config.TLS)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	a.client = client
	defer a.client.Close()

	// Register with server
	if err := a.register(ctx); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Start log stream
	if err := a.client.StartStream(ctx); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

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

// register registers the agent with the server.
func (a *Agent) register(ctx context.Context) error {
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

	info := &blazelogv1.AgentInfo{
		AgentId:  a.config.ID,
		Name:     a.config.Name,
		Hostname: hostname,
		Version:  config.Version,
		Os:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Labels:   a.config.Labels,
		Sources:  sources,
	}

	resp, err := a.client.Register(ctx, info)
	if err != nil {
		return err
	}

	a.logf("registered with server, agent_id=%s", resp.AgentId)
	return nil
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

// flushBatch sends the current batch to the server.
func (a *Agent) flushBatch(ctx context.Context) {
	if len(a.batchBuffer) == 0 {
		return
	}

	batch := a.batchBuffer
	a.batchBuffer = make([]*blazelogv1.LogEntry, 0, a.config.BatchSize)

	if err := a.client.SendBatch(ctx, batch); err != nil {
		a.logf("error sending batch: %v", err)
		return
	}

	a.logf("sent batch of %d entries", len(batch))
}

// handleResponses processes responses from the server.
func (a *Agent) handleResponses(ctx context.Context) {
	responses, errs := a.client.ReceiveResponses(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-responses:
			if !ok {
				return
			}
			a.handleResponse(resp)
		case err, ok := <-errs:
			if !ok {
				return
			}
			a.logf("stream error: %v", err)
			return
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

	// Close client
	if a.client != nil {
		if err := a.client.Close(); err != nil {
			a.logf("error closing client: %v", err)
		}
	}

	return nil
}

// logf logs a message if verbose mode is enabled.
func (a *Agent) logf(format string, args ...interface{}) {
	if a.config.Verbose {
		log.Printf("[agent] "+format, args...)
	}
}
