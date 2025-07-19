package hostapi

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// HostAPIRegistry manages all available host API factories
type HostAPIRegistry interface {
	// Register adds a new host API factory
	Register(factory HostAPIFactory) error

	// Get retrieves a host API factory by name
	Get(name string) (HostAPIFactory, bool)

	// List returns all registered API factories
	List() []HostAPIFactory

	// CreateHostAPISet creates a set of host API instances for a specific service
	CreateHostAPISet(ctx context.Context, apis []string, config HostAPIConfig) (HostAPISet, error)
}

// defaultHostAPIRegistry is the concrete implementation
type defaultHostAPIRegistry struct {
	factories map[string]HostAPIFactory
	mu        sync.RWMutex
}

// NewHostAPIRegistry creates a new host API registry
func NewHostAPIRegistry() HostAPIRegistry {
	return &defaultHostAPIRegistry{
		factories: make(map[string]HostAPIFactory),
	}
}

// Register adds a new host API factory
func (r *defaultHostAPIRegistry) Register(factory HostAPIFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := factory.Name()
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("host API factory %s already registered", name)
	}

	r.factories[name] = factory
	return nil
}

// Get retrieves a host API factory by name
func (r *defaultHostAPIRegistry) Get(name string) (HostAPIFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[name]
	return factory, ok
}

// List returns all registered API factories
func (r *defaultHostAPIRegistry) List() []HostAPIFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factories := make([]HostAPIFactory, 0, len(r.factories))
	for _, factory := range r.factories {
		factories = append(factories, factory)
	}
	return factories
}

// CreateHostAPISet creates a set of host API instances for a specific service
func (r *defaultHostAPIRegistry) CreateHostAPISet(ctx context.Context, apis []string, config HostAPIConfig) (HostAPISet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostAPIs := make(map[string]HostAPI)

	// Create instances for each requested API
	for _, apiName := range apis {
		factory, ok := r.factories[apiName]
		if !ok {
			return nil, fmt.Errorf("host API %s not found", apiName)
		}

		api, err := factory.Create(ctx, config)
		if err != nil {
			// Cleanup already created APIs
			for _, created := range hostAPIs {
				if closer, ok := created.(io.Closer); ok {
					closer.Close()
				}
			}
			return nil, fmt.Errorf("failed to create %s: %w", apiName, err)
		}

		hostAPIs[apiName] = api
	}

	return &defaultHostAPISet{
		apis:      hostAPIs,
		iterators: make(map[string]*iteratorInfo),
		config:    config,
		closed:    false,
	}, nil
}
