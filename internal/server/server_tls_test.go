package server

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/agent"
	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"github.com/good-yellow-bee/blazelog/internal/security"
)

func TestServerClientMTLS(t *testing.T) {
	// Setup test certificates
	tmpDir := t.TempDir()

	// Generate CA
	if err := security.GenerateCA(tmpDir, 365); err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Generate server cert
	if err := security.GenerateServerCert(tmpDir, "server", tmpDir, 365, nil); err != nil {
		t.Fatalf("GenerateServerCert failed: %v", err)
	}

	// Generate agent cert
	if err := security.GenerateAgentCert(tmpDir, "agent", tmpDir, 365); err != nil {
		t.Fatalf("GenerateAgentCert failed: %v", err)
	}

	// Find free port
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Create server with TLS
	serverCfg := &Config{
		GRPCAddress: addr,
		Verbose:     true,
		TLS: &TLSConfig{
			CertFile:     filepath.Join(tmpDir, "server.crt"),
			KeyFile:      filepath.Join(tmpDir, "server.key"),
			ClientCAFile: filepath.Join(tmpDir, "ca.crt"),
		},
	}

	srv, err := New(serverCfg)
	if err != nil {
		t.Fatalf("New server failed: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.Run(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client with TLS
	clientTLS := &agent.TLSConfig{
		CertFile: filepath.Join(tmpDir, "agent.crt"),
		KeyFile:  filepath.Join(tmpDir, "agent.key"),
		CAFile:   filepath.Join(tmpDir, "ca.crt"),
	}

	client, err := agent.NewClient(addr, clientTLS)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// Test registration
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	agentInfo := &blazelogv1.AgentInfo{
		Name:     "test-agent",
		Hostname: "localhost",
		Version:  "test",
		Os:       "linux",
		Arch:     "amd64",
	}

	resp, err := client.Register(clientCtx, agentInfo)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("registration not successful: %s", resp.ErrorMessage)
	}

	if resp.AgentId == "" {
		t.Error("expected non-empty agent ID")
	}

	// Shutdown
	cancel()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Logf("server exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server did not shutdown in time")
	}
}

func TestServerRejectsInvalidClientCert(t *testing.T) {
	// Setup test certificates
	tmpDir := t.TempDir()

	// Generate CA
	if err := security.GenerateCA(tmpDir, 365); err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// Generate server cert
	if err := security.GenerateServerCert(tmpDir, "server", tmpDir, 365, nil); err != nil {
		t.Fatalf("GenerateServerCert failed: %v", err)
	}

	// Generate a DIFFERENT CA and agent cert (should be rejected)
	otherCADir := t.TempDir()
	if err := security.GenerateCA(otherCADir, 365); err != nil {
		t.Fatalf("GenerateCA (other) failed: %v", err)
	}
	if err := security.GenerateAgentCert(otherCADir, "bad-agent", otherCADir, 365); err != nil {
		t.Fatalf("GenerateAgentCert (other) failed: %v", err)
	}

	// Find free port
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Create server with TLS (using first CA)
	serverCfg := &Config{
		GRPCAddress: addr,
		Verbose:     false,
		TLS: &TLSConfig{
			CertFile:     filepath.Join(tmpDir, "server.crt"),
			KeyFile:      filepath.Join(tmpDir, "server.key"),
			ClientCAFile: filepath.Join(tmpDir, "ca.crt"),
		},
	}

	srv, err := New(serverCfg)
	if err != nil {
		t.Fatalf("New server failed: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		srv.Run(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client with cert from DIFFERENT CA (should fail)
	clientTLS := &agent.TLSConfig{
		CertFile:           filepath.Join(otherCADir, "bad-agent.crt"),
		KeyFile:            filepath.Join(otherCADir, "bad-agent.key"),
		CAFile:             filepath.Join(tmpDir, "ca.crt"), // Use server's CA to verify server
		InsecureSkipVerify: true,                            // Skip server verification for this test
	}

	client, err := agent.NewClient(addr, clientTLS)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// Test registration - should fail due to certificate verification
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	agentInfo := &blazelogv1.AgentInfo{
		Name:     "bad-agent",
		Hostname: "localhost",
		Version:  "test",
		Os:       "linux",
		Arch:     "amd64",
	}

	_, err = client.Register(clientCtx, agentInfo)
	if err == nil {
		t.Error("expected Register to fail with invalid client certificate")
	}

	cancel()
}

func TestServerInsecureMode(t *testing.T) {
	// Find free port
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Create server WITHOUT TLS
	serverCfg := &Config{
		GRPCAddress: addr,
		Verbose:     false,
		TLS:         nil, // No TLS
	}

	srv, err := New(serverCfg)
	if err != nil {
		t.Fatalf("New server failed: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		srv.Run(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client WITHOUT TLS
	client, err := agent.NewClient(addr, nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// Test registration
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	agentInfo := &blazelogv1.AgentInfo{
		Name:     "insecure-agent",
		Hostname: "localhost",
		Version:  "test",
		Os:       "linux",
		Arch:     "amd64",
	}

	resp, err := client.Register(clientCtx, agentInfo)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("registration not successful: %s", resp.ErrorMessage)
	}

	cancel()
}
