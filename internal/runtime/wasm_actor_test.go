package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
)

// MockWASMWorkerPool is a mock implementation of WASMWorkerPool
type MockWASMWorkerPool struct {
	mock.Mock
}

func (m *MockWASMWorkerPool) Invoke(ctx context.Context, method string, input []byte) ([]byte, error) {
	args := m.Called(ctx, method, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockWASMWorkerPool) ActiveWorkers() uint {
	args := m.Called()
	return args.Get(0).(uint)
}

func (m *MockWASMWorkerPool) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockWASMCompiledModule is a mock implementation of WASMCompiledModule
type MockWASMCompiledModule struct {
	mock.Mock
}

func (m *MockWASMCompiledModule) Instantiate(ctx context.Context) (wasm.WASMWorker, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(wasm.WASMWorker), args.Error(1)
}

func (m *MockWASMCompiledModule) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Helper function to create a test service package
func createTestServicePackage() *ServicePackage {
	testSchema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{
						Name:       "add",
						InputType:  "AddInput",
						OutputType: "AddOutput",
					},
					{
						Name:       "echo",
						InputType:  "EchoInput",
						OutputType: "EchoOutput",
					},
					{
						Name:       "noInput",
						InputType:  "",
						OutputType: "NoInputOutput",
					},
				},
			},
		},
	}
	
	testConfig := &config.Config{
		Name:     "test-service",
		Language: "go",
	}
	
	mockModule := &MockWASMCompiledModule{}
	
	pkg, _ := NewServicePackage(mockModule, testSchema, testConfig)
	return pkg
}

// MockWASMWorker is a mock implementation of WASMWorker
type MockWASMWorker struct {
	mock.Mock
}

func (m *MockWASMWorker) Invoke(ctx context.Context, method string, input []byte) ([]byte, error) {
	args := m.Called(ctx, method, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockWASMWorker) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Helper to create a mock pool with common expectations
func createMockPool() *MockWASMWorkerPool {
	mockPool := &MockWASMWorkerPool{}
	mockPool.On("Shutdown", mock.Anything).Return(nil).Maybe()
	return mockPool
}

func TestNewWASMActor(t *testing.T) {
	// Test: Create actor with valid package
	t.Run("valid package", func(t *testing.T) {
		pkg := createTestServicePackage()
		actor := NewWASMActor(pkg)
		
		assert.NotNil(t, actor)
		assert.Equal(t, pkg, actor.servicePackage)
		assert.Equal(t, 1, actor.minWorkers)
		assert.Equal(t, 10, actor.maxWorkers)
		assert.False(t, actor.ready)
	})
	
	// Test: Create actor with options
	t.Run("with options", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		
		actor := NewWASMActor(pkg, 
			WithMinWorkers(5),
			WithMaxWorkers(20),
			WithWorkerPool(mockPool),
		)
		
		assert.NotNil(t, actor)
		assert.Equal(t, 5, actor.minWorkers)
		assert.Equal(t, 20, actor.maxWorkers)
		assert.Equal(t, mockPool, actor.workerPool)
	})
}

func TestWASMActor_PreStart(t *testing.T) {
	// Test: PreStart creates worker pool
	t.Run("creates worker pool", func(t *testing.T) {
		// Create package with mocked module
		testSchema := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "TestService",
					Methods: []schema.Method{
						{Name: "test"},
					},
				},
			},
		}
		
		mockModule := &MockWASMCompiledModule{}
		mockWorker := &MockWASMWorker{}
		mockWorker.On("Close", mock.Anything).Return(nil).Maybe()
		mockModule.On("Instantiate", mock.Anything).Return(mockWorker, nil).Times(1)
		mockModule.On("Close", mock.Anything).Return(nil).Maybe()
		
		pkg, err := NewServicePackage(mockModule, testSchema, &config.Config{})
		require.NoError(t, err)
		
		actor := NewWASMActor(pkg)
		
		ctx := context.Background()
		err = actor.PreStart(ctx)
		
		assert.NoError(t, err)
		assert.True(t, actor.ready)
		assert.NotNil(t, actor.workerPool)
		
		// Cleanup
		actor.workerPool.Shutdown(ctx)
		mockModule.AssertExpectations(t)
	})
	
	// Test: PreStart with existing pool
	t.Run("with existing pool", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		
		ctx := context.Background()
		err := actor.PreStart(ctx)
		
		assert.NoError(t, err)
		assert.True(t, actor.ready)
		assert.Equal(t, mockPool, actor.workerPool)
	})
}

func TestWASMActor_PostStop(t *testing.T) {
	// Test: PostStop shuts down worker pool
	t.Run("shuts down worker pool", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		actor.ready = true
		
		ctx := context.Background()
		err := actor.PostStop(ctx)
		
		assert.NoError(t, err)
		assert.False(t, actor.ready)
		mockPool.AssertExpectations(t)
	})
	
	// Test: PostStop with nil pool
	t.Run("nil pool", func(t *testing.T) {
		pkg := createTestServicePackage()
		actor := NewWASMActor(pkg)
		actor.ready = true
		
		ctx := context.Background()
		err := actor.PostStop(ctx)
		
		assert.NoError(t, err)
		assert.False(t, actor.ready)
	})
}

func TestWASMActor_Receive_ServiceRequest(t *testing.T) {
	// Test: Successful service request
	t.Run("successful request", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		expectedOutput := []byte(`{"result": 3}`)
		mockPool.On("Invoke", mock.Anything, "add", []byte(`{"a": 1, "b": 2}`)).Return(expectedOutput, nil)
		
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		actor.ready = true
		
		// Create a mock receive context
		actorSystem, err := actors.NewActorSystem("test-system")
		require.NoError(t, err)
		
		err = actorSystem.Start(context.Background())
		require.NoError(t, err)
		defer actorSystem.Stop(context.Background())
		
		actorRef, err := actorSystem.Spawn(context.Background(), "test-actor", actor)
		require.NoError(t, err)
		require.NotNil(t, actorRef)
		
		// Send service request
		req := &pb.ServiceRequest{
			Id:     "test-1",
			Method: "add",
			Input:  []byte(`{"a": 1, "b": 2}`),
		}
		
		// Use Ask pattern to get response
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reply, err := actors.Ask(ctx, actorRef, req, time.Second)
		require.NoError(t, err)
		require.NotNil(t, reply)
		
		resp, ok := reply.(*pb.ServiceResponse)
		require.True(t, ok)
		assert.True(t, resp.Success)
		assert.Equal(t, "test-1", resp.Id)
		assert.Equal(t, expectedOutput, resp.Output)
		assert.Nil(t, resp.Error)
		
		mockPool.AssertExpectations(t)
	})
	
	// Test: Actor not ready
	t.Run("actor not ready", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		// PreStart will be called by GoAKT, but we'll set ready to false after spawn
		
		actorSystem, err := actors.NewActorSystem("test-system")
		require.NoError(t, err)
		
		err = actorSystem.Start(context.Background())
		require.NoError(t, err)
		defer actorSystem.Stop(context.Background())
		
		actorRef, err := actorSystem.Spawn(context.Background(), "test-actor", actor)
		require.NoError(t, err)
		require.NotNil(t, actorRef)
		
		// Manually set ready to false to simulate unready state
		actor.ready = false
		
		req := &pb.ServiceRequest{
			Id:     "test-2",
			Method: "add",
			Input:  []byte(`{"a": 1, "b": 2}`),
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reply, err := actors.Ask(ctx, actorRef, req, time.Second)
		require.NoError(t, err)
		require.NotNil(t, reply)
		
		resp, ok := reply.(*pb.ServiceResponse)
		require.True(t, ok)
		assert.False(t, resp.Success)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "actor not ready")
	})
	
	// Test: Validation error
	t.Run("validation error", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		actor.ready = true
		
		actorSystem, err := actors.NewActorSystem("test-system")
		require.NoError(t, err)
		
		err = actorSystem.Start(context.Background())
		require.NoError(t, err)
		defer actorSystem.Stop(context.Background())
		
		actorRef, err := actorSystem.Spawn(context.Background(), "test-actor", actor)
		require.NoError(t, err)
		require.NotNil(t, actorRef)
		
		req := &pb.ServiceRequest{
			Id:     "test-3",
			Method: "invalidMethod",
			Input:  []byte(`{}`),
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reply, err := actors.Ask(ctx, actorRef, req, time.Second)
		require.NoError(t, err)
		require.NotNil(t, reply)
		
		resp, ok := reply.(*pb.ServiceResponse)
		require.True(t, ok)
		assert.False(t, resp.Success)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, "VALIDATION_ERROR", resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "method not found")
	})
}

func TestWASMActor_Receive_HealthCheck(t *testing.T) {
	// Test: Health check when ready
	t.Run("health check ready", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		actor.ready = true
		
		actorSystem, err := actors.NewActorSystem("test-system")
		require.NoError(t, err)
		
		err = actorSystem.Start(context.Background())
		require.NoError(t, err)
		defer actorSystem.Stop(context.Background())
		
		actorRef, err := actorSystem.Spawn(context.Background(), "test-actor", actor)
		require.NoError(t, err)
		require.NotNil(t, actorRef)
		
		req := &pb.HealthCheck{
			Ping: "hello",
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reply, err := actors.Ask(ctx, actorRef, req, time.Second)
		require.NoError(t, err)
		require.NotNil(t, reply)
		
		resp, ok := reply.(*pb.HealthCheckResponse)
		require.True(t, ok)
		assert.Equal(t, "hello", resp.Pong)
		assert.True(t, resp.Ready)
	})
	
	// Test: Health check when not ready
	t.Run("health check not ready", func(t *testing.T) {
		pkg := createTestServicePackage()
		mockPool := createMockPool()
		actor := NewWASMActor(pkg, WithWorkerPool(mockPool))
		// PreStart will be called by GoAKT, but we'll set ready to false after spawn
		
		actorSystem, err := actors.NewActorSystem("test-system")
		require.NoError(t, err)
		
		err = actorSystem.Start(context.Background())
		require.NoError(t, err)
		defer actorSystem.Stop(context.Background())
		
		actorRef, err := actorSystem.Spawn(context.Background(), "test-actor", actor)
		require.NoError(t, err)
		require.NotNil(t, actorRef)
		
		// Manually set ready to false to simulate unready state
		actor.ready = false
		
		req := &pb.HealthCheck{
			Ping: "ping",
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reply, err := actors.Ask(ctx, actorRef, req, time.Second)
		require.NoError(t, err)
		require.NotNil(t, reply)
		
		resp, ok := reply.(*pb.HealthCheckResponse)
		require.True(t, ok)
		assert.Equal(t, "ping", resp.Pong)
		assert.False(t, resp.Ready)
	})
}

func TestWASMActor_validateRequest(t *testing.T) {
	pkg := createTestServicePackage()
	actor := NewWASMActor(pkg)
	
	tests := []struct {
		name    string
		request *pb.ServiceRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			request: &pb.ServiceRequest{
				Method: "add",
				Input:  []byte(`{"a": 1, "b": 2}`),
			},
			wantErr: false,
		},
		{
			name: "empty method",
			request: &pb.ServiceRequest{
				Method: "",
				Input:  []byte(`{}`),
			},
			wantErr: true,
			errMsg:  "method name is required",
		},
		{
			name: "unknown method",
			request: &pb.ServiceRequest{
				Method: "unknown",
				Input:  []byte(`{}`),
			},
			wantErr: true,
			errMsg:  "method not found",
		},
		{
			name: "missing input",
			request: &pb.ServiceRequest{
				Method: "add",
				Input:  []byte(``),
			},
			wantErr: true,
			errMsg:  "requires input",
		},
		{
			name: "method with no input type",
			request: &pb.ServiceRequest{
				Method: "noInput",
				Input:  []byte(``),
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := actor.validateRequest(tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServicePackage(t *testing.T) {
	// Test: Create valid package
	t.Run("valid package", func(t *testing.T) {
		testSchema := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "TestService",
					Methods: []schema.Method{
						{Name: "method1"},
						{Name: "method2"},
					},
				},
			},
		}
		
		testConfig := &config.Config{}
		mockModule := &MockWASMCompiledModule{}
		
		pkg, err := NewServicePackage(mockModule, testSchema, testConfig)
		require.NoError(t, err)
		assert.NotNil(t, pkg)
		assert.Equal(t, "TestService", pkg.ServiceName)
		assert.Len(t, pkg.Methods, 2)
		
		// Test GetMethod
		method, ok := pkg.GetMethod("method1")
		assert.True(t, ok)
		assert.NotNil(t, method)
		assert.Equal(t, "method1", method.Name)
		
		_, ok = pkg.GetMethod("unknown")
		assert.False(t, ok)
	})
	
	// Test: Error cases
	t.Run("nil module", func(t *testing.T) {
		pkg, err := NewServicePackage(nil, &schema.Schema{}, &config.Config{})
		assert.Error(t, err)
		assert.Equal(t, ErrNilModule, err)
		assert.Nil(t, pkg)
	})
	
	t.Run("nil schema", func(t *testing.T) {
		mockModule := &MockWASMCompiledModule{}
		pkg, err := NewServicePackage(mockModule, nil, &config.Config{})
		assert.Error(t, err)
		assert.Equal(t, ErrNilSchema, err)
		assert.Nil(t, pkg)
	})
	
	t.Run("nil config", func(t *testing.T) {
		mockModule := &MockWASMCompiledModule{}
		pkg, err := NewServicePackage(mockModule, &schema.Schema{}, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilConfig, err)
		assert.Nil(t, pkg)
	})
	
	t.Run("no services", func(t *testing.T) {
		mockModule := &MockWASMCompiledModule{}
		pkg, err := NewServicePackage(mockModule, &schema.Schema{}, &config.Config{})
		assert.Error(t, err)
		assert.Equal(t, ErrNoServices, err)
		assert.Nil(t, pkg)
	})
}