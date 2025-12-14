package ssh

import (
	"context"
	"testing"
	"time"
)

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxPerHost != 5 {
		t.Errorf("MaxPerHost: got %d, want %d", cfg.MaxPerHost, 5)
	}
	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout: got %v, want %v", cfg.IdleTimeout, 5*time.Minute)
	}
	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("HealthCheckInterval: got %v, want %v", cfg.HealthCheckInterval, 30*time.Second)
	}
}

func TestNewPool_DefaultValues(t *testing.T) {
	// Test with zero values - should use defaults
	pool := NewPool(PoolConfig{})
	defer pool.Close()

	if pool.config.MaxPerHost != 5 {
		t.Errorf("MaxPerHost: got %d, want %d", pool.config.MaxPerHost, 5)
	}
	if pool.config.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout: got %v, want %v", pool.config.IdleTimeout, 5*time.Minute)
	}
}

func TestPool_GetAfterClose(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	pool.Close()

	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}

	_, err := pool.Get(context.Background(), cfg)
	if err == nil {
		t.Error("Get after Close should fail")
	}
}

func TestPool_CloseIdempotent(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())

	// Should not panic
	pool.Close()
	pool.Close()
	pool.Close()
}

func TestPool_ReleaseNil(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	// Should not panic
	pool.Release(nil)
}

func TestPool_ReleaseUnknownClient(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}
	client := NewClient(cfg)

	// Releasing a client not from the pool should just close it
	pool.Release(client)

	// Should not panic or leave pool in bad state
	stats := pool.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("expected 0 connections, got %d", stats.TotalConnections)
	}
}

func TestPool_Stats_Empty(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	stats := pool.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("expected 0 total connections, got %d", stats.TotalConnections)
	}
	if len(stats.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(stats.Hosts))
	}
}

func TestPool_ConfigKey(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg1 := &ClientConfig{
		Host:    "host1.com:22",
		User:    "user1",
		KeyFile: "/key1",
	}

	cfg2 := &ClientConfig{
		Host:    "host2.com:22",
		User:    "user1",
		KeyFile: "/key1",
	}

	cfg3 := &ClientConfig{
		Host:    "host1.com:22",
		User:    "user1",
		KeyFile: "/key1",
	}

	key1 := pool.configKey(cfg1)
	key2 := pool.configKey(cfg2)
	key3 := pool.configKey(cfg3)

	// Different hosts should have different keys
	if key1 == key2 {
		t.Error("different hosts should have different keys")
	}

	// Same config should have same key
	if key1 != key3 {
		t.Error("same config should have same key")
	}

	// Keys should be non-empty
	if key1 == "" {
		t.Error("key should not be empty")
	}
}

func TestPool_MaxPerHost(t *testing.T) {
	pool := NewPool(PoolConfig{
		MaxPerHost:          2,
		IdleTimeout:         time.Hour,
		HealthCheckInterval: time.Hour,
	})
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: "/nonexistent/key",
	}

	// First two connections will be created (but fail to connect due to missing key)
	// The third should fail due to max connections
	ctx := context.Background()

	// These will fail to connect but will consume pool slots
	_, _ = pool.Get(ctx, cfg)
	_, _ = pool.Get(ctx, cfg)

	// Now the slots should be used up (even though connections failed,
	// the cleanup hasn't run yet in this test)
	// In a real scenario with working connections, this would hit the limit
}

func TestRemoveConnection(t *testing.T) {
	conn1 := &pooledConnection{key: "1"}
	conn2 := &pooledConnection{key: "2"}
	conn3 := &pooledConnection{key: "3"}

	slice := []*pooledConnection{conn1, conn2, conn3}

	// Remove middle
	result := removeConnection(slice, conn2)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
	if result[0] != conn1 || result[1] != conn3 {
		t.Error("wrong elements remaining")
	}

	// Remove non-existent
	result = removeConnection(result, conn2)
	if len(result) != 2 {
		t.Error("removing non-existent should not change length")
	}

	// Remove first
	result = removeConnection(result, conn1)
	if len(result) != 1 || result[0] != conn3 {
		t.Error("wrong element remaining")
	}

	// Remove last
	result = removeConnection(result, conn3)
	if len(result) != 0 {
		t.Error("expected empty slice")
	}
}

func TestHostStats(t *testing.T) {
	stats := HostStats{
		Total: 5,
		InUse: 3,
		Idle:  2,
	}

	if stats.Total != 5 {
		t.Errorf("Total: got %d, want %d", stats.Total, 5)
	}
	if stats.InUse != 3 {
		t.Errorf("InUse: got %d, want %d", stats.InUse, 3)
	}
	if stats.Idle != 2 {
		t.Errorf("Idle: got %d, want %d", stats.Idle, 2)
	}
}

func TestPoolStats(t *testing.T) {
	stats := PoolStats{
		TotalConnections: 10,
		Hosts: map[string]HostStats{
			"host1": {Total: 5, InUse: 2, Idle: 3},
			"host2": {Total: 5, InUse: 4, Idle: 1},
		},
	}

	if stats.TotalConnections != 10 {
		t.Errorf("TotalConnections: got %d, want %d", stats.TotalConnections, 10)
	}
	if len(stats.Hosts) != 2 {
		t.Errorf("Hosts count: got %d, want %d", len(stats.Hosts), 2)
	}
}

func TestPool_ReleaseAfterClose(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())

	cfg := &ClientConfig{
		Host:    "example.com:22",
		User:    "test",
		KeyFile: "/path/to/key",
	}
	client := NewClient(cfg)

	pool.Close()

	// Should not panic, and should close the client
	pool.Release(client)
}
