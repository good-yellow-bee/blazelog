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

// createBenchEntries creates test log entries
func createBenchEntries(count int) []*blazelogv1.LogEntry {
	entries := make([]*blazelogv1.LogEntry, count)
	now := timestamppb.Now()
	for i := 0; i < count; i++ {
		entries[i] = &blazelogv1.LogEntry{
			Timestamp: now,
			Level:     blazelogv1.LogLevel_LOG_LEVEL_INFO,
			Message:   "Benchmark log message with some realistic content for testing purposes",
			Source:    "benchmark-agent",
			Type:      blazelogv1.LogType_LOG_TYPE_NGINX,
			Labels: map[string]string{
				"env":     "benchmark",
				"service": "test",
				"host":    "bench-host-1",
			},
		}
	}
	return entries
}

// startBenchServer starts a gRPC server for benchmarking
func startBenchServer(b *testing.B) (string, func()) {
	b.Helper()

	// Find available port
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start server
	cfg := &Config{
		GRPCAddress: addr,
		Verbose:     false,
	}
	srv, err := New(cfg)
	if err != nil {
		b.Fatalf("New server failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.Run(ctx)
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	cleanup := func() {
		cancel()
		<-serverDone
	}

	return addr, cleanup
}

// BenchmarkHandler_Register benchmarks agent registration
func BenchmarkHandler_Register(b *testing.B) {
	addr, cleanup := startBenchServer(b)
	defer cleanup()

	// Connect client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := blazelogv1.NewLogServiceClient(conn)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.Register(ctx, &blazelogv1.RegisterRequest{
			Agent: &blazelogv1.AgentInfo{
				Name:     "bench-agent",
				Hostname: "bench-host",
				Version:  "1.0.0",
			},
		})
		if err != nil {
			b.Fatalf("register failed: %v", err)
		}
	}
}

// BenchmarkHandler_Heartbeat benchmarks agent heartbeat
func BenchmarkHandler_Heartbeat(b *testing.B) {
	addr, cleanup := startBenchServer(b)
	defer cleanup()

	// Connect client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := blazelogv1.NewLogServiceClient(conn)
	ctx := context.Background()

	// Register first
	_, err = client.Register(ctx, &blazelogv1.RegisterRequest{
		Agent: &blazelogv1.AgentInfo{
			AgentId:  "heartbeat-agent",
			Name:     "Heartbeat Agent",
			Hostname: "bench-host",
			Version:  "1.0.0",
		},
	})
	if err != nil {
		b.Fatalf("register failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.Heartbeat(ctx, &blazelogv1.HeartbeatRequest{
			AgentId:   "heartbeat-agent",
			Timestamp: timestamppb.Now(),
			Status: &blazelogv1.AgentStatus{
				EntriesProcessed: 1000,
				BufferSize:       100,
				ActiveSources:    2,
				MemoryBytes:      50000000,
				CpuPercent:       5.0,
			},
		})
		if err != nil {
			b.Fatalf("heartbeat failed: %v", err)
		}
	}
}

// BenchmarkHandler_StreamLogs_SmallBatch benchmarks streaming with small batches
func BenchmarkHandler_StreamLogs_SmallBatch(b *testing.B) {
	benchmarkStreamLogs(b, 10)
}

// BenchmarkHandler_StreamLogs_MediumBatch benchmarks streaming with medium batches
func BenchmarkHandler_StreamLogs_MediumBatch(b *testing.B) {
	benchmarkStreamLogs(b, 100)
}

// BenchmarkHandler_StreamLogs_LargeBatch benchmarks streaming with large batches
func BenchmarkHandler_StreamLogs_LargeBatch(b *testing.B) {
	benchmarkStreamLogs(b, 1000)
}

func benchmarkStreamLogs(b *testing.B, batchSize int) {
	addr, cleanup := startBenchServer(b)
	defer cleanup()

	// Connect client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := blazelogv1.NewLogServiceClient(conn)
	ctx := context.Background()

	// Prepare test batch
	entries := createBenchEntries(batchSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stream, err := client.StreamLogs(ctx)
		if err != nil {
			b.Fatalf("failed to create stream: %v", err)
		}

		// Send batch
		batch := &blazelogv1.LogBatch{
			AgentId:  "bench-agent",
			Sequence: uint64(i + 1),
			Entries:  entries,
		}
		if err := stream.Send(batch); err != nil {
			b.Fatalf("failed to send: %v", err)
		}

		// Receive ack
		_, err = stream.Recv()
		if err != nil {
			b.Fatalf("failed to recv: %v", err)
		}

		// Close stream
		if err := stream.CloseSend(); err != nil {
			b.Fatalf("failed to close stream: %v", err)
		}
	}
}

// BenchmarkHandler_StreamLogs_Concurrent benchmarks concurrent agent streams
func BenchmarkHandler_StreamLogs_Concurrent(b *testing.B) {
	addr, cleanup := startBenchServer(b)
	defer cleanup()

	// Prepare test batch
	entries := createBenchEntries(100)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine creates its own connection
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return
		}
		defer conn.Close()

		client := blazelogv1.NewLogServiceClient(conn)
		ctx := context.Background()
		seq := uint64(0)

		for pb.Next() {
			seq++
			stream, err := client.StreamLogs(ctx)
			if err != nil {
				continue
			}

			batch := &blazelogv1.LogBatch{
				AgentId:  "concurrent-agent",
				Sequence: seq,
				Entries:  entries,
			}
			_ = stream.Send(batch)
			_, _ = stream.Recv()
			_ = stream.CloseSend()
		}
	})
}

// BenchmarkHandler_Throughput measures entries/sec throughput
func BenchmarkHandler_Throughput(b *testing.B) {
	addr, cleanup := startBenchServer(b)
	defer cleanup()

	// Connect client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := blazelogv1.NewLogServiceClient(conn)
	ctx := context.Background()

	// Large batch for throughput test
	batchSize := 1000
	entries := createBenchEntries(batchSize)

	b.ResetTimer()
	b.SetBytes(int64(batchSize)) // Report entries processed

	for i := 0; i < b.N; i++ {
		stream, err := client.StreamLogs(ctx)
		if err != nil {
			b.Fatalf("failed to create stream: %v", err)
		}

		batch := &blazelogv1.LogBatch{
			AgentId:  "throughput-agent",
			Sequence: uint64(i + 1),
			Entries:  entries,
		}
		if err := stream.Send(batch); err != nil {
			b.Fatalf("failed to send: %v", err)
		}

		_, err = stream.Recv()
		if err != nil {
			b.Fatalf("failed to recv: %v", err)
		}

		if err := stream.CloseSend(); err != nil {
			b.Fatalf("failed to close stream: %v", err)
		}
	}
}
