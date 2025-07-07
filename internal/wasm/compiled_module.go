package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMCompiledModule represents a compiled WASM module that can create worker instances.
type WASMCompiledModule interface {
	// Instantiate creates a new isolated WASMWorker.
	// Each worker is backed by a fresh module instance.
	Instantiate(ctx context.Context) (WASMWorker, error)

	// Close cleans up any resources tied to the compiled module.
	Close(ctx context.Context) error
}

// NewWASMCompiledModule creates a new compiled module from WASM bytes.
func NewWASMCompiledModule(ctx context.Context, wasmBytes []byte) (WASMCompiledModule, error) {
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm bytes cannot be empty")
	}

	// Create a new runtime
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI
	_, err := wasi_snapshot_preview1.Instantiate(ctx, runtime)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Compile the module
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile module: %w", err)
	}

	return &wasmCompiledModule{
		runtime:  runtime,
		compiled: compiled,
	}, nil
}

type wasmCompiledModule struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

func (m *wasmCompiledModule) Instantiate(ctx context.Context) (WASMWorker, error) {
	// Create module config - don't call _start since this is a reactor module
	config := wazero.NewModuleConfig().
		WithStdout(nil).
		WithStderr(nil).
		WithName("").
		WithStartFunctions() // Don't call _start

	// Instantiate the module
	module, err := m.runtime.InstantiateModule(ctx, m.compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}

	// Call _initialize if it exists
	if initialize := module.ExportedFunction("_initialize"); initialize != nil {
		if _, err := initialize.Call(ctx); err != nil {
			module.Close(ctx)
			return nil, fmt.Errorf("failed to call _initialize: %w", err)
		}
	}

	// Get required functions
	handleRequest := module.ExportedFunction("handle_request")
	if handleRequest == nil {
		module.Close(ctx)
		return nil, fmt.Errorf("handle_request function not found")
	}

	allocate := module.ExportedFunction("allocate")
	if allocate == nil {
		module.Close(ctx)
		return nil, fmt.Errorf("allocate function not found")
	}

	deallocate := module.ExportedFunction("deallocate")
	if deallocate == nil {
		module.Close(ctx)
		return nil, fmt.Errorf("deallocate function not found")
	}

	return &wasmWorker{
		module:        module,
		handleRequest: handleRequest,
		allocate:      allocate,
		deallocate:    deallocate,
	}, nil
}

func (m *wasmCompiledModule) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}