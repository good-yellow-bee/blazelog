package server

import (
	"context"
	"net"
	"testing"
	"time"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestServerIntegration(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start server
	cfg := &Config{
		GRPCAddress: addr,
		Verbose:     true,
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New server failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.Run(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := blazelogv1.NewLogServiceClient(conn)

	// Test Register
	t.Run("Register", func(t *testing.T) {
		resp, err := client.Register(ctx, &blazelogv1.RegisterRequest{
			Agent: &blazelogv1.AgentInfo{
				AgentId:  "test-agent-1",
				Name:     "Test Agent",
				Hostname: "localhost",
				Version:  "1.0.0",
				Os:       "linux",
				Arch:     "amd64",
				Sources: []*blazelogv1.LogSource{
					{Name: "nginx", Path: "/var/log/nginx/access.log"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
		if !resp.Success {
			t.Fatalf("Register not successful: %s", resp.ErrorMessage)
		}
		if resp.AgentId != "test-agent-1" {
			t.Errorf("expected agent ID 'test-agent-1', got '%s'", resp.AgentId)
		}
		if resp.Config == nil {
			t.Error("expected config in response")
		}
	})

	// Test StreamLogs
	t.Run("StreamLogs", func(t *testing.T) {
		stream, err := client.StreamLogs(ctx)
		if err != nil {
			t.Fatalf("StreamLogs failed: %v", err)
		}

		// Send a batch
		batch := &blazelogv1.LogBatch{
			AgentId:  "test-agent-1",
			Sequence: 1,
			Entries: []*blazelogv1.LogEntry{
				{
					Timestamp: timestamppb.Now(),
					Level:     blazelogv1.LogLevel_LOG_LEVEL_INFO,
					Message:   "Test log message",
					Source:    "test-source",
					Type:      blazelogv1.LogType_LOG_TYPE_NGINX,
				},
				{
					Timestamp: timestamppb.Now(),
					Level:     blazelogv1.LogLevel_LOG_LEVEL_ERROR,
					Message:   "Test error message",
					Source:    "test-source",
					Type:      blazelogv1.LogType_LOG_TYPE_NGINX,
				},
			},
		}

		if err := stream.Send(batch); err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Receive acknowledgement
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		if resp.AckedSequence != 1 {
			t.Errorf("expected acked sequence 1, got %d", resp.AckedSequence)
		}
		if resp.Error != "" {
			t.Errorf("unexpected error: %s", resp.Error)
		}

		// Close stream
		if err := stream.CloseSend(); err != nil {
			t.Fatalf("CloseSend failed: %v", err)
		}
	})

	// Test Heartbeat
	t.Run("Heartbeat", func(t *testing.T) {
		resp, err := client.Heartbeat(ctx, &blazelogv1.HeartbeatRequest{
			AgentId:   "test-agent-1",
			Timestamp: timestamppb.Now(),
			Status: &blazelogv1.AgentStatus{
				EntriesProcessed: 100,
				BufferSize:       10,
				ActiveSources:    1,
			},
		})
		if err != nil {
			t.Fatalf("Heartbeat failed: %v", err)
		}
		if !resp.Acknowledged {
			t.Error("expected heartbeat to be acknowledged")
		}
	})

	// Verify stats
	batches, entries, _, agents := srv.Stats()
	if batches != 1 {
		t.Errorf("expected 1 batch, got %d", batches)
	}
	if entries != 2 {
		t.Errorf("expected 2 entries, got %d", entries)
	}
	if agents != 1 {
		t.Errorf("expected 1 agent, got %d", agents)
	}

	// Shutdown
	cancel()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Errorf("server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server shutdown timeout")
	}
}

func TestHandler_RegisterWithoutAgentInfo(t *testing.T) {
	processor := NewProcessor(false)
	handler := NewHandler(processor, false)

	resp, err := handler.Register(context.Background(), &blazelogv1.RegisterRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success {
		t.Error("expected registration to fail without agent info")
	}
	if resp.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestHandler_RegisterGeneratesAgentID(t *testing.T) {
	processor := NewProcessor(false)
	handler := NewHandler(processor, false)

	resp, err := handler.Register(context.Background(), &blazelogv1.RegisterRequest{
		Agent: &blazelogv1.AgentInfo{
			Name:     "Test Agent",
			Hostname: "localhost",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected registration to succeed: %s", resp.ErrorMessage)
	}
	if resp.AgentId == "" {
		t.Error("expected generated agent ID")
	}
}

func TestProcessor_FormatEntry(t *testing.T) {
	processor := NewProcessor(false)

	entry := &blazelogv1.LogEntry{
		Timestamp: timestamppb.Now(),
		Level:     blazelogv1.LogLevel_LOG_LEVEL_ERROR,
		Message:   "Test error",
		Source:    "nginx",
	}

	output := processor.formatEntry(entry, "agent-123")
	if output == "" {
		t.Error("expected non-empty output")
	}
	// Should contain the message
	if !contains(output, "Test error") {
		t.Errorf("output should contain message: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
