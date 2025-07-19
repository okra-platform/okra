package wasm

import (
	"context"
	"fmt"

	"github.com/okra-platform/okra/internal/hostapi"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMCompiledModuleWithHostAPIs extends WASMCompiledModule to support host APIs
type WASMCompiledModuleWithHostAPIs interface {
	WASMCompiledModule

	// WithHostAPIs configures which host APIs this module can access
	WithHostAPIs(apis []string) WASMCompiledModuleWithHostAPIs

	// WithHostAPIRegistry sets the registry to use for creating host API instances
	WithHostAPIRegistry(registry hostapi.HostAPIRegistry) WASMCompiledModuleWithHostAPIs

	// WithHostAPIConfig sets the configuration for host APIs
	WithHostAPIConfig(config hostapi.HostAPIConfig) WASMCompiledModuleWithHostAPIs
}

// NewWASMCompiledModuleWithHostAPIs creates a new compiled module with host API support
func NewWASMCompiledModuleWithHostAPIs(ctx context.Context, wasmBytes []byte) (WASMCompiledModuleWithHostAPIs, error) {
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

	return &wasmCompiledModuleWithHostAPIs{
		runtime:  runtime,
		compiled: compiled,
		hostAPIs: []string{},
	}, nil
}

type wasmCompiledModuleWithHostAPIs struct {
	runtime       wazero.Runtime
	compiled      wazero.CompiledModule
	hostAPIs      []string
	registry      hostapi.HostAPIRegistry
	hostAPIConfig hostapi.HostAPIConfig
}

func (m *wasmCompiledModuleWithHostAPIs) WithHostAPIs(apis []string) WASMCompiledModuleWithHostAPIs {
	m.hostAPIs = apis
	return m
}

func (m *wasmCompiledModuleWithHostAPIs) WithHostAPIRegistry(registry hostapi.HostAPIRegistry) WASMCompiledModuleWithHostAPIs {
	m.registry = registry
	return m
}

func (m *wasmCompiledModuleWithHostAPIs) WithHostAPIConfig(config hostapi.HostAPIConfig) WASMCompiledModuleWithHostAPIs {
	m.hostAPIConfig = config
	return m
}

func (m *wasmCompiledModuleWithHostAPIs) Instantiate(ctx context.Context) (WASMWorker, error) {
	// Create host API set if registry is configured
	var hostAPISet hostapi.HostAPISet
	if m.registry != nil && len(m.hostAPIs) > 0 {
		var err error
		hostAPISet, err = m.registry.CreateHostAPISet(ctx, m.hostAPIs, m.hostAPIConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create host API set: %w", err)
		}

		// Register host functions with the runtime
		err = hostapi.RegisterHostAPI(ctx, m.runtime, hostAPISet)
		if err != nil {
			hostAPISet.Close()
			return nil, fmt.Errorf("failed to register host APIs: %w", err)
		}
	}

	// Create module config - don't call _start since this is a reactor module
	config := wazero.NewModuleConfig().
		WithStdout(nil).
		WithStderr(nil).
		WithName("").
		WithStartFunctions() // Don't call _start

	// Instantiate the module
	module, err := m.runtime.InstantiateModule(ctx, m.compiled, config)
	if err != nil {
		if hostAPISet != nil {
			hostAPISet.Close()
		}
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}

	// Call _initialize if it exists
	if initialize := module.ExportedFunction("_initialize"); initialize != nil {
		if _, err := initialize.Call(ctx); err != nil {
			module.Close(ctx)
			if hostAPISet != nil {
				hostAPISet.Close()
			}
			return nil, fmt.Errorf("failed to call _initialize: %w", err)
		}
	}

	// Get required functions
	handleRequest := module.ExportedFunction("handle_request")
	if handleRequest == nil {
		module.Close(ctx)
		if hostAPISet != nil {
			hostAPISet.Close()
		}
		return nil, fmt.Errorf("handle_request function not found")
	}

	allocate := module.ExportedFunction("allocate")
	if allocate == nil {
		module.Close(ctx)
		if hostAPISet != nil {
			hostAPISet.Close()
		}
		return nil, fmt.Errorf("allocate function not found")
	}

	deallocate := module.ExportedFunction("deallocate")
	if deallocate == nil {
		module.Close(ctx)
		if hostAPISet != nil {
			hostAPISet.Close()
		}
		return nil, fmt.Errorf("deallocate function not found")
	}

	return &wasmWorkerWithHostAPIs{
		wasmWorker: wasmWorker{
			module:        module,
			handleRequest: handleRequest,
			allocate:      allocate,
			deallocate:    deallocate,
		},
		hostAPISet: hostAPISet,
	}, nil
}

func (m *wasmCompiledModuleWithHostAPIs) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

// wasmWorkerWithHostAPIs extends wasmWorker with host API cleanup
type wasmWorkerWithHostAPIs struct {
	wasmWorker
	hostAPISet hostapi.HostAPISet
}

func (w *wasmWorkerWithHostAPIs) Close(ctx context.Context) error {
	// Close the module first
	err := w.wasmWorker.Close(ctx)

	// Then close the host API set
	if w.hostAPISet != nil {
		if closeErr := w.hostAPISet.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				// Combine errors
				err = fmt.Errorf("module close: %w, host API close: %v", err, closeErr)
			}
		}
	}

	return err
}
