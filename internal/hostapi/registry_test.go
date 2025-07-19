package hostapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Test Plan:
// 1. Test registry creation and basic operations
// 2. Test duplicate registration prevention
// 3. Test concurrent access to registry
// 4. Test host API set creation with valid APIs
// 5. Test host API set creation with missing APIs
// 6. Test cleanup on failed host API set creation

// Test: Registry creation and basic operations
func TestHostAPIRegistry_BasicOperations(t *testing.T) {
	registry := NewHostAPIRegistry()
	require.NotNil(t, registry)

	// Initially empty
	assert.Empty(t, registry.List())

	// Register a factory
	factory1 := &mockHostAPIFactory{
		name:    "test.api1",
		version: "v1.0.0",
	}
	err := registry.Register(factory1)
	require.NoError(t, err)

	// Verify it's registered
	assert.Len(t, registry.List(), 1)

	// Get the factory
	retrieved, ok := registry.Get("test.api1")
	assert.True(t, ok)
	assert.Equal(t, factory1, retrieved)

	// Get non-existent factory
	_, ok = registry.Get("test.nonexistent")
	assert.False(t, ok)

	// Register another factory
	factory2 := &mockHostAPIFactory{
		name:    "test.api2",
		version: "v1.0.0",
	}
	err = registry.Register(factory2)
	require.NoError(t, err)

	// Verify both are registered
	assert.Len(t, registry.List(), 2)
}

// Test: Duplicate registration prevention
func TestHostAPIRegistry_DuplicateRegistration(t *testing.T) {
	registry := NewHostAPIRegistry()

	factory1 := &mockHostAPIFactory{
		name:    "test.api",
		version: "v1.0.0",
	}

	// First registration should succeed
	err := registry.Register(factory1)
	require.NoError(t, err)

	// Duplicate registration should fail
	factory2 := &mockHostAPIFactory{
		name:    "test.api", // Same name
		version: "v2.0.0",
	}
	err = registry.Register(factory2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Verify only the first one is registered
	retrieved, ok := registry.Get("test.api")
	assert.True(t, ok)
	assert.Equal(t, "v1.0.0", retrieved.Version())
}

// Test: Concurrent access to registry
func TestHostAPIRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewHostAPIRegistry()

	// Register some initial factories
	for i := 0; i < 10; i++ {
		factory := &mockHostAPIFactory{
			name:    fmt.Sprintf("test.api%d", i),
			version: "v1.0.0",
		}
		err := registry.Register(factory)
		require.NoError(t, err)
	}

	// Concurrent reads and writes
	done := make(chan bool)
	errors := make(chan error, 100)

	// Writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			factory := &mockHostAPIFactory{
				name:    fmt.Sprintf("test.concurrent%d", id),
				version: "v1.0.0",
			}
			if err := registry.Register(factory); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			// List all
			list := registry.List()
			if len(list) < 10 {
				errors <- fmt.Errorf("expected at least 10 factories, got %d", len(list))
			}

			// Get specific
			_, ok := registry.Get("test.api5")
			if !ok {
				errors <- fmt.Errorf("failed to get test.api5")
			}

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

// mockClosableAPI implements both HostAPI and io.Closer
type mockClosableAPI struct {
	mockHostAPI
	closed bool
}

func (m *mockClosableAPI) Close() error {
	m.closed = true
	return nil
}

// mockFailingFactory creates APIs that fail
type mockFailingFactory struct {
	mockHostAPIFactory
	failAt int
	count  int
}

func (f *mockFailingFactory) Create(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
	f.count++
	if f.count == f.failAt {
		return nil, errors.New("intentional failure")
	}
	return &mockClosableAPI{
		mockHostAPI: mockHostAPI{
			name:    f.name,
			version: f.version,
		},
	}, nil
}

// Test: Host API set creation with valid APIs
func TestHostAPIRegistry_CreateHostAPISet(t *testing.T) {
	registry := NewHostAPIRegistry()

	// Register factories
	factory1 := &mockHostAPIFactory{
		name:    "test.api1",
		version: "v1.0.0",
	}
	factory2 := &mockHostAPIFactory{
		name:    "test.api2",
		version: "v1.0.0",
	}

	require.NoError(t, registry.Register(factory1))
	require.NoError(t, registry.Register(factory2))

	// Create config
	config := HostAPIConfig{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Logger:         slog.Default(),
		Tracer:         tracenoop.NewTracerProvider().Tracer("test"),
		Meter:          metricnoop.NewMeterProvider().Meter("test"),
		PolicyEngine:   &mockPolicyEngine{},
	}

	// Create host API set
	apiSet, err := registry.CreateHostAPISet(
		context.Background(),
		[]string{"test.api1", "test.api2"},
		config,
	)
	require.NoError(t, err)
	require.NotNil(t, apiSet)

	// Verify APIs are available
	api1, ok := apiSet.Get("test.api1")
	assert.True(t, ok)
	assert.Equal(t, "test.api1", api1.Name())

	api2, ok := apiSet.Get("test.api2")
	assert.True(t, ok)
	assert.Equal(t, "test.api2", api2.Name())

	// Clean up
	require.NoError(t, apiSet.Close())
}

// Test: Host API set creation with missing APIs
func TestHostAPIRegistry_CreateHostAPISetMissingAPI(t *testing.T) {
	registry := NewHostAPIRegistry()

	// Register only one factory
	factory := &mockHostAPIFactory{
		name:    "test.api1",
		version: "v1.0.0",
	}
	require.NoError(t, registry.Register(factory))

	config := HostAPIConfig{
		ServiceName: "test-service",
		Logger:      slog.Default(),
		Tracer:      tracenoop.NewTracerProvider().Tracer("test"),
		Meter:       metricnoop.NewMeterProvider().Meter("test"),
	}

	// Try to create set with missing API
	_, err := registry.CreateHostAPISet(
		context.Background(),
		[]string{"test.api1", "test.missing"},
		config,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test.missing not found")
}

// Test: Cleanup on failed host API set creation
func TestHostAPIRegistry_CreateHostAPISetCleanupOnFailure(t *testing.T) {
	registry := NewHostAPIRegistry().(*defaultHostAPIRegistry)

	// Create factories where the second one fails
	factory1 := &mockHostAPIFactory{
		name:    "test.api1",
		version: "v1.0.0",
	}
	factory2 := &mockFailingFactory{
		mockHostAPIFactory: mockHostAPIFactory{
			name:    "test.api2",
			version: "v1.0.0",
		},
		failAt: 1, // Fail on first create
	}

	require.NoError(t, registry.Register(factory1))
	require.NoError(t, registry.Register(factory2))

	// Track created APIs
	var createdAPIs []*mockClosableAPI
	registry.factories["test.api1"] = &mockHostAPIFactory{
		name:    "test.api1",
		version: "v1.0.0",
	}
	// Override factory1 to track creation
	registry.factories["test.api1"] = HostAPIFactoryFunc(func(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
		api := &mockClosableAPI{
			mockHostAPI: mockHostAPI{
				name:    "test.api1",
				version: "v1.0.0",
			},
		}
		createdAPIs = append(createdAPIs, api)
		return api, nil
	})

	config := HostAPIConfig{
		ServiceName:  "test-service",
		Logger:       slog.Default(),
		Tracer:       tracenoop.NewTracerProvider().Tracer("test"),
		Meter:        metricnoop.NewMeterProvider().Meter("test"),
		PolicyEngine: &mockPolicyEngine{},
	}

	// Create should fail due to factory2
	_, err := registry.CreateHostAPISet(
		context.Background(),
		[]string{"test.api1", "test.api2"},
		config,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intentional failure")

	// Verify cleanup happened
	assert.Len(t, createdAPIs, 1)
	assert.True(t, createdAPIs[0].closed, "API should have been closed on cleanup")
}

// HostAPIFactoryFunc is a function adapter for HostAPIFactory
type HostAPIFactoryFunc func(ctx context.Context, config HostAPIConfig) (HostAPI, error)

func (f HostAPIFactoryFunc) Name() string              { return "test.api1" }
func (f HostAPIFactoryFunc) Version() string           { return "v1.0.0" }
func (f HostAPIFactoryFunc) Methods() []MethodMetadata { return nil }
func (f HostAPIFactoryFunc) Create(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
	return f(ctx, config)
}
