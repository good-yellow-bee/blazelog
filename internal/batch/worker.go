package batch

import (
	"context"
	"runtime"
	"sync"
)

// WorkerPool manages parallel file processing.
type WorkerPool struct {
	workers    int
	bufferSize int
	jobs       chan string
	results    chan *FileStats
	errors     chan error
	wg         sync.WaitGroup
	started    bool
	mu         sync.Mutex
}

// NewWorkerPool creates a pool with N workers.
// If workers <= 0, defaults to runtime.NumCPU().
func NewWorkerPool(workers, bufferSize int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if bufferSize <= 0 {
		bufferSize = workers * 2
	}

	return &WorkerPool{
		workers:    workers,
		bufferSize: bufferSize,
		jobs:       make(chan string, bufferSize),
		results:    make(chan *FileStats, bufferSize),
		errors:     make(chan error, bufferSize),
	}
}

// Start begins worker goroutines with the given processor function.
// The processor receives a file path and returns stats or error.
func (p *WorkerPool) Start(ctx context.Context, processor func(context.Context, string) (*FileStats, error)) {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	p.mu.Unlock()

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case path, ok := <-p.jobs:
					if !ok {
						return
					}
					stats, err := processor(ctx, path)
					if err != nil {
						select {
						case p.errors <- err:
						case <-ctx.Done():
							return
						}
						continue
					}
					select {
					case p.results <- stats:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
}

// Submit adds a file path to the job queue.
// Returns error if pool is closed or context canceled.
func (p *WorkerPool) Submit(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.jobs <- path:
		return nil
	}
}

// Close signals no more jobs and waits for completion.
// Closes results and errors channels after all workers finish.
func (p *WorkerPool) Close() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
	close(p.errors)
}

// Results returns the results channel.
func (p *WorkerPool) Results() <-chan *FileStats {
	return p.results
}

// Errors returns the errors channel.
func (p *WorkerPool) Errors() <-chan error {
	return p.errors
}

// Workers returns the number of workers.
func (p *WorkerPool) Workers() int {
	return p.workers
}
