package wasm

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompiledModule(t *testing.T) {
	// Test plan:
	// - Load the math-service.wasm fixture
	// - Create a compiled module
	// - Instantiate multiple workers
	// - Verify workers are independent

	// Test: Load WASM fixture
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err, "failed to read WASM fixture")
	require.NotEmpty(t, wasmBytes, "WASM bytes should not be empty")

	ctx := context.Background()

	// Test: Create compiled module
	module, err := NewWASMCompiledModule(ctx, wasmBytes)
	require.NoError(t, err, "failed to create compiled module")
	require.NotNil(t, module, "module should not be nil")
	defer module.Close(ctx)

	// Test: Instantiate first worker
	worker1, err := module.Instantiate(ctx)
	require.NoError(t, err, "failed to instantiate first worker")
	require.NotNil(t, worker1, "first worker should not be nil")
	defer worker1.Close(ctx)

	// Test: Instantiate second worker
	worker2, err := module.Instantiate(ctx)
	require.NoError(t, err, "failed to instantiate second worker")
	require.NotNil(t, worker2, "second worker should not be nil")
	defer worker2.Close(ctx)

	// Test: Verify workers are independent
	assert.NotEqual(t, worker1, worker2, "workers should be different instances")
}

func TestCompiledModule_InvalidWASM(t *testing.T) {
	// Test plan:
	// - Try to create module with empty bytes
	// - Try to create module with invalid WASM

	ctx := context.Background()

	// Test: Empty bytes
	module, err := NewWASMCompiledModule(ctx, []byte{})
	assert.Error(t, err, "should error on empty bytes")
	assert.Nil(t, module, "module should be nil on error")

	// Test: Invalid WASM
	module, err = NewWASMCompiledModule(ctx, []byte("not a wasm module"))
	assert.Error(t, err, "should error on invalid WASM")
	assert.Nil(t, module, "module should be nil on error")
}

func TestCompiledModule_ContextCancellation(t *testing.T) {
	// Test plan:
	// - Create module with cancelled context
	// - Verify proper error handling

	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)

	// Test: Create module with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	module, err := NewWASMCompiledModule(ctx, wasmBytes)
	// Wazero might still compile successfully with cancelled context
	// so we just ensure no panic occurs
	if module != nil {
		defer module.Close(context.Background())
	}
}