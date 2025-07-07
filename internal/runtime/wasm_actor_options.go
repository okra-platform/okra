package runtime

import "github.com/okra-platform/okra/internal/wasm"

// WASMActorOption is a functional option for configuring a WASMActor
type WASMActorOption func(*WASMActor)

// WithWorkerPool sets a pre-configured worker pool for testing
func WithWorkerPool(pool wasm.WASMWorkerPool) WASMActorOption {
	return func(a *WASMActor) {
		a.workerPool = pool
	}
}

// WithMinWorkers sets the minimum number of workers in the pool
func WithMinWorkers(minWorkers int) WASMActorOption {
	return func(a *WASMActor) {
		a.minWorkers = minWorkers
	}
}

// WithMaxWorkers sets the maximum number of workers in the pool
func WithMaxWorkers(maxWorkers int) WASMActorOption {
	return func(a *WASMActor) {
		a.maxWorkers = maxWorkers
	}
}