package agent

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestToProtoLogLevel(t *testing.T) {
	tests := []struct {
		input models.LogLevel
		want  blazelogv1.LogLevel
	}{
		{models.LevelDebug, blazelogv1.LogLevel_LOG_LEVEL_DEBUG},
		{models.LevelInfo, blazelogv1.LogLevel_LOG_LEVEL_INFO},
		{models.LevelWarning, blazelogv1.LogLevel_LOG_LEVEL_WARNING},
		{models.LevelError, blazelogv1.LogLevel_LOG_LEVEL_ERROR},
		{models.LevelFatal, blazelogv1.LogLevel_LOG_LEVEL_FATAL},
		{models.LevelUnknown, blazelogv1.LogLevel_LOG_LEVEL_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := ToProtoLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("ToProtoLogLevel(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToProtoLogType(t *testing.T) {
	tests := []struct {
		input models.LogType
		want  blazelogv1.LogType
	}{
		{models.LogTypeNginx, blazelogv1.LogType_LOG_TYPE_NGINX},
		{models.LogTypeApache, blazelogv1.LogType_LOG_TYPE_APACHE},
		{models.LogTypeMagento, blazelogv1.LogType_LOG_TYPE_MAGENTO},
		{models.LogTypePrestaShop, blazelogv1.LogType_LOG_TYPE_PRESTASHOP},
		{models.LogTypeWordPress, blazelogv1.LogType_LOG_TYPE_WORDPRESS},
		{models.LogTypeUnknown, blazelogv1.LogType_LOG_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := ToProtoLogType(tt.input)
			if got != tt.want {
				t.Errorf("ToProtoLogType(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToProtoLogEntry(t *testing.T) {
	now := time.Now()
	entry := &models.LogEntry{
		Timestamp:  now,
		Level:      models.LevelError,
		Message:    "test error message",
		Source:     "nginx-access",
		Type:       models.LogTypeNginx,
		Raw:        "raw log line",
		FilePath:   "/var/log/nginx/access.log",
		LineNumber: 42,
		Labels:     map[string]string{"env": "prod"},
		Fields:     map[string]interface{}{"status": 500},
	}

	proto := ToProtoLogEntry(entry)

	if proto.Level != blazelogv1.LogLevel_LOG_LEVEL_ERROR {
		t.Errorf("Level = %v, want ERROR", proto.Level)
	}
	if proto.Message != "test error message" {
		t.Errorf("Message = %v, want 'test error message'", proto.Message)
	}
	if proto.Source != "nginx-access" {
		t.Errorf("Source = %v, want 'nginx-access'", proto.Source)
	}
	if proto.Type != blazelogv1.LogType_LOG_TYPE_NGINX {
		t.Errorf("Type = %v, want NGINX", proto.Type)
	}
	if proto.FilePath != "/var/log/nginx/access.log" {
		t.Errorf("FilePath = %v, want '/var/log/nginx/access.log'", proto.FilePath)
	}
	if proto.LineNumber != 42 {
		t.Errorf("LineNumber = %v, want 42", proto.LineNumber)
	}
	if proto.Labels["env"] != "prod" {
		t.Errorf("Labels[env] = %v, want 'prod'", proto.Labels["env"])
	}
}

func TestToProtoLogEntryNil(t *testing.T) {
	proto := ToProtoLogEntry(nil)
	if proto != nil {
		t.Errorf("ToProtoLogEntry(nil) = %v, want nil", proto)
	}
}

func TestStringToLogType(t *testing.T) {
	tests := []struct {
		input string
		want  models.LogType
	}{
		{"nginx", models.LogTypeNginx},
		{"nginx-access", models.LogTypeNginx},
		{"nginx-error", models.LogTypeNginx},
		{"apache", models.LogTypeApache},
		{"magento", models.LogTypeMagento},
		{"prestashop", models.LogTypePrestaShop},
		{"wordpress", models.LogTypeWordPress},
		{"unknown", models.LogTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stringToLogType(tt.input)
			if got != tt.want {
				t.Errorf("stringToLogType(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCollector(t *testing.T) {
	// Create temp directory and log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Write nginx access log line
	logLine := `192.168.1.1 - - [14/Dec/2024:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`
	if err := os.WriteFile(logFile, []byte(logLine+"\n"), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	// Create collector
	src := SourceConfig{
		Name:   "test-nginx",
		Type:   "nginx",
		Path:   logFile,
		Follow: false,
	}
	collector, err := NewCollector(src, map[string]string{"test": "true"})
	if err != nil {
		t.Fatalf("NewCollector: %v", err)
	}

	// Start collector with context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := collector.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer collector.Stop()

	// Read entry
	select {
	case entry := <-collector.Entries():
		if entry == nil {
			t.Fatal("got nil entry")
		}
		if entry.Source != "test-nginx" {
			t.Errorf("Source = %v, want 'test-nginx'", entry.Source)
		}
		if entry.Labels["test"] != "true" {
			t.Errorf("Labels[test] = %v, want 'true'", entry.Labels["test"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for entry")
	}
}

func TestCollectorUnknownParser(t *testing.T) {
	src := SourceConfig{
		Name: "test",
		Type: "nonexistent-parser-type",
		Path: "/tmp/test.log",
	}
	_, err := NewCollector(src, nil)
	if err == nil {
		t.Fatal("expected error for unknown parser type")
	}
}

// mockLogServer implements LogServiceServer for testing.
type mockLogServer struct {
	blazelogv1.UnimplementedLogServiceServer
	registrations chan *blazelogv1.RegisterRequest
	batches       chan *blazelogv1.LogBatch
}

func newMockLogServer() *mockLogServer {
	return &mockLogServer{
		registrations: make(chan *blazelogv1.RegisterRequest, 10),
		batches:       make(chan *blazelogv1.LogBatch, 100),
	}
}

func (s *mockLogServer) Register(ctx context.Context, req *blazelogv1.RegisterRequest) (*blazelogv1.RegisterResponse, error) {
	s.registrations <- req
	return &blazelogv1.RegisterResponse{
		Success: true,
		AgentId: "test-agent-123",
	}, nil
}

func (s *mockLogServer) StreamLogs(stream blazelogv1.LogService_StreamLogsServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		s.batches <- batch
	}
}

func (s *mockLogServer) Heartbeat(ctx context.Context, req *blazelogv1.HeartbeatRequest) (*blazelogv1.HeartbeatResponse, error) {
	return &blazelogv1.HeartbeatResponse{
		Acknowledged: true,
	}, nil
}

func TestClientRegister(t *testing.T) {
	// Start mock server
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	mockServer := newMockLogServer()
	blazelogv1.RegisterLogServiceServer(server, mockServer)

	go server.Serve(lis)
	defer server.Stop()

	// Create client
	client, err := NewClient(lis.Addr().String())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	// Register
	ctx := context.Background()
	info := &blazelogv1.AgentInfo{
		AgentId:  "my-agent",
		Name:     "test-agent",
		Hostname: "localhost",
	}
	resp, err := client.Register(ctx, info)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if !resp.Success {
		t.Error("expected registration success")
	}
	if client.AgentID() != "test-agent-123" {
		t.Errorf("AgentID = %v, want 'test-agent-123'", client.AgentID())
	}

	// Verify registration was received
	select {
	case req := <-mockServer.registrations:
		if req.Agent.Name != "test-agent" {
			t.Errorf("registered name = %v, want 'test-agent'", req.Agent.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for registration")
	}
}

func TestClientSendBatch(t *testing.T) {
	// Start mock server
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	mockServer := newMockLogServer()
	blazelogv1.RegisterLogServiceServer(server, mockServer)

	go server.Serve(lis)
	defer server.Stop()

	// Create and connect client
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := &Client{
		conn:    conn,
		client:  blazelogv1.NewLogServiceClient(conn),
		agentID: "test-agent",
	}

	// Start stream
	ctx := context.Background()
	if err := client.StartStream(ctx); err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	// Send batch
	entries := []*blazelogv1.LogEntry{
		{Message: "test log 1"},
		{Message: "test log 2"},
	}
	if err := client.SendBatch(ctx, entries); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	// Verify batch was received
	select {
	case batch := <-mockServer.batches:
		if len(batch.Entries) != 2 {
			t.Errorf("batch size = %d, want 2", len(batch.Entries))
		}
		if batch.Entries[0].Message != "test log 1" {
			t.Errorf("entry 0 message = %v, want 'test log 1'", batch.Entries[0].Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for batch")
	}
}

func TestAgentConfig(t *testing.T) {
	cfg := &AgentConfig{
		ID:            "test-id",
		Name:          "test-agent",
		ServerAddress: "localhost:9443",
		Sources: []SourceConfig{
			{Name: "test", Type: "nginx", Path: "/var/log/test.log"},
		},
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Check defaults
	if agent.config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", agent.config.BatchSize)
	}
	if agent.config.FlushInterval != time.Second {
		t.Errorf("FlushInterval = %v, want 1s", agent.config.FlushInterval)
	}
}
