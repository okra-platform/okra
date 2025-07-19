package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// WASMWorker represents a single WASM worker instance.
type WASMWorker interface {
	Invoke(ctx context.Context, method string, input []byte) ([]byte, error)
	Close(ctx context.Context) error
}

type wasmWorker struct {
	module        api.Module
	handleRequest api.Function
	allocate      api.Function
	deallocate    api.Function
}

func (w *wasmWorker) Invoke(ctx context.Context, method string, input []byte) ([]byte, error) {
	// Allocate memory for method string
	methodBytes := []byte(method)
	methodPtr, err := w.allocate.Call(ctx, uint64(len(methodBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory for method: %w", err)
	}
	defer func() { _, _ = w.deallocate.Call(ctx, methodPtr[0]) }()

	// Write method to memory
	if !w.module.Memory().Write(uint32(methodPtr[0]), methodBytes) {
		return nil, fmt.Errorf("failed to write method to memory")
	}

	// Allocate memory for input
	inputPtr, err := w.allocate.Call(ctx, uint64(len(input)))
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory for input: %w", err)
	}
	defer func() { _, _ = w.deallocate.Call(ctx, inputPtr[0]) }()

	// Write input to memory
	if !w.module.Memory().Write(uint32(inputPtr[0]), input) {
		return nil, fmt.Errorf("failed to write input to memory")
	}

	// Call handle_request
	result, err := w.handleRequest.Call(ctx,
		methodPtr[0], uint64(len(methodBytes)),
		inputPtr[0], uint64(len(input)))
	if err != nil {
		return nil, fmt.Errorf("failed to call handle_request: %w", err)
	}

	// Parse result (ptr << 32 | len)
	resultValue := result[0]
	if resultValue == 0 {
		return nil, fmt.Errorf("handle_request returned null")
	}

	outputPtr := uint32(resultValue >> 32)
	outputLen := uint32(resultValue & 0xFFFFFFFF)

	// Read output from memory
	output, ok := w.module.Memory().Read(outputPtr, outputLen)
	if !ok {
		return nil, fmt.Errorf("failed to read output from memory")
	}

	// Make a copy since the memory will be deallocated
	outputCopy := make([]byte, len(output))
	copy(outputCopy, output)

	// Deallocate output memory
	_, _ = w.deallocate.Call(ctx, uint64(outputPtr))

	return outputCopy, nil
}

func (w *wasmWorker) Close(ctx context.Context) error {
	return w.module.Close(ctx)
}
