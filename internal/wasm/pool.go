package wasm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// WASMWorkerPool manages a pool of pre-initialized WASM workers created from a shared compiled module.
type WASMWorkerPool interface {
	// Invokes a method on an available worker.
	// Blocks until a worker is available or context is canceled.
	Invoke(ctx context.Context, method string, input []byte) ([]byte, error)

	// Returns the number of currently active (in-use) workers.
	ActiveWorkers() uint

	// Gracefully shuts down the pool, cleaning up all idle workers.
	Shutdown(ctx context.Context) error
}

// WASMWorker represents a single WASM worker instance.
type WASMWorker interface {
	Invoke(ctx context.Context, method string, input []byte) ([]byte, error)
	Close(ctx context.Context) error
}

// WASMCompiledModule represents a compiled WASM module that can create worker instances.
type WASMCompiledModule interface {
	// Instantiate creates a new isolated WASMWorker.
	// Each worker is backed by a fresh module instance.
	Instantiate(ctx context.Context) (WASMWorker, error)

	// Close cleans up any resources tied to the compiled module.
	Close(ctx context.Context) error
}

// WASMWorkerPoolConfig holds configuration for the worker pool.
type WASMWorkerPoolConfig struct {
	MinWorkers int
	MaxWorkers int
	Module     WASMCompiledModule
}

// NewWASMWorkerPool creates a new worker pool with the given configuration.
func NewWASMWorkerPool(ctx context.Context, config WASMWorkerPoolConfig) (WASMWorkerPool, error) {
	if config.MinWorkers < 0 {
		return nil, errors.New("min workers cannot be negative")
	}
	if config.MaxWorkers < 1 {
		return nil, errors.New("max workers must be at least 1")
	}
	if config.MinWorkers > config.MaxWorkers {
		return nil, errors.New("min workers cannot be greater than max workers")
	}
	if config.Module == nil {
		return nil, errors.New("module cannot be nil")
	}

	pool := &wasmWorkerPool{
		minWorkers:    config.MinWorkers,
		maxWorkers:    config.MaxWorkers,
		module:        config.Module,
		idleWorkers:   make(chan WASMWorker, config.MaxWorkers),
		workerCount:   0,
		activeWorkers: 0,
		shutdown:      make(chan struct{}),
	}

	// Pre-warm minimum workers
	for range config.MinWorkers {
		worker, err := config.Module.Instantiate(ctx)
		if err != nil {
			// Clean up any workers created so far
			pool.cleanupWorkers(ctx)
			return nil, err
		}
		pool.idleWorkers <- worker
		pool.workerCount++
	}

	return pool, nil
}

type wasmWorkerPool struct {
	minWorkers    int
	maxWorkers    int
	module        WASMCompiledModule
	idleWorkers   chan WASMWorker
	workerCount   int32
	activeWorkers int32
	mu            sync.RWMutex
	shutdown      chan struct{}
	shutdownOnce  sync.Once
}

func (p *wasmWorkerPool) Invoke(ctx context.Context, method string, input []byte) ([]byte, error) {
	select {
	case <-p.shutdown:
		return nil, errors.New("worker pool is shut down")
	default:
	}

	worker, err := p.acquireWorker(ctx)
	if err != nil {
		return nil, err
	}

	defer p.releaseWorker(worker)

	return worker.Invoke(ctx, method, input)
}

func (p *wasmWorkerPool) ActiveWorkers() uint {
	return uint(atomic.LoadInt32(&p.activeWorkers))
}

func (p *wasmWorkerPool) Shutdown(ctx context.Context) error {
	var shutdownErr error
	p.shutdownOnce.Do(func() {
		close(p.shutdown)
		shutdownErr = p.cleanupWorkers(ctx)
	})
	return shutdownErr
}

func (p *wasmWorkerPool) acquireWorker(ctx context.Context) (WASMWorker, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.shutdown:
		return nil, errors.New("worker pool is shut down")
	case worker := <-p.idleWorkers:
		atomic.AddInt32(&p.activeWorkers, 1)
		return worker, nil
	default:
		// No idle workers available, try to create a new one
		if atomic.LoadInt32(&p.workerCount) < int32(p.maxWorkers) {
			worker, err := p.module.Instantiate(ctx)
			if err != nil {
				return nil, err
			}
			atomic.AddInt32(&p.workerCount, 1)
			atomic.AddInt32(&p.activeWorkers, 1)
			return worker, nil
		}

		// Wait for a worker to become available
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.shutdown:
			return nil, errors.New("worker pool is shut down")
		case worker := <-p.idleWorkers:
			atomic.AddInt32(&p.activeWorkers, 1)
			return worker, nil
		}
	}
}

func (p *wasmWorkerPool) releaseWorker(worker WASMWorker) {
	atomic.AddInt32(&p.activeWorkers, -1)
	select {
	case p.idleWorkers <- worker:
		// Worker returned to pool
	default:
		// Pool is full or shutting down, close the worker
		worker.Close(context.Background())
		atomic.AddInt32(&p.workerCount, -1)
	}
}

func (p *wasmWorkerPool) cleanupWorkers(ctx context.Context) error {
	var lastErr error
	
	// Close all idle workers
	for {
		select {
		case worker := <-p.idleWorkers:
			if err := worker.Close(ctx); err != nil {
				lastErr = err
			}
		default:
			return lastErr
		}
	}
}