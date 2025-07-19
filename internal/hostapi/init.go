package hostapi

import "fmt"

// InitializeHostAPIs registers all available host API factories
func InitializeHostAPIs(registry HostAPIRegistry) error {
	// TODO: Register core host APIs when they are implemented
	// This is a placeholder for now - actual implementations will come
	// in the core-host-apis feature
	factories := []HostAPIFactory{
		// NewStateAPIFactory(),
		// NewLogAPIFactory(),
		// NewEnvAPIFactory(),
		// NewSecretsAPIFactory(),
		// NewFetchAPIFactory(),
	}

	for _, factory := range factories {
		if err := registry.Register(factory); err != nil {
			return fmt.Errorf("failed to register %s: %w", factory.Name(), err)
		}
	}

	return nil
}
