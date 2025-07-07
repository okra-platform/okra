package wasm

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWASMWorkerPoolWithRealModule tests the pool with a real WASM module
func TestWASMWorkerPoolWithRealModule(t *testing.T) {
	// Test plan:
	// - Load real WASM module
	// - Create pool with min/max workers
	// - Execute multiple concurrent requests
	// - Verify results are correct

	// Test: Load WASM fixture
	wasmBytes, err := os.ReadFile("fixture/math-service/math-service.wasm")
	require.NoError(t, err)

	ctx := context.Background()

	// Test: Create compiled module
	module, err := NewWASMCompiledModule(ctx, wasmBytes)
	require.NoError(t, err)
	defer module.Close(ctx)

	// Test: Create pool with min=2, max=5
	pool, err := NewWASMWorkerPool(ctx, WASMWorkerPoolConfig{
		MinWorkers: 2,
		MaxWorkers: 5,
		Module:     module,
	})
	require.NoError(t, err)
	defer pool.Shutdown(ctx)

	// Test: Simple invocation
	input := AddInput{A: 10, B: 20}
	inputBytes, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := pool.Invoke(ctx, "add", inputBytes)
	require.NoError(t, err)

	var response AddResponse
	err = json.Unmarshal(output, &response)
	require.NoError(t, err)
	assert.Equal(t, 30, response.Sum)

	// Test: Concurrent invocations
	type testCase struct {
		a, b int
	}
	testCases := []testCase{
		{1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5},
		{10, 10}, {20, 20}, {30, 30}, {40, 40}, {50, 50},
	}

	results := make(chan int, len(testCases))
	errors := make(chan error, len(testCases))

	for _, tc := range testCases {
		go func(a, b int) {
			input := AddInput{A: a, B: b}
			inputBytes, err := json.Marshal(input)
			if err != nil {
				errors <- err
				return
			}

			output, err := pool.Invoke(ctx, "add", inputBytes)
			if err != nil {
				errors <- err
				return
			}

			var response AddResponse
			err = json.Unmarshal(output, &response)
			if err != nil {
				errors <- err
				return
			}

			results <- response.Sum
		}(tc.a, tc.b)
	}

	// Collect results
	for range len(testCases) {
		select {
		case err := <-errors:
			t.Fatalf("concurrent invocation failed: %v", err)
		case sum := <-results:
			// Verify the sum is one of our expected values
			found := false
			for _, tc := range testCases {
				if sum == tc.a+tc.b {
					found = true
					break
				}
			}
			assert.True(t, found, "unexpected sum: %d", sum)
		}
	}
}