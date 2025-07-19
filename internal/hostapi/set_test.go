package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Test Plan:
// 1. Test basic execute functionality
// 2. Test policy enforcement (allow and deny)
// 3. Test telemetry recording
// 4. Test streaming API with iterator management
// 5. Test iterator lifecycle (next, close, cleanup)
// 6. Test resource limits (max iterators)
// 7. Test concurrent access
// 8. Test closing behavior and idempotency

// Test: Basic execute functionality
func TestHostAPISet_Execute(t *testing.T) {
	// Create a mock API
	api := &mockHostAPI{
		name:    "test.api",
		version: "v1.0.0",
		methods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error){
			"echo": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				return params, nil
			},
		},
	}

	// Create host API set
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.api": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:       slog.Default(),
			Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
			Meter:        metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine: &mockPolicyEngine{}, // Default allows all
		},
		closed: false,
	}

	// Test successful execution
	params := json.RawMessage(`{"message":"hello"}`)
	result, err := apiSet.Execute(context.Background(), "test.api", "echo", params)
	require.NoError(t, err)
	assert.Equal(t, params, result)
}

// Test: Execute on closed set
func TestHostAPISet_ExecuteOnClosed(t *testing.T) {
	apiSet := &defaultHostAPISet{
		apis:      make(map[string]HostAPI),
		iterators: make(map[string]*iteratorInfo),
		config:    HostAPIConfig{},
		closed:    true,
	}

	_, err := apiSet.Execute(context.Background(), "test.api", "method", nil)
	require.Error(t, err)

	var apiErr *HostAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrorCodeHostAPISetClosed, apiErr.Code)
}

// Test: Execute with unknown API
func TestHostAPISet_ExecuteUnknownAPI(t *testing.T) {
	apiSet := &defaultHostAPISet{
		apis:      make(map[string]HostAPI),
		iterators: make(map[string]*iteratorInfo),
		config:    HostAPIConfig{},
		closed:    false,
	}

	_, err := apiSet.Execute(context.Background(), "unknown.api", "method", nil)
	require.Error(t, err)

	var apiErr *HostAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrorCodeAPINotFound, apiErr.Code)
}

// Test: Policy enforcement
func TestHostAPISet_PolicyEnforcement(t *testing.T) {
	// Create API
	api := &mockHostAPI{
		name:    "test.api",
		version: "v1.0.0",
		methods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error){
			"allowed": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"result":"success"}`), nil
			},
			"denied": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"result":"should not see this"}`), nil
			},
		},
	}

	// Create policy engine
	policyEngine := &mockPolicyEngine{
		decisions: map[string]PolicyDecision{
			"test.api.allowed": {
				Allowed: true,
			},
			"test.api.denied": {
				Allowed: false,
				Reason:  "access denied by policy",
			},
		},
	}

	// Create host API set
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.api": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:       slog.Default(),
			Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
			Meter:        metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine: policyEngine,
		},
		closed: false,
	}

	// Add service info to context
	ctx := context.WithValue(context.Background(), serviceInfoKey{}, ServiceInfo{
		Name:    "test-service",
		Version: "v1.0.0",
	})

	// Test allowed method
	t.Run("allowed method", func(t *testing.T) {
		result, err := apiSet.Execute(ctx, "test.api", "allowed", nil)
		require.NoError(t, err)

		var resp map[string]string
		require.NoError(t, json.Unmarshal(result, &resp))
		assert.Equal(t, "success", resp["result"])
	})

	// Test denied method
	t.Run("denied method", func(t *testing.T) {
		_, err := apiSet.Execute(ctx, "test.api", "denied", nil)
		require.Error(t, err)

		var apiErr *HostAPIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, ErrorCodePolicyDenied, apiErr.Code)
		assert.Equal(t, "access denied by policy", apiErr.Message)
	})
}

// Test: Policy evaluation error
func TestHostAPISet_PolicyError(t *testing.T) {
	// Create API
	api := &mockHostAPI{
		name:    "test.api",
		version: "v1.0.0",
		methods: make(map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)),
	}

	// Create failing policy engine
	policyEngine := &mockPolicyEngine{
		err: errors.New("policy evaluation failed"),
	}

	// Create host API set
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.api": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:       slog.Default(),
			Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
			Meter:        metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine: policyEngine,
		},
		closed: false,
	}

	_, err := apiSet.Execute(context.Background(), "test.api", "method", nil)
	require.Error(t, err)

	var apiErr *HostAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrorCodePolicyError, apiErr.Code)
}

// Test: Streaming API with iterator
func TestHostAPISet_StreamingAPI(t *testing.T) {
	// Create streaming API
	iteratorData := []json.RawMessage{
		json.RawMessage(`{"item":1}`),
		json.RawMessage(`{"item":2}`),
		json.RawMessage(`{"item":3}`),
	}

	api := &mockStreamingAPI{
		mockHostAPI: mockHostAPI{
			name:    "test.streaming",
			version: "v1.0.0",
			methods: make(map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)),
		},
		streamingMethods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error){
			"list": func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error) {
				iterator := &mockIterator{
					data:  iteratorData,
					index: 0,
				}

				resp := StreamingResponse{
					IteratorID: "test-iter-123",
					HasData:    true,
				}
				respJSON, _ := json.Marshal(resp)

				return respJSON, iterator, nil
			},
		},
	}

	// Create host API set
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.streaming": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:                 slog.Default(),
			Tracer:                 tracenoop.NewTracerProvider().Tracer("test"),
			Meter:                  metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine:           &mockPolicyEngine{},
			MaxIteratorsPerService: 10,
		},
		closed: false,
	}

	// Execute streaming method
	result, err := apiSet.Execute(context.Background(), "test.streaming", "list", nil)
	require.NoError(t, err)

	var resp StreamingResponse
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.Equal(t, "test-iter-123", resp.IteratorID)

	// Verify iterator was registered
	apiSet.mu.RLock()
	assert.Len(t, apiSet.iterators, 1)
	assert.Contains(t, apiSet.iterators, resp.IteratorID)
	apiSet.mu.RUnlock()

	// Use the iterator
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		data, hasMore, err := apiSet.NextIterator(ctx, resp.IteratorID)
		require.NoError(t, err)

		var item map[string]int
		require.NoError(t, json.Unmarshal(data, &item))
		assert.Equal(t, i+1, item["item"])

		if i < 2 {
			assert.True(t, hasMore)
		} else {
			assert.False(t, hasMore)
		}
	}

	// Iterator should be auto-cleaned up after exhaustion
	apiSet.mu.RLock()
	assert.Len(t, apiSet.iterators, 0)
	apiSet.mu.RUnlock()
}

// Test: Iterator not found
func TestHostAPISet_IteratorNotFound(t *testing.T) {
	apiSet := &defaultHostAPISet{
		apis:      make(map[string]HostAPI),
		iterators: make(map[string]*iteratorInfo),
		config:    HostAPIConfig{},
		closed:    false,
	}

	_, _, err := apiSet.NextIterator(context.Background(), "non-existent-iter")
	require.Error(t, err)

	var apiErr *HostAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrorCodeIteratorNotFound, apiErr.Code)
}

// Test: Max iterators limit
func TestHostAPISet_MaxIteratorsLimit(t *testing.T) {
	// Create streaming API that always returns new iterators
	api := &mockStreamingAPI{
		mockHostAPI: mockHostAPI{
			name:    "test.streaming",
			version: "v1.0.0",
		},
		streamingMethods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error){
			"list": func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error) {
				iterator := &mockIterator{
					data: []json.RawMessage{json.RawMessage(`{"item":1}`)},
				}

				resp := StreamingResponse{
					IteratorID: generateIteratorID(),
					HasData:    true,
				}
				respJSON, _ := json.Marshal(resp)

				return respJSON, iterator, nil
			},
		},
	}

	// Create host API set with low max iterators
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.streaming": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:                 slog.Default(),
			Tracer:                 tracenoop.NewTracerProvider().Tracer("test"),
			Meter:                  metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine:           &mockPolicyEngine{},
			MaxIteratorsPerService: 2,
		},
		closed: false,
	}

	// Create first two iterators (should succeed)
	for i := 0; i < 2; i++ {
		_, err := apiSet.Execute(context.Background(), "test.streaming", "list", nil)
		require.NoError(t, err)
	}

	// Third iterator should fail
	_, err := apiSet.Execute(context.Background(), "test.streaming", "list", nil)
	require.Error(t, err)

	var apiErr *HostAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, ErrorCodeIteratorLimitExceeded, apiErr.Code)
}

// Test: Close iterator
func TestHostAPISet_CloseIterator(t *testing.T) {
	closeCalled := false
	iterator := &mockIterator{
		data: []json.RawMessage{json.RawMessage(`{"item":1}`)},
		closeFunc: func() error {
			closeCalled = true
			return nil
		},
	}

	apiSet := &defaultHostAPISet{
		apis: make(map[string]HostAPI),
		iterators: map[string]*iteratorInfo{
			"test-iter": {
				iterator:  iterator,
				apiName:   "test.api",
				method:    "list",
				createdAt: time.Now(),
			},
		},
		config: HostAPIConfig{
			Logger: slog.Default(),
		},
		closed: false,
	}

	// Close iterator
	err := apiSet.CloseIterator(context.Background(), "test-iter")
	require.NoError(t, err)
	assert.True(t, closeCalled)

	// Verify iterator was removed
	apiSet.mu.RLock()
	assert.Len(t, apiSet.iterators, 0)
	apiSet.mu.RUnlock()

	// Closing again should be no-op
	err = apiSet.CloseIterator(context.Background(), "test-iter")
	require.NoError(t, err)
}

// Test: Cleanup stale iterators
func TestHostAPISet_CleanupStaleIterators(t *testing.T) {
	now := time.Now()

	// Create iterators with different ages
	apiSet := &defaultHostAPISet{
		apis: make(map[string]HostAPI),
		iterators: map[string]*iteratorInfo{
			"fresh": {
				iterator:  &mockIterator{},
				createdAt: now.Add(-1 * time.Minute),
			},
			"stale1": {
				iterator:  &mockIterator{},
				createdAt: now.Add(-10 * time.Minute),
			},
			"stale2": {
				iterator:  &mockIterator{},
				createdAt: now.Add(-15 * time.Minute),
			},
		},
		config: HostAPIConfig{
			IteratorTimeout: 5 * time.Minute,
			Logger:          slog.Default(),
		},
		closed: false,
	}

	// Clean up stale iterators
	cleaned := apiSet.CleanupStaleIterators()
	assert.Equal(t, 2, cleaned)

	// Verify only fresh iterator remains
	apiSet.mu.RLock()
	assert.Len(t, apiSet.iterators, 1)
	assert.Contains(t, apiSet.iterators, "fresh")
	apiSet.mu.RUnlock()
}

// Test: Concurrent access
func TestHostAPISet_ConcurrentAccess(t *testing.T) {
	// Create API with methods
	api := &mockHostAPI{
		name:    "test.api",
		version: "v1.0.0",
		methods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error){
			"method": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				// Simulate some work
				time.Sleep(time.Millisecond)
				return json.RawMessage(`{"result":"ok"}`), nil
			},
		},
	}

	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"test.api": api,
		},
		iterators: make(map[string]*iteratorInfo),
		config: HostAPIConfig{
			Logger:       slog.Default(),
			Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
			Meter:        metricnoop.NewMeterProvider().Meter("test"),
			PolicyEngine: &mockPolicyEngine{},
		},
		closed: false,
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// Concurrent executions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := apiSet.Execute(context.Background(), "test.api", "method", nil)
			if err != nil {
				errs <- err
			}
		}()
	}

	// Concurrent iterator operations
	// First create an iterator
	streamingAPI := &mockStreamingAPI{
		mockHostAPI: mockHostAPI{
			name:    "test.streaming",
			version: "v1.0.0",
		},
		streamingMethods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error){
			"list": func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error) {
				iterator := &mockIterator{
					data: []json.RawMessage{
						json.RawMessage(`{"item":1}`),
						json.RawMessage(`{"item":2}`),
					},
				}
				resp := StreamingResponse{
					IteratorID: "shared-iter",
					HasData:    true,
				}
				respJSON, _ := json.Marshal(resp)
				return respJSON, iterator, nil
			},
		},
	}
	apiSet.apis["test.streaming"] = streamingAPI

	// Create iterator
	_, err := apiSet.Execute(context.Background(), "test.streaming", "list", nil)
	require.NoError(t, err)

	// Concurrent iterator access
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := apiSet.NextIterator(context.Background(), "shared-iter")
			if err != nil {
				var apiErr *HostAPIError
				if !errors.As(err, &apiErr) || apiErr.Code != ErrorCodeIteratorNotFound {
					errs <- err
				}
			}
		}()
	}

	// Wait for all operations
	wg.Wait()
	close(errs)

	// Check for errors
	for err := range errs {
		t.Errorf("concurrent operation failed: %v", err)
	}
}

// Test: Close behavior and idempotency
func TestHostAPISet_Close(t *testing.T) {
	// Track close calls
	api1Closed := false
	api2Closed := false
	iterClosed := false

	// Create closable APIs
	api1 := &mockClosableAPI{
		mockHostAPI: mockHostAPI{name: "api1"},
	}
	api1.closed = false

	api2 := &mockClosableAPI{
		mockHostAPI: mockHostAPI{name: "api2"},
	}

	// Create tracking wrappers
	api1Track := &trackingClosableAPI{
		mockClosableAPI: api1,
		closedFlag:      &api1Closed,
	}
	api2Track := &trackingClosableAPI{
		mockClosableAPI: api2,
		closedFlag:      &api2Closed,
	}

	// Create host API set with closable APIs and an iterator
	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"api1": api1Track,
			"api2": api2Track,
		},
		iterators: map[string]*iteratorInfo{
			"iter1": {
				iterator: &mockIterator{
					closeFunc: func() error {
						iterClosed = true
						return nil
					},
				},
			},
		},
		config: HostAPIConfig{},
		closed: false,
	}

	// First close should succeed
	err := apiSet.Close()
	require.NoError(t, err)
	assert.True(t, apiSet.closed)
	assert.True(t, iterClosed)

	// Verify all resources were cleaned
	assert.Nil(t, apiSet.iterators)

	// Second close should be idempotent
	err = apiSet.Close()
	require.NoError(t, err)
}

// trackingClosableAPI wraps mockClosableAPI to track close calls
type trackingClosableAPI struct {
	*mockClosableAPI
	closedFlag *bool
}

func (t *trackingClosableAPI) Close() error {
	*t.closedFlag = true
	return t.mockClosableAPI.Close()
}

// Test: Close with errors
func TestHostAPISet_CloseWithErrors(t *testing.T) {
	// Create API that fails to close
	failingAPI := &failingClosableAPI{
		err: errors.New("close failed"),
	}

	// Create iterator that fails to close
	failingIterator := &mockIterator{
		closeFunc: func() error {
			return errors.New("iterator close failed")
		},
	}

	apiSet := &defaultHostAPISet{
		apis: map[string]HostAPI{
			"failing": failingAPI,
		},
		iterators: map[string]*iteratorInfo{
			"iter1": {
				iterator: failingIterator,
			},
		},
		config: HostAPIConfig{},
		closed: false,
	}

	// Close should return combined errors
	err := apiSet.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
	assert.Contains(t, err.Error(), "iterator close failed")
}

type failingClosableAPI struct {
	mockHostAPI
	err error
}

func (f *failingClosableAPI) Close() error {
	return f.err
}

// Ensure interfaces are satisfied
var (
	_ io.Closer = (*mockClosableAPI)(nil)
	_ io.Closer = (*failingClosableAPI)(nil)
)
