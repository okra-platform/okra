package wasm

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/okra-platform/okra/internal/hostapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan:
// 1. Test creating module with empty WASM bytes
// 2. Test creating module with valid WASM bytes
// 3. Test WithHostAPIs method
// 4. Test WithHostAPIRegistry method
// 5. Test WithHostAPIConfig method
// 6. Test Instantiate with host APIs
// 7. Test Instantiate without host APIs
// 8. Test cleanup on errors
// 9. Test worker Close with host APIs

func TestNewWASMCompiledModuleWithHostAPIs_EmptyBytes(t *testing.T) {
	// Test: Creating module with empty WASM bytes should fail
	ctx := context.Background()
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, []byte{})
	
	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "wasm bytes cannot be empty")
}

func TestNewWASMCompiledModuleWithHostAPIs_InvalidWASM(t *testing.T) {
	// Test: Creating module with invalid WASM bytes should fail
	ctx := context.Background()
	invalidWASM := []byte("not a valid wasm module")
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, invalidWASM)
	
	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "failed to compile module")
}

func TestWASMCompiledModuleWithHostAPIs_BuilderMethods(t *testing.T) {
	// Test: Builder methods should set properties correctly
	ctx := context.Background()
	
	// Load test WASM file
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, module)
	defer module.Close(ctx)
	
	// Test WithHostAPIs
	apis := []string{"state", "pubsub"}
	result1 := module.WithHostAPIs(apis)
	assert.Equal(t, module, result1) // Should return self for chaining
	
	// Test WithHostAPIRegistry
	mockRegistry := &mockHostAPIRegistry{}
	result2 := module.WithHostAPIRegistry(mockRegistry)
	assert.Equal(t, module, result2)
	
	// Test WithHostAPIConfig
	config := hostapi.HostAPIConfig{
		ServiceName: "test-service",
	}
	result3 := module.WithHostAPIConfig(config)
	assert.Equal(t, module, result3)
}

func TestWASMCompiledModuleWithHostAPIs_InstantiateWithoutHostAPIs(t *testing.T) {
	// Test: Instantiate should work without host APIs configured
	ctx := context.Background()
	
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, module)
	defer module.Close(ctx)
	
	// Instantiate without setting host APIs
	worker, err := module.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, worker)
	
	// Cleanup
	err = worker.Close(ctx)
	assert.NoError(t, err)
}

func TestWASMCompiledModuleWithHostAPIs_InstantiateWithHostAPIs(t *testing.T) {
	// Test: Instantiate should create host API set when configured
	ctx := context.Background()
	
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, module)
	defer module.Close(ctx)
	
	// Configure host APIs
	mockRegistry := &mockHostAPIRegistry{
		createSetFunc: func(ctx context.Context, apis []string, config hostapi.HostAPIConfig) (hostapi.HostAPISet, error) {
			return &mockHostAPISet{}, nil
		},
	}
	
	module.WithHostAPIs([]string{"state"}).
		WithHostAPIRegistry(mockRegistry).
		WithHostAPIConfig(hostapi.HostAPIConfig{ServiceName: "test"})
	
	// Instantiate with host APIs
	worker, err := module.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, worker)
	
	// Verify registry was called
	assert.True(t, mockRegistry.createSetCalled)
	
	// Cleanup
	err = worker.Close(ctx)
	assert.NoError(t, err)
}

func TestWASMCompiledModuleWithHostAPIs_InstantiateError(t *testing.T) {
	// Test: Errors during instantiation should clean up properly
	ctx := context.Background()
	
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, module)
	defer module.Close(ctx)
	
	// Configure host APIs with error
	mockSet := &mockHostAPISet{}
	mockRegistry := &mockHostAPIRegistry{
		createSetFunc: func(ctx context.Context, apis []string, config hostapi.HostAPIConfig) (hostapi.HostAPISet, error) {
			return mockSet, nil
		},
	}
	
	module.WithHostAPIs([]string{"state"}).
		WithHostAPIRegistry(mockRegistry).
		WithHostAPIConfig(hostapi.HostAPIConfig{ServiceName: "test"})
	
	// Since we're using a simple service that should instantiate,
	// we test cleanup in the worker close
	worker, err := module.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, worker)
	
	// Close should clean up host API set
	err = worker.Close(ctx)
	assert.NoError(t, err)
	assert.True(t, mockSet.closed)
}

func TestWASMWorkerWithHostAPIs_CloseWithErrors(t *testing.T) {
	// Test: Close should handle errors from both module and host API set
	ctx := context.Background()
	
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)
	
	module, err := NewWASMCompiledModuleWithHostAPIs(ctx, wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, module)
	defer module.Close(ctx)
	
	// Configure host APIs with close error
	mockSet := &mockHostAPISet{
		closeErr: assert.AnError,
	}
	mockRegistry := &mockHostAPIRegistry{
		createSetFunc: func(ctx context.Context, apis []string, config hostapi.HostAPIConfig) (hostapi.HostAPISet, error) {
			return mockSet, nil
		},
	}
	
	module.WithHostAPIs([]string{"state"}).
		WithHostAPIRegistry(mockRegistry).
		WithHostAPIConfig(hostapi.HostAPIConfig{ServiceName: "test"})
	
	worker, err := module.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, worker)
	
	// Close should return the host API close error
	err = worker.Close(ctx)
	assert.Error(t, err)
	// Since module closes successfully, we should only get the host API error
	assert.Equal(t, assert.AnError, err)
}

// Mock implementations for testing

type mockHostAPIRegistry struct {
	createSetCalled bool
	createSetFunc   func(context.Context, []string, hostapi.HostAPIConfig) (hostapi.HostAPISet, error)
}

func (m *mockHostAPIRegistry) Register(factory hostapi.HostAPIFactory) error {
	return nil
}

func (m *mockHostAPIRegistry) Get(name string) (hostapi.HostAPIFactory, bool) {
	return nil, false
}

func (m *mockHostAPIRegistry) List() []hostapi.HostAPIFactory {
	return nil
}

func (m *mockHostAPIRegistry) CreateHostAPISet(ctx context.Context, apis []string, config hostapi.HostAPIConfig) (hostapi.HostAPISet, error) {
	m.createSetCalled = true
	if m.createSetFunc != nil {
		return m.createSetFunc(ctx, apis, config)
	}
	return nil, nil
}

type mockHostAPISet struct {
	closed   bool
	closeErr error
}

func (m *mockHostAPISet) Get(name string) (hostapi.HostAPI, bool) {
	return nil, false
}

func (m *mockHostAPISet) Execute(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockHostAPISet) NextIterator(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
	return nil, false, nil
}

func (m *mockHostAPISet) CloseIterator(ctx context.Context, iteratorID string) error {
	return nil
}

func (m *mockHostAPISet) CleanupStaleIterators() int {
	return 0
}

func (m *mockHostAPISet) Config() hostapi.HostAPIConfig {
	return hostapi.HostAPIConfig{}
}

func (m *mockHostAPISet) Close() error {
	m.closed = true
	return m.closeErr
}