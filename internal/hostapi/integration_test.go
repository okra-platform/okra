package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"log/slog"
)

// Test plan:
// 1. Test full WASM → Host API → Policy flow
// 2. Test iterator lifecycle across WASM invocations
// 3. Test concurrent WASM calls with shared host APIs
// 4. Test memory allocation/deallocation patterns
// 5. Test oversized responses
// 6. Test policy enforcement at boundaries
// 7. Test error propagation through the stack
// 8. Test cleanup on WASM module termination
// 9. Test race conditions in host function calls
// 10. Test iterator cleanup and timeout

func TestHostAPI_FullIntegration(t *testing.T) {
	// Test: Complete flow through Host API to Policy Engine (without WASM)
	ctx := context.Background()
	
	// Create host API registry
	registry := NewHostAPIRegistry()
	
	// Register a test host API
	testFactory := &testHostAPIFactory{
		name:    "test.api",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testHostAPI{
				responses: map[string]json.RawMessage{
					"echo": json.RawMessage(`{"message":"hello from host"}`),
				},
			}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	// Create policy engine
	policy := &testPolicyEngine{
		allowAll: true,
	}
	
	// Create host API config
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: policy,
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	// Create host API set
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.api"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Execute host API call
	params := json.RawMessage(`{"input": "test"}`)
	response, err := hostAPISet.Execute(ctx, "test.api", "echo", params)
	require.NoError(t, err)
	
	// Parse response
	var resp map[string]interface{}
	err = json.Unmarshal(response, &resp)
	require.NoError(t, err)
	
	assert.Equal(t, "hello from host", resp["message"])
}

func TestHostAPI_IteratorLifecycle(t *testing.T) {
	// Test: Iterator lifecycle across multiple WASM invocations
	ctx := context.Background()
	
	// Create registry and register streaming API
	registry := NewHostAPIRegistry()
	
	iteratorData := []string{"item1", "item2", "item3"}
	testFactory := &testStreamingHostAPIFactory{
		testHostAPIFactory: testHostAPIFactory{
			name:    "test.streaming",
			version: "1.0.0",
			createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
				return &testStreamingHostAPI{
					data: iteratorData,
				}, nil
			},
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	// Create host API set
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: &testPolicyEngine{allowAll: true},
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.streaming"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// First invocation - create iterator
	resp1, err := hostAPISet.Execute(ctx, "test.streaming", "list", nil)
	require.NoError(t, err)
	
	var listResp struct {
		IteratorID string `json:"iteratorId"`
		HasData    bool   `json:"hasData"`
	}
	err = json.Unmarshal(resp1, &listResp)
	require.NoError(t, err)
	assert.NotEmpty(t, listResp.IteratorID)
	
	// Multiple invocations - consume iterator
	var collected []string
	for i := range iteratorData {
		data, hasMore, err := hostAPISet.NextIterator(ctx, listResp.IteratorID)
		require.NoError(t, err)
		
		var item struct {
			Value string `json:"value"`
		}
		err = json.Unmarshal(data, &item)
		require.NoError(t, err)
		
		collected = append(collected, item.Value)
		
		if i < len(iteratorData)-1 {
			assert.True(t, hasMore)
		} else {
			assert.False(t, hasMore)
		}
	}
	
	assert.Equal(t, iteratorData, collected)
	
	// Try to use exhausted iterator
	_, _, err = hostAPISet.NextIterator(ctx, listResp.IteratorID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "iterator not found")
}

func TestHostAPI_ConcurrentWASMCalls(t *testing.T) {
	// Test: Multiple concurrent WASM calls with shared host APIs
	ctx := context.Background()
	
	// Create registry
	registry := NewHostAPIRegistry()
	
	// Track concurrent calls
	var activeCallsMax int32
	var activeCalls int32
	
	testFactory := &testHostAPIFactory{
		name:    "test.concurrent",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testConcurrentHostAPI{
				onExecute: func() {
					current := atomic.AddInt32(&activeCalls, 1)
					defer atomic.AddInt32(&activeCalls, -1)
					
					// Track max concurrent calls
					for {
						max := atomic.LoadInt32(&activeCallsMax)
						if current <= max || atomic.CompareAndSwapInt32(&activeCallsMax, max, current) {
							break
						}
					}
					
					// Simulate work
					time.Sleep(10 * time.Millisecond)
				},
			}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	// Create host API set
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: &testPolicyEngine{allowAll: true},
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.concurrent"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Launch concurrent calls
	var wg sync.WaitGroup
	concurrency := 20
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			
			params := json.RawMessage(fmt.Sprintf(`{"id":%d}`, n))
			_, err := hostAPISet.Execute(ctx, "test.concurrent", "process", params)
			assert.NoError(t, err)
		}(i)
	}
	
	wg.Wait()
	
	// Verify we had concurrent execution
	maxConcurrent := atomic.LoadInt32(&activeCallsMax)
	t.Logf("Max concurrent calls: %d", maxConcurrent)
	assert.Greater(t, maxConcurrent, int32(1))
}

func TestHostAPI_OversizedResponse(t *testing.T) {
	// Test: Handling of oversized responses
	ctx := context.Background()
	
	registry := NewHostAPIRegistry()
	
	// Create API that returns large response
	largeData := make([]byte, 5*1024*1024) // 5MB
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}
	
	testFactory := &testHostAPIFactory{
		name:    "test.large",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testHostAPI{
				responses: map[string]json.RawMessage{
					"getLarge": json.RawMessage(fmt.Sprintf(`{"data":"%s"}`, string(largeData))),
				},
			}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: &testPolicyEngine{allowAll: true},
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.large"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Should handle large response appropriately
	resp, err := hostAPISet.Execute(ctx, "test.large", "getLarge", nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	
	// Verify response is valid JSON despite size
	var parsed map[string]interface{}
	err = json.Unmarshal(resp, &parsed)
	require.NoError(t, err)
}

func TestHostAPI_PolicyEnforcement(t *testing.T) {
	// Test: Policy enforcement at API boundaries
	ctx := context.Background()
	
	registry := NewHostAPIRegistry()
	
	// Register test API
	testFactory := &testHostAPIFactory{
		name:    "test.restricted",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testHostAPI{
				responses: map[string]json.RawMessage{
					"allowed":    json.RawMessage(`{"result":"allowed"}`),
					"restricted": json.RawMessage(`{"result":"restricted"}`),
				},
			}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	// Create policy that restricts certain methods
	policy := &testPolicyEngine{
		allowFunc: func(api, method string) bool {
			return method != "restricted"
		},
	}
	
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: policy,
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.restricted"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Allowed method should work
	resp, err := hostAPISet.Execute(ctx, "test.restricted", "allowed", nil)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "allowed")
	
	// Restricted method should fail
	_, err = hostAPISet.Execute(ctx, "test.restricted", "restricted", nil)
	assert.Error(t, err)
	var apiErr *HostAPIError
	assert.ErrorAs(t, err, &apiErr)
	assert.Equal(t, ErrorCodePolicyDenied, apiErr.Code)
}

func TestHostAPI_ErrorPropagation(t *testing.T) {
	// Test: Error propagation through WASM → Host API stack
	ctx := context.Background()
	
	registry := NewHostAPIRegistry()
	
	// Register API that returns errors
	testFactory := &testHostAPIFactory{
		name:    "test.errors",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testErrorHostAPI{}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: &testPolicyEngine{allowAll: true},
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.errors"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Test different error types
	tests := []struct {
		method   string
		wantCode string
	}{
		{"notFound", ErrorCodeAPINotFound},
		{"invalid", ErrorCodeInternalError},
		{"internal", ErrorCodeInternalError},
	}
	
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			_, err := hostAPISet.Execute(ctx, "test.errors", tt.method, nil)
			require.Error(t, err)
			
			var apiErr *HostAPIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tt.wantCode, apiErr.Code)
		})
	}
}

func TestHostAPI_CleanupOnTermination(t *testing.T) {
	// Test: Proper cleanup when host API set is closed
	ctx := context.Background()
	
	registry := NewHostAPIRegistry()
	cleanupCalled := false
	
	// Register API with cleanup tracking
	testFactory := &testHostAPIFactory{
		name:    "test.cleanup",
		version: "1.0.0",
		createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
			return &testCleanupHostAPI{
				onClose: func() {
					cleanupCalled = true
				},
			}, nil
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	config := HostAPIConfig{
		ServiceName:  "test-service",
		PolicyEngine: &testPolicyEngine{allowAll: true},
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		Logger:       slog.Default(),
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.cleanup"}, config)
	require.NoError(t, err)
	
	// Use the host API
	_, err = hostAPISet.Execute(ctx, "test.cleanup", "test", nil)
	require.NoError(t, err)
	
	// Close host API set
	err = hostAPISet.Close()
	require.NoError(t, err)
	
	// Verify cleanup was called
	assert.True(t, cleanupCalled)
}

func TestHostAPI_IteratorCleanupTimeout(t *testing.T) {
	// Test: Iterator cleanup after timeout
	ctx := context.Background()
	
	registry := NewHostAPIRegistry()
	
	// Register streaming API
	testFactory := &testStreamingHostAPIFactory{
		testHostAPIFactory: testHostAPIFactory{
			name:    "test.timeout",
			version: "1.0.0",
			createFunc: func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
				return &testStreamingHostAPI{
					data: []string{"item1", "item2"},
				}, nil
			},
		},
	}
	err := registry.Register(testFactory)
	require.NoError(t, err)
	
	// Create config with short timeout
	config := HostAPIConfig{
		ServiceName:     "test-service",
		PolicyEngine:    &testPolicyEngine{allowAll: true},
		Tracer:          tracenoop.NewTracerProvider().Tracer("test"),
		Meter:           metricnoop.NewMeterProvider().Meter("test"),
		Logger:          slog.Default(),
		IteratorTimeout: 100 * time.Millisecond, // Short timeout for testing
	}
	
	hostAPISet, err := registry.CreateHostAPISet(ctx, []string{"test.timeout"}, config)
	require.NoError(t, err)
	defer hostAPISet.Close()
	
	// Create iterator
	resp, err := hostAPISet.Execute(ctx, "test.timeout", "list", nil)
	require.NoError(t, err)
	
	var listResp struct {
		IteratorID string `json:"iteratorId"`
		HasData    bool   `json:"hasData"`
	}
	err = json.Unmarshal(resp, &listResp)
	require.NoError(t, err)
	
	// Use iterator once
	_, hasMore, err := hostAPISet.NextIterator(ctx, listResp.IteratorID)
	require.NoError(t, err)
	assert.True(t, hasMore)
	
	// Wait for timeout
	time.Sleep(200 * time.Millisecond)
	
	// Trigger cleanup
	cleaned := hostAPISet.CleanupStaleIterators()
	assert.Equal(t, 1, cleaned)
	
	// Iterator should be gone
	_, _, err = hostAPISet.NextIterator(ctx, listResp.IteratorID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "iterator not found")
}

// Helper functions and test implementations

// Test host API implementations

type testHostAPI struct {
	responses map[string]json.RawMessage
}

func (t *testHostAPI) Name() string    { return "test.api" }
func (t *testHostAPI) Version() string { return "1.0.0" }

func (t *testHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if resp, ok := t.responses[method]; ok {
		return resp, nil
	}
	return nil, &HostAPIError{
		Code:    ErrorCodeAPINotFound,
		Message: "method not found",
	}
}

type testStreamingHostAPI struct {
	data []string
}

func (t *testStreamingHostAPI) Name() string    { return "test.streaming" }
func (t *testStreamingHostAPI) Version() string { return "1.0.0" }

func (t *testStreamingHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	return nil, &HostAPIError{Code: ErrorCodeAPINotFound}
}

func (t *testStreamingHostAPI) ExecuteStreaming(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, Iterator, error) {
	if method == "list" {
		iter := &testIterator{
			data:  t.data,
			index: 0,
		}
		resp := json.RawMessage(fmt.Sprintf(`{"iteratorId":"%s","hasData":true}`, iter.ID()))
		return resp, iter, nil
	}
	return nil, nil, &HostAPIError{Code: ErrorCodeAPINotFound}
}

type testIterator struct {
	data  []string
	index int
}

func (t *testIterator) ID() string { return "test-iterator" }

func (t *testIterator) Next(ctx context.Context) (json.RawMessage, bool, error) {
	if t.index >= len(t.data) {
		return nil, false, nil
	}
	
	item := t.data[t.index]
	t.index++
	
	data := json.RawMessage(fmt.Sprintf(`{"value":"%s"}`, item))
	hasMore := t.index < len(t.data)
	
	return data, hasMore, nil
}

func (t *testIterator) Close() error { return nil }

type testConcurrentHostAPI struct {
	onExecute func()
}

func (t *testConcurrentHostAPI) Name() string    { return "test.concurrent" }
func (t *testConcurrentHostAPI) Version() string { return "1.0.0" }

func (t *testConcurrentHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if t.onExecute != nil {
		t.onExecute()
	}
	return json.RawMessage(`{"status":"ok"}`), nil
}

type testErrorHostAPI struct{}

func (t *testErrorHostAPI) Name() string    { return "test.errors" }
func (t *testErrorHostAPI) Version() string { return "1.0.0" }

func (t *testErrorHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "notFound":
		return nil, &HostAPIError{Code: ErrorCodeAPINotFound, Message: "not found"}
	case "invalid":
		return nil, &HostAPIError{Code: ErrorCodeInternalError, Message: "invalid"}
	case "internal":
		return nil, &HostAPIError{Code: ErrorCodeInternalError, Message: "internal error"}
	default:
		return nil, &HostAPIError{Code: ErrorCodeAPINotFound}
	}
}

type testCleanupHostAPI struct {
	onClose func()
}

func (t *testCleanupHostAPI) Name() string    { return "test.cleanup" }
func (t *testCleanupHostAPI) Version() string { return "1.0.0" }

func (t *testCleanupHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (t *testCleanupHostAPI) Close() error {
	if t.onClose != nil {
		t.onClose()
	}
	return nil
}

type testHostAPIFactory struct {
	name       string
	version    string
	createFunc func(context.Context, HostAPIConfig) (HostAPI, error)
}

func (f *testHostAPIFactory) Name() string    { return f.name }
func (f *testHostAPIFactory) Version() string { return f.version }

func (f *testHostAPIFactory) Create(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
	if f.createFunc != nil {
		return f.createFunc(ctx, config)
	}
	return nil, nil
}

func (f *testHostAPIFactory) Methods() []MethodMetadata {
	return []MethodMetadata{}
}

type testStreamingHostAPIFactory struct {
	testHostAPIFactory
}

type testPolicyEngine struct {
	allowAll  bool
	allowFunc func(api, method string) bool
}

func (p *testPolicyEngine) Evaluate(ctx context.Context, check PolicyCheck) (PolicyDecision, error) {
	allowed := p.allowAll
	if !allowed && p.allowFunc != nil {
		allowed = p.allowFunc(check.Request.API, check.Request.Method)
	}
	
	if allowed {
		return PolicyDecision{
			Allowed: true,
			Reason:  "test policy allowed",
		}, nil
	}
	
	return PolicyDecision{
		Allowed: false,
		Reason:  "test policy denied",
	}, nil
}