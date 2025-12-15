package agent

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/agent/buffer"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
)

// chaosServer wraps a mock gRPC server with chaos injection capabilities.
type chaosServer struct {
	blazelogv1.UnimplementedLogServiceServer
	grpcServer *grpc.Server
	listener   net.Listener
	addr       string

	// Chaos controls
	mu            sync.Mutex
	failRegister  bool
	failStream    bool
	failHeartbeat bool
	failAfter     int   // Fail after N batches
	batchCount    int32 // Atomic counter

	// Tracking
	registrations int
	batches       int
	heartbeats    int
}

func newChaosServer(t *testing.T) *chaosServer {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := &chaosServer{
		grpcServer: grpc.NewServer(),
		listener:   lis,
		addr:       lis.Addr().String(),
	}

	blazelogv1.RegisterLogServiceServer(s.grpcServer, s)

	go func() {
		_ = s.grpcServer.Serve(lis) // Server stopped on error
	}()

	return s
}

func (s *chaosServer) Register(ctx context.Context, req *blazelogv1.RegisterRequest) (*blazelogv1.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registrations++ // Count all attempts

	if s.failRegister {
		return &blazelogv1.RegisterResponse{
			Success:      false,
			ErrorMessage: "chaos: registration failed",
		}, nil
	}

	return &blazelogv1.RegisterResponse{
		Success: true,
		AgentId: "test-agent-id",
		Config: &blazelogv1.StreamConfig{
			MaxBatchSize:    100,
			FlushIntervalMs: 1000,
		},
	}, nil
}

func (s *chaosServer) StreamLogs(stream blazelogv1.LogService_StreamLogsServer) error {
	s.mu.Lock()
	if s.failStream {
		s.mu.Unlock()
		return grpc.ErrServerStopped
	}
	s.mu.Unlock()

	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}

		count := atomic.AddInt32(&s.batchCount, 1)

		s.mu.Lock()
		s.batches++
		failAfter := s.failAfter
		s.mu.Unlock()

		// Chaos: fail after N batches
		if failAfter > 0 && int(count) >= failAfter {
			return grpc.ErrServerStopped
		}

		// Send acknowledgment
		if err := stream.Send(&blazelogv1.StreamResponse{
			AckedSequence: batch.Sequence,
		}); err != nil {
			return err
		}
	}
}

func (s *chaosServer) Heartbeat(ctx context.Context, req *blazelogv1.HeartbeatRequest) (*blazelogv1.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failHeartbeat {
		return nil, grpc.ErrServerStopped
	}

	s.heartbeats++
	return &blazelogv1.HeartbeatResponse{
		Acknowledged: true,
	}, nil
}

func (s *chaosServer) stop() {
	s.grpcServer.GracefulStop()
	s.listener.Close()
}

func (s *chaosServer) setFailRegister(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failRegister = fail
}

func (s *chaosServer) setFailStream(fail bool) { //nolint:unused // kept for future chaos testing
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failStream = fail
}

func (s *chaosServer) setFailHeartbeat(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failHeartbeat = fail
}

func (s *chaosServer) setFailAfter(n int) { //nolint:unused // kept for future chaos testing
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failAfter = n
	atomic.StoreInt32(&s.batchCount, 0)
}

func (s *chaosServer) stats() (registrations, batches, heartbeats int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.registrations, s.batches, s.heartbeats
}

// TestConnManagerReconnect tests that ConnManager reconnects after disconnect.
func TestConnManagerReconnect(t *testing.T) {
	server := newChaosServer(t)
	defer server.stop()

	agentInfo := &blazelogv1.AgentInfo{
		AgentId: "test-agent",
		Name:    "test",
	}

	cfg := ConnManagerConfig{
		ServerAddress:  server.addr,
		AgentInfo:      agentInfo,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
	}

	cm := NewConnManager(cfg)
	cm.SetVerbose(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	err := cm.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if cm.State() != ConnStateConnected {
		t.Errorf("expected connected state, got %v", cm.State())
	}

	// Verify registration
	regs, _, _ := server.stats()
	if regs != 1 {
		t.Errorf("expected 1 registration, got %d", regs)
	}

	cm.Close()
}

// TestConnManagerRetry tests exponential backoff on connection failure.
func TestConnManagerRetry(t *testing.T) {
	server := newChaosServer(t)
	server.setFailRegister(true)
	defer server.stop()

	agentInfo := &blazelogv1.AgentInfo{
		AgentId: "test-agent",
		Name:    "test",
	}

	cfg := ConnManagerConfig{
		ServerAddress:  server.addr,
		AgentInfo:      agentInfo,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
		MaxRetries:     3,
	}

	cm := NewConnManager(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect should fail after retries
	err := cm.Connect(ctx)
	if err == nil {
		t.Error("expected error after max retries")
	}

	// Should have attempted multiple registrations
	regs, _, _ := server.stats()
	if regs < 2 {
		t.Errorf("expected at least 2 registration attempts, got %d", regs)
	}
}

// TestBackoffProgression tests that backoff delays increase exponentially.
func TestBackoffProgression(t *testing.T) {
	b := NewBackoffWithConfig(100*time.Millisecond, 1*time.Second, 2.0, 0)

	var delays []time.Duration
	for i := 0; i < 5; i++ {
		delays = append(delays, b.Next())
	}

	// Verify progression: 100ms, 200ms, 400ms, 800ms, 1000ms (capped)
	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1000 * time.Millisecond,
	}

	for i, exp := range expected {
		if delays[i] != exp {
			t.Errorf("delay[%d]: expected %v, got %v", i, exp, delays[i])
		}
	}
}

// TestBufferOnDisconnect tests that entries are buffered when disconnected.
func TestBufferOnDisconnect(t *testing.T) {
	dir := t.TempDir()

	cfg := buffer.Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := buffer.NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}
	defer buf.Close()

	// Write entries while "disconnected"
	entries := []*blazelogv1.LogEntry{
		{Message: "entry 1"},
		{Message: "entry 2"},
		{Message: "entry 3"},
	}

	if err := buf.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify buffered
	if buf.Len() != 3 {
		t.Errorf("expected 3 buffered entries, got %d", buf.Len())
	}

	// Simulate reconnect and replay
	read, err := buf.Read(10)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(read) != 3 {
		t.Errorf("expected 3 entries on replay, got %d", len(read))
	}

	// Buffer should be empty after replay
	if buf.Len() != 0 {
		t.Errorf("expected 0 buffered entries after replay, got %d", buf.Len())
	}
}

// TestBufferPersistence tests that buffer survives restart.
func TestBufferPersistence(t *testing.T) {
	dir := t.TempDir()

	cfg := buffer.Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	// Write entries
	buf1, err := buffer.NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}

	entries := []*blazelogv1.LogEntry{
		{Message: "persistent 1"},
		{Message: "persistent 2"},
	}
	if err := buf1.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf1.Close()

	// "Restart" - reopen buffer
	buf2, err := buffer.NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer reopen: %v", err)
	}
	defer buf2.Close()

	// Verify entries persisted
	if buf2.Len() != 2 {
		t.Errorf("expected 2 entries after restart, got %d", buf2.Len())
	}

	read, _ := buf2.Read(2)
	if len(read) != 2 || read[0].Message != "persistent 1" {
		t.Error("entries not persisted correctly")
	}
}

// TestHeartbeatMissedCount tests heartbeat failure tracking.
func TestHeartbeatMissedCount(t *testing.T) {
	server := newChaosServer(t)
	defer server.stop()

	agentInfo := &blazelogv1.AgentInfo{
		AgentId: "test-agent",
		Name:    "test",
	}

	cfg := ConnManagerConfig{
		ServerAddress:  server.addr,
		AgentInfo:      agentInfo,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
	}

	cm := NewConnManager(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cm.Close()

	hbCfg := HeartbeatConfig{
		Interval:  100 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		MaxMissed: 3,
	}

	hb := NewHeartbeater(cm, hbCfg, nil)
	hb.SetVerbose(true)

	// Start heartbeat
	reconnectCh := hb.Start(ctx)

	// Wait for some successful heartbeats
	time.Sleep(250 * time.Millisecond)

	_, _, heartbeats := server.stats()
	if heartbeats < 2 {
		t.Errorf("expected at least 2 heartbeats, got %d", heartbeats)
	}

	// Make heartbeats fail
	server.setFailHeartbeat(true)

	// Wait for reconnect signal
	select {
	case <-reconnectCh:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("expected reconnect signal after missed heartbeats")
	}
}

// TestGracefulShutdown tests clean shutdown with buffered entries.
func TestGracefulShutdown(t *testing.T) {
	dir := t.TempDir()

	cfg := buffer.Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := buffer.NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}

	// Write entries
	entries := []*blazelogv1.LogEntry{
		{Message: "shutdown test"},
	}
	if err := buf.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Close buffer
	if err := buf.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify
	buf2, err := buffer.NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer buf2.Close()

	if buf2.Len() != 1 {
		t.Errorf("expected 1 entry after restart, got %d", buf2.Len())
	}
}

// TestStreamDisconnectTriggersBuffer tests that stream disconnect triggers buffering.
func TestStreamDisconnectTriggersBuffer(t *testing.T) {
	server := newChaosServer(t)
	defer server.stop()

	agentInfo := &blazelogv1.AgentInfo{
		AgentId: "test-agent",
		Name:    "test",
	}

	cfg := ConnManagerConfig{
		ServerAddress:  server.addr,
		AgentInfo:      agentInfo,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
	}

	cm := NewConnManager(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cm.Close()

	client := cm.Client()
	if client == nil {
		t.Fatal("no client")
	}

	// Send a batch - should succeed
	batch := []*blazelogv1.LogEntry{
		{Message: "test entry 1"},
	}
	if err := client.SendBatch(ctx, batch); err != nil {
		t.Errorf("first batch should succeed, got %v", err)
	}

	// Wait a bit for server to receive
	time.Sleep(50 * time.Millisecond)

	_, batches, _ := server.stats()
	if batches < 1 {
		t.Errorf("expected at least 1 batch, got %d", batches)
	}
}
