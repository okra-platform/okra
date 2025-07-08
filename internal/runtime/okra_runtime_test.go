package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOkraRuntime(t *testing.T) {
	// Test: Create new runtime
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	runtime := NewOkraRuntime(logger)
	
	assert.NotNil(t, runtime)
	assert.NotNil(t, runtime.deployedActors)
	assert.False(t, runtime.started)
}

func TestOkraRuntime_Start(t *testing.T) {
	// Test: Start runtime
	t.Run("successful start", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		
		require.NoError(t, err)
		assert.True(t, runtime.started)
		assert.NotNil(t, runtime.actorSystem)
		
		// Cleanup
		runtime.Shutdown(ctx)
	})
	
	// Test: Start already started runtime
	t.Run("already started", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		// Try to start again
		err = runtime.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
		
		// Cleanup
		runtime.Shutdown(ctx)
	})
}

func TestOkraRuntime_Deploy(t *testing.T) {
	// Helper to create a test service package
	createTestPackage := func() *ServicePackage {
		// Load the math-service WASM
		wasmPath := filepath.Join("..", "..", "internal", "wasm", "fixture", "math-service", "math-service.wasm")
		wasmBytes, err := os.ReadFile(wasmPath)
		require.NoError(t, err)
		
		// Create compiled module
		ctx := context.Background()
		module, err := wasm.NewWASMCompiledModule(ctx, wasmBytes)
		require.NoError(t, err)
		
		// Create test schema
		testSchema := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "MathService",
					Methods: []schema.Method{
						{Name: "add", InputType: "AddInput", OutputType: "AddOutput"},
						{Name: "multiply", InputType: "MultiplyInput", OutputType: "MultiplyOutput"},
					},
				},
			},
			Meta: schema.Metadata{
				Namespace: "test",
				Version:   "v1",  // API version for backward compatibility
			},
		}
		
		// Create test config
		testConfig := &config.Config{
			Name:     "math-service",
			Version:  "1.0.0",  // Implementation version
			Language: "go",
		}
		
		pkg, err := NewServicePackage(module, testSchema, testConfig)
		require.NoError(t, err)
		
		return pkg
	}
	
	// Test: Deploy service successfully
	t.Run("successful deploy", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		pkg := createTestPackage()
		actorID, err := runtime.Deploy(ctx, pkg)
		
		require.NoError(t, err)
		assert.Equal(t, "test.MathService.v1", actorID)
		assert.True(t, runtime.IsDeployed(actorID))
		
		// Cleanup
		runtime.Shutdown(ctx)
		pkg.Module.Close(ctx)
	})
	
	// Test: Deploy without starting runtime
	t.Run("runtime not started", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		pkg := createTestPackage()
		
		_, err := runtime.Deploy(ctx, pkg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "runtime not started")
		
		// Cleanup
		pkg.Module.Close(ctx)
	})
	
	// Test: Deploy nil package
	t.Run("nil package", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		_, err = runtime.Deploy(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "package cannot be nil")
		
		// Cleanup
		runtime.Shutdown(ctx)
	})
	
	// Test: Deploy same service twice
	t.Run("already deployed", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		pkg := createTestPackage()
		actorID, err := runtime.Deploy(ctx, pkg)
		require.NoError(t, err)
		
		// Try to deploy again
		_, err = runtime.Deploy(ctx, pkg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already deployed")
		assert.Equal(t, actorID, "test.MathService.v1")
		
		// Cleanup
		runtime.Shutdown(ctx)
		pkg.Module.Close(ctx)
	})
}

func TestOkraRuntime_Undeploy(t *testing.T) {
	// Test: Undeploy deployed service
	t.Run("successful undeploy", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		// Deploy a service first
		wasmPath := filepath.Join("..", "..", "internal", "wasm", "fixture", "math-service", "math-service.wasm")
		wasmBytes, err := os.ReadFile(wasmPath)
		require.NoError(t, err)
		
		module, err := wasm.NewWASMCompiledModule(ctx, wasmBytes)
		require.NoError(t, err)
		
		testSchema := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "MathService",
					Methods: []schema.Method{
						{Name: "add"},
					},
				},
			},
			Meta: schema.Metadata{
				Version: "v1",  // API version
			},
		}
		
		testConfig := &config.Config{
			Name:    "math-service",
			Version: "1.0.0",  // Implementation version
		}
		
		pkg, err := NewServicePackage(module, testSchema, testConfig)
		require.NoError(t, err)
		
		actorID, err := runtime.Deploy(ctx, pkg)
		require.NoError(t, err)
		assert.True(t, runtime.IsDeployed(actorID))
		
		// Undeploy
		err = runtime.Undeploy(ctx, actorID)
		require.NoError(t, err)
		assert.False(t, runtime.IsDeployed(actorID))
		
		// Cleanup
		runtime.Shutdown(ctx)
		module.Close(ctx)
	})
	
	// Test: Undeploy non-existent service
	t.Run("service not deployed", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		err = runtime.Undeploy(ctx, "non.existent.v1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not deployed")
		
		// Cleanup
		runtime.Shutdown(ctx)
	})
	
	// Test: Undeploy without starting runtime
	t.Run("runtime not started", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Undeploy(ctx, "some.service.v1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "runtime not started")
	})
}

func TestOkraRuntime_Shutdown(t *testing.T) {
	// Test: Shutdown with deployed services
	t.Run("shutdown with services", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Start(ctx)
		require.NoError(t, err)
		
		// Deploy a service
		wasmPath := filepath.Join("..", "..", "internal", "wasm", "fixture", "math-service", "math-service.wasm")
		wasmBytes, err := os.ReadFile(wasmPath)
		require.NoError(t, err)
		
		module, err := wasm.NewWASMCompiledModule(ctx, wasmBytes)
		require.NoError(t, err)
		
		testSchema := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "MathService",
					Methods: []schema.Method{
						{Name: "add"},
					},
				},
			},
			Meta: schema.Metadata{
				Version: "v1",  // API version
			},
		}
		
		testConfig := &config.Config{
			Name:    "math-service",
			Version: "1.0.0",  // Implementation version
		}
		
		pkg, err := NewServicePackage(module, testSchema, testConfig)
		require.NoError(t, err)
		
		actorID, err := runtime.Deploy(ctx, pkg)
		require.NoError(t, err)
		assert.NotEmpty(t, actorID)
		
		// Shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		err = runtime.Shutdown(shutdownCtx)
		require.NoError(t, err)
		assert.False(t, runtime.started)
		assert.Empty(t, runtime.deployedActors)
		
		// Cleanup
		module.Close(ctx)
	})
	
	// Test: Shutdown without starting
	t.Run("shutdown not started", func(t *testing.T) {
		logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
		runtime := NewOkraRuntime(logger)
		
		ctx := context.Background()
		err := runtime.Shutdown(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "runtime not started")
	})
}

func TestOkraRuntime_generateActorID(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	runtime := NewOkraRuntime(logger)
	
	tests := []struct {
		name     string
		pkg      *ServicePackage
		expected string
	}{
		{
			name: "with namespace and version",
			pkg: &ServicePackage{
				ServiceName: "MyService",
				Schema: &schema.Schema{
					Meta: schema.Metadata{
						Namespace: "prod",
						Version:   "v2",
					},
				},
				Config: &config.Config{
					Version: "2.1.0",  // Implementation version
				},
			},
			expected: "prod.MyService.v2",
		},
		{
			name: "default namespace",
			pkg: &ServicePackage{
				ServiceName: "MyService",
				Schema: &schema.Schema{
					Meta: schema.Metadata{
						Version: "v3",
					},
				},
				Config: &config.Config{
					Version: "3.0.0",
				},
			},
			expected: "default.MyService.v3",
		},
		{
			name: "default version",
			pkg: &ServicePackage{
				ServiceName: "MyService",
				Schema: &schema.Schema{
					Meta: schema.Metadata{
						Namespace: "test",
					},
				},
				Config: &config.Config{},
			},
			expected: "test.MyService.v1",
		},
		{
			name: "all defaults",
			pkg: &ServicePackage{
				ServiceName: "MyService",
				Config:      &config.Config{},
			},
			expected: "default.MyService.v1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actorID := runtime.generateActorID(tt.pkg)
			assert.Equal(t, tt.expected, actorID)
		})
	}
}

// Test: GetActorPID returns correct PID for deployed service
// Test: GetActorPID returns nil for non-deployed service
// Test: GetActorPID returns nil when runtime not started
func TestOkraRuntime_GetActorPID(t *testing.T) {
	// Helper to create a simple test package
	createSimpleTestPackage := func() *ServicePackage {
		// Load the math-service WASM as it's a known good test file
		wasmPath := filepath.Join("..", "..", "internal", "wasm", "fixture", "math-service", "math-service.wasm")
		wasmBytes, err := os.ReadFile(wasmPath)
		require.NoError(t, err)
		
		// Create compiled module
		ctx := context.Background()
		module, err := wasm.NewWASMCompiledModule(ctx, wasmBytes)
		require.NoError(t, err)
		
		return &ServicePackage{
			ServiceName: "TestService",
			Module:      module,
			Schema: &schema.Schema{
				Meta: schema.Metadata{
					Namespace: "test",
					Version:   "v1",
				},
			},
			Config: &config.Config{},
		}
	}

	tests := []struct {
		name           string
		setupRuntime   func() *OkraRuntime
		serviceName    string
		expectedResult bool
	}{
		{
			name: "returns PID for deployed service",
			setupRuntime: func() *OkraRuntime {
				logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
				runtime := NewOkraRuntime(logger)
				err := runtime.Start(context.Background())
				require.NoError(t, err)
				
				// Deploy a service
				pkg := createSimpleTestPackage()
				_, err = runtime.Deploy(context.Background(), pkg)
				require.NoError(t, err)
				
				return runtime
			},
			serviceName:    "test.TestService.v1",
			expectedResult: true,
		},
		{
			name: "returns nil for non-deployed service",
			setupRuntime: func() *OkraRuntime {
				logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
				runtime := NewOkraRuntime(logger)
				err := runtime.Start(context.Background())
				require.NoError(t, err)
				return runtime
			},
			serviceName:    "test.NonExistent.v1",
			expectedResult: false,
		},
		{
			name: "returns nil when runtime not started",
			setupRuntime: func() *OkraRuntime {
				logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
				return NewOkraRuntime(logger)
			},
			serviceName:    "test.TestService.v1",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := tt.setupRuntime()
			defer runtime.Shutdown(context.Background())

			pid := runtime.GetActorPID(tt.serviceName)
			
			if tt.expectedResult {
				assert.NotNil(t, pid, "expected PID to be returned")
			} else {
				assert.Nil(t, pid, "expected nil PID")
			}
		})
	}
}