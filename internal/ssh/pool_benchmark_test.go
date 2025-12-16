package ssh

import (
	"sync"
	"testing"
	"time"
)

// BenchmarkPool_Stats measures lock acquisition for Stats() method.
// This tests the read path of pool locks.
func BenchmarkPool_Stats(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.Stats()
	}
}

// BenchmarkPool_Stats_Concurrent measures lock contention under concurrent Stats() calls.
// Validates that the lock implementation scales with parallel readers.
func BenchmarkPool_Stats_Concurrent(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Stats()
		}
	})
}

// BenchmarkPool_ReleaseUnknown measures the Release path for unknown clients.
// This exercises the clientKeys map lookup (O(1) operation added in perf fix).
func BenchmarkPool_ReleaseUnknown(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "benchmark.example.com:22",
		User:    "bench",
		KeyFile: "/nonexistent/key",
	}

	// Pre-create clients to benchmark Release
	clients := make([]*Client, b.N)
	for i := 0; i < b.N; i++ {
		clients[i] = NewClient(cfg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Release(clients[i])
	}
}

// BenchmarkPool_ReleaseUnknown_Concurrent measures concurrent Release operations.
// Tests the lock order fix under parallel access - validates no deadlocks occur.
func BenchmarkPool_ReleaseUnknown_Concurrent(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "benchmark.example.com:22",
		User:    "bench",
		KeyFile: "/nonexistent/key",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client := NewClient(cfg)
			pool.Release(client)
		}
	})
}

// BenchmarkPool_ConfigKey measures the key generation performance.
// This is called on every Get/Release, so it should be fast.
func BenchmarkPool_ConfigKey(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "benchmark.example.com:22",
		User:    "benchuser",
		KeyFile: "/path/to/benchmark/key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.configKey(cfg)
	}
}

// BenchmarkPool_MixedOperations simulates realistic mixed workload.
// Combines Stats reads with Release operations to test lock contention.
func BenchmarkPool_MixedOperations(b *testing.B) {
	pool := NewPool(DefaultPoolConfig())
	defer pool.Close()

	cfg := &ClientConfig{
		Host:    "benchmark.example.com:22",
		User:    "bench",
		KeyFile: "/nonexistent/key",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				// 1/3 of operations are releases
				client := NewClient(cfg)
				pool.Release(client)
			} else {
				// 2/3 are stats reads
				_ = pool.Stats()
			}
			i++
		}
	})
}

// BenchmarkPool_HighContention simulates high lock contention scenario.
// Multiple goroutines competing for pool operations.
func BenchmarkPool_HighContention(b *testing.B) {
	pool := NewPool(PoolConfig{
		MaxPerHost:          2, // Low limit to increase contention
		IdleTimeout:         time.Hour,
		HealthCheckInterval: time.Hour,
	})
	defer pool.Close()

	var wg sync.WaitGroup
	numGoroutines := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(numGoroutines)
		for g := 0; g < numGoroutines; g++ {
			go func(id int) {
				defer wg.Done()
				cfg := &ClientConfig{
					Host:    "benchmark.example.com:22",
					User:    "bench",
					KeyFile: "/nonexistent/key",
				}
				client := NewClient(cfg)
				pool.Release(client)
				_ = pool.Stats()
			}(g)
		}
		wg.Wait()
	}
}
