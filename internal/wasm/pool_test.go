package wasm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Plan for WASMWorkerPool:
// - Can initialize with min and max workers
// - Validates configuration parameters correctly
// - Pre-warms minimum workers on startup
// - Invoke returns output from an idle worker
// - Creates new workers up to max when needed
// - Blocks if all workers are busy and max reached
// - Releases workers back to pool after use
// - Respects context cancellation during acquire
// - Respects context cancellation during invoke
// - Shutdown closes all workers gracefully
// - Handles errors during worker creation
// - Handles errors during worker invocation
// - Tracks active worker count correctly
// - Handles concurrent access safely
// - Prevents use after shutdown

// Mock implementations for testing

type mockWASMWorker struct {
	invokeFunc func(ctx context.Context, method string, input []byte) ([]byte, error)
	closeFunc  func(ctx context.Context) error
	closed     bool
	mu         sync.Mutex
}

func (m *mockWASMWorker) Invoke(ctx context.Context, method string, input []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("worker is closed")
	}
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, method, input)
	}
	return []byte("mock result"), nil
}

func (m *mockWASMWorker) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

type mockWASMCompiledModule struct {
	instantiateFunc func(ctx context.Context) (WASMWorker, error)
	closeFunc       func(ctx context.Context) error
	workerCount     int32
	mu              sync.Mutex
}

func (m *mockWASMCompiledModule) Instantiate(ctx context.Context) (WASMWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.instantiateFunc != nil {
		return m.instantiateFunc(ctx)
	}
	return &mockWASMWorker{}, nil
}

func (m *mockWASMCompiledModule) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

// Test: Initializing with valid min/max workers should succeed
// Test: Pre-warming minimum workers on startup should succeed
func TestWASMWorkerPool_ValidConfiguration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	config := WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 5,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, pool)

	// Should pre-warm minimum workers
	assert.Equal(t, uint(0), pool.ActiveWorkers())

	// Clean up
	require.NoError(t, pool.Shutdown(ctx))
}

// Test: Initializing with negative min workers should return error
// Test: Initializing with zero max workers should return error
// Test: Initializing with min workers greater than max workers should return error
// Test: Initializing with nil module should return error
func TestWASMWorkerPool_InvalidConfigurations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	tests := []struct {
		name   string
		config WASMWorkerPoolConfig
		errMsg string
	}{
		{
			name: "negative min workers",
			config: WASMWorkerPoolConfig{
				MinWorkers: -1,
				MaxWorkers: 5,
				Module:     module,
			},
			errMsg: "min workers cannot be negative",
		},
		{
			name: "zero max workers",
			config: WASMWorkerPoolConfig{
				MinWorkers: 0,
				MaxWorkers: 0,
				Module:     module,
			},
			errMsg: "max workers must be at least 1",
		},
		{
			name: "min greater than max",
			config: WASMWorkerPoolConfig{
				MinWorkers: 5,
				MaxWorkers: 2,
				Module:     module,
			},
			errMsg: "min workers cannot be greater than max workers",
		},
		{
			name: "nil module",
			config: WASMWorkerPoolConfig{
				MinWorkers: 1,
				MaxWorkers: 5,
				Module:     nil,
			},
			errMsg: "module cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pool, err := NewWASMWorkerPool(ctx, tt.config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, pool)
		})
	}
}

// Test: Invoke should return result from idle worker
func TestWASMWorkerPool_InvokeWithIdleWorker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	config := WASMWorkerPoolConfig{
		MinWorkers: 1,
		MaxWorkers: 3,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	result, err := pool.Invoke(ctx, "test", []byte("input"))
	require.NoError(t, err)
	assert.Equal(t, []byte("mock result"), result)
}

// Test: Invoke should create new worker when pool not at max capacity
func TestWASMWorkerPool_InvokeCreatesNewWorker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	config := WASMWorkerPoolConfig{
		MinWorkers: 0,
		MaxWorkers: 2,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Should create new worker since pool starts empty
	result, err := pool.Invoke(ctx, "test", []byte("input"))
	require.NoError(t, err)
	assert.Equal(t, []byte("mock result"), result)
}

// Test: Invoke should block when all workers busy and at max capacity
func TestWASMWorkerPool_InvokeBlocksWhenMaxReached(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a worker that blocks until signaled
	blockCh := make(chan struct{})
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					<-blockCh
					return []byte("result"), nil
				},
			}, nil
		},
	}

	config := WASMWorkerPoolConfig{
		MinWorkers: 0,
		MaxWorkers: 1,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Start first invocation that will block
	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		_, err := pool.Invoke(ctx, "test", []byte("input"))
		require.NoError(t, err)
	}()

	// Give first goroutine time to acquire the worker
	time.Sleep(10 * time.Millisecond)

	// Second invocation should block
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err = pool.Invoke(ctxWithTimeout, "test", []byte("input"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	// Unblock first invocation
	close(blockCh)
	<-done1
}

// Test: ActiveWorkers should return correct count
// Test: ActiveWorkers should track concurrent worker usage
func TestWASMWorkerPool_ActiveWorkers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a worker that blocks until signaled
	blockCh := make(chan struct{})
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					<-blockCh
					return []byte("result"), nil
				},
			}, nil
		},
	}

	config := WASMWorkerPoolConfig{
		MinWorkers: 0,
		MaxWorkers: 2,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	assert.Equal(t, uint(0), pool.ActiveWorkers())

	// Start first invocation
	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		pool.Invoke(ctx, "test", []byte("input"))
	}()

	// Give time for worker to be acquired
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, uint(1), pool.ActiveWorkers())

	// Start second invocation
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		pool.Invoke(ctx, "test", []byte("input"))
	}()

	// Give time for second worker to be acquired
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, uint(2), pool.ActiveWorkers())

	// Unblock workers
	close(blockCh)
	<-done1
	<-done2

	// Give time for workers to be released
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, uint(0), pool.ActiveWorkers())
}

// Test: Invoke should respect context cancellation during acquire
// Test: Invoke should respect context cancellation during worker invoke
func TestWASMWorkerPool_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a worker that blocks indefinitely
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				},
			}, nil
		},
	}

	config := WASMWorkerPoolConfig{
		MinWorkers: 0,
		MaxWorkers: 1,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Test context cancellation during invoke
	ctxWithCancel, cancel := context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err = pool.Invoke(ctxWithCancel, "test", []byte("input"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	<-done
}

// Test: Shutdown should close all idle workers
// Test: Shutdown should prevent further invocations
// Test: Shutdown should be idempotent
// Test: Invoke should return error after shutdown
func TestWASMWorkerPool_Shutdown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	config := WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 5,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)

	// Shutdown should succeed
	err = pool.Shutdown(ctx)
	require.NoError(t, err)

	// Subsequent invocations should fail
	_, err = pool.Invoke(ctx, "test", []byte("input"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker pool is shut down")

	// Shutdown should be idempotent
	err = pool.Shutdown(ctx)
	require.NoError(t, err)
}

// Test: Worker creation error should be handled gracefully
func TestWASMWorkerPool_WorkerCreationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedErr := errors.New("worker creation failed")
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			return nil, expectedErr
		},
	}

	config := WASMWorkerPoolConfig{
		MinWorkers: 1,
		MaxWorkers: 2,
		Module:     module,
	}

	// Should fail during initialization due to pre-warming
	pool, err := NewWASMWorkerPool(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker creation failed")
	assert.Nil(t, pool)
}

// Test: Worker invocation error should be propagated
func TestWASMWorkerPool_WorkerInvocationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedErr := errors.New("invocation failed")
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					return nil, expectedErr
				},
			}, nil
		},
	}

	config := WASMWorkerPoolConfig{
		MinWorkers: 1,
		MaxWorkers: 2,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	_, err = pool.Invoke(ctx, "test", []byte("input"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invocation failed")
}

// Test: Concurrent access should be safe
func TestWASMWorkerPool_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module := &mockWASMCompiledModule{}

	config := WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 10,
		Module:     module,
	}

	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Run multiple concurrent invocations
	const numGoroutines = 20
	done := make(chan struct{}, numGoroutines)

	for range numGoroutines {
		go func() {
			defer func() { done <- struct{}{} }()

			result, err := pool.Invoke(ctx, "test", []byte("input"))
			assert.NoError(t, err)
			assert.Equal(t, []byte("mock result"), result)
		}()
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// All workers should be returned to pool
	assert.Equal(t, uint(0), pool.ActiveWorkers())
}

// Test: Race conditions with aggressive concurrent operations
func TestWASMWorkerPool_RaceConditions(t *testing.T) {
	// Run with: go test -race
	t.Parallel()
	
	ctx := context.Background()
	
	// Create module with variable delays
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			// Add small random delay
			time.Sleep(time.Duration(time.Now().UnixNano()%1000) * time.Nanosecond)
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					// Variable processing time
					time.Sleep(time.Duration(time.Now().UnixNano()%1000) * time.Nanosecond)
					return []byte("result"), nil
				},
			}, nil
		},
	}
	
	config := WASMWorkerPoolConfig{
		MinWorkers: 1,
		MaxWorkers: 5,
		Module:     module,
	}
	
	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	
	// Launch multiple concurrent operations
	var wg sync.WaitGroup
	stopFlag := make(chan struct{})
	
	// Rapid invoke operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopFlag:
					return
				default:
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					pool.Invoke(ctx, "test", []byte{byte(id)})
					cancel()
				}
			}
		}(i)
	}
	
	// Concurrent metrics access
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopFlag:
					return
				default:
					pool.ActiveWorkers()
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}
	
	// Run for a short time
	time.Sleep(100 * time.Millisecond)
	close(stopFlag)
	
	// Concurrent shutdown attempts
	shutdownDone := make(chan error, 3)
	for range 3 {
		go func() {
			shutdownDone <- pool.Shutdown(ctx)
		}()
	}
	
	// Wait for operations to complete
	wg.Wait()
	
	// Collect shutdown results
	var shutdownErrors []error
	for range 3 {
		if err := <-shutdownDone; err != nil {
			shutdownErrors = append(shutdownErrors, err)
		}
	}
	
	// Should handle concurrent shutdown gracefully
	assert.Empty(t, shutdownErrors)
}

// Test: Context cancellation during various stages
func TestWASMWorkerPool_ContextCancellationStress(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	
	// Module with controlled delays
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Millisecond):
				return &mockWASMWorker{
					invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						case <-time.After(5 * time.Millisecond):
							return []byte("result"), nil
						}
					},
				}, nil
			}
		},
	}
	
	config := WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 8,
		Module:     module,
	}
	
	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	defer pool.Shutdown(ctx)
	
	var wg sync.WaitGroup
	successCount := int32(0)
	cancelCount := int32(0)
	
	// Launch requests with varying cancellation patterns
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(delay int) {
			defer wg.Done()
			
			ctx, cancel := context.WithCancel(context.Background())
			
			// Cancel after variable delay
			if delay > 0 {
				go func() {
					time.Sleep(time.Duration(delay) * time.Millisecond)
					cancel()
				}()
			} else {
				defer cancel()
			}
			
			_, err := pool.Invoke(ctx, "test", []byte("data"))
			if err == context.Canceled {
				atomic.AddInt32(&cancelCount, 1)
			} else if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i % 10) // Delay from 0-9ms
	}
	
	wg.Wait()
	
	t.Logf("Success: %d, Cancelled: %d", successCount, cancelCount)
	
	// Should have a mix of both
	assert.Greater(t, int(successCount), 0)
	assert.Greater(t, int(cancelCount), 0)
	
	// Pool should remain functional
	result, err := pool.Invoke(ctx, "final", []byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("result"), result)
}

// Test: Pool behavior under memory pressure
func TestWASMWorkerPool_MemoryPressure(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	
	// Track worker creation/destruction
	var created int32
	var destroyed int32
	
	module := &mockWASMCompiledModule{
		instantiateFunc: func(ctx context.Context) (WASMWorker, error) {
			atomic.AddInt32(&created, 1)
			return &mockWASMWorker{
				invokeFunc: func(ctx context.Context, method string, input []byte) ([]byte, error) {
					// Add some delay to make workers busy
					time.Sleep(10 * time.Millisecond)
					return []byte("result"), nil
				},
				closeFunc: func(ctx context.Context) error {
					atomic.AddInt32(&destroyed, 1)
					return nil
				},
			}, nil
		},
	}
	
	config := WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 20, // Large max to test scaling
		Module:     module,
	}
	
	pool, err := NewWASMWorkerPool(ctx, config)
	require.NoError(t, err)
	
	// Burst of requests to force scaling
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			pool.Invoke(ctx, "test", []byte("data"))
		}()
	}
	
	wg.Wait()
	
	createdCount := atomic.LoadInt32(&created)
	t.Logf("Workers created: %d", createdCount)
	
	// Should have created more than min but reasonable number
	assert.Greater(t, int(createdCount), 2)
	assert.LessOrEqual(t, int(createdCount), 20)
	
	// Shutdown and verify cleanup
	err = pool.Shutdown(ctx)
	assert.NoError(t, err)
	
	destroyedCount := atomic.LoadInt32(&destroyed)
	assert.Equal(t, createdCount, destroyedCount, "All created workers should be destroyed")
}
