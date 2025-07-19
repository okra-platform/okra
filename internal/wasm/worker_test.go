package wasm

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test types matching the fixture schema
type AddInput struct {
	A int `json:"a"`
	B int `json:"b"`
}

type AddResponse struct {
	Sum int `json:"sum"`
}

func TestWorker_Invoke(t *testing.T) {
	// Test plan:
	// - Create a worker from math-service fixture
	// - Test successful invocation of add method
	// - Test error cases (unknown method, invalid input)
	// - Test multiple invocations

	ctx := context.Background()
	worker := createTestWorker(t, ctx)
	defer worker.Close(ctx)

	// Test: Successful add invocation
	input := AddInput{A: 5, B: 3}
	inputBytes, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := worker.Invoke(ctx, "add", inputBytes)
	require.NoError(t, err, "invoke should succeed")
	require.NotEmpty(t, output, "output should not be empty")

	var response AddResponse
	err = json.Unmarshal(output, &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.Equal(t, 8, response.Sum, "sum should be correct")

	// Test: Unknown method
	output, err = worker.Invoke(ctx, "unknown", inputBytes)
	assert.Error(t, err, "should error on unknown method")
	assert.Nil(t, output, "output should be nil on error")

	// Test: Invalid input JSON
	output, err = worker.Invoke(ctx, "add", []byte("invalid json"))
	assert.Error(t, err, "should error on invalid JSON")
	assert.Nil(t, output, "output should be nil on error")

	// Test: Multiple invocations
	testCases := []struct {
		a, b, expectedSum int
	}{
		{1, 2, 3},
		{10, 20, 30},
		{-5, 5, 0},
		{100, 200, 300},
	}

	for _, tc := range testCases {
		input := AddInput{A: tc.a, B: tc.b}
		inputBytes, err := json.Marshal(input)
		require.NoError(t, err)

		output, err := worker.Invoke(ctx, "add", inputBytes)
		require.NoError(t, err)

		var response AddResponse
		err = json.Unmarshal(output, &response)
		require.NoError(t, err)
		assert.Equal(t, tc.expectedSum, response.Sum, "sum should be %d for %d + %d", tc.expectedSum, tc.a, tc.b)
	}
}

func TestWorker_LargeInput(t *testing.T) {
	// Test plan:
	// - Test with large numbers
	// - Verify memory handling

	ctx := context.Background()
	worker := createTestWorker(t, ctx)
	defer worker.Close(ctx)

	// Test: Large numbers
	input := AddInput{A: 1000000, B: 2000000}
	inputBytes, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := worker.Invoke(ctx, "add", inputBytes)
	require.NoError(t, err)

	var response AddResponse
	err = json.Unmarshal(output, &response)
	require.NoError(t, err)
	assert.Equal(t, 3000000, response.Sum)
}

func TestWorker_ContextCancellation(t *testing.T) {
	// Test plan:
	// - Create worker
	// - Cancel context during invocation
	// - Verify proper cleanup

	ctx := context.Background()
	worker := createTestWorker(t, ctx)
	defer worker.Close(context.Background())

	// Test: Invoke with cancelled context
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	input := AddInput{A: 1, B: 2}
	inputBytes, err := json.Marshal(input)
	require.NoError(t, err)

	// The behavior here depends on wazero's context handling
	// We mainly want to ensure no panic occurs
	output, _ := worker.Invoke(cancelCtx, "add", inputBytes)
	_ = output
}

// Helper function to create a test worker
func createTestWorker(t *testing.T, ctx context.Context) WASMWorker {
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)

	module, err := NewWASMCompiledModule(ctx, wasmBytes)
	require.NoError(t, err)
	t.Cleanup(func() {
		module.Close(context.Background())
	})

	worker, err := module.Instantiate(ctx)
	require.NoError(t, err)

	return worker
}
