package serve

import (
	"context"
	"net/http"

	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/mock"
	"github.com/tochemey/goakt/v2/actors"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Mock implementations for testing admin server
type mockRuntime struct {
	mock.Mock
}

func (m *mockRuntime) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockRuntime) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockRuntime) Deploy(ctx context.Context, pkg *runtime.ServicePackage) (string, error) {
	args := m.Called(ctx, pkg)
	return args.String(0), args.Error(1)
}

func (m *mockRuntime) Undeploy(ctx context.Context, actorID string) error {
	args := m.Called(ctx, actorID)
	return args.Error(0)
}

func (m *mockRuntime) Invoke(ctx context.Context, actorID, method string, input []byte) ([]byte, error) {
	args := m.Called(ctx, actorID, method, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockRuntime) GetConnectGateway() runtime.ConnectGateway {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(runtime.ConnectGateway)
}

func (m *mockRuntime) GetGraphQLGateway() runtime.GraphQLGateway {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(runtime.GraphQLGateway)
}

func (m *mockRuntime) IsDeployed(actorID string) bool {
	args := m.Called(actorID)
	return args.Bool(0)
}

func (m *mockRuntime) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockConnectGateway struct {
	mock.Mock
}

func (m *mockConnectGateway) Handler() http.Handler {
	args := m.Called()
	return args.Get(0).(http.Handler)
}

func (m *mockConnectGateway) UpdateService(ctx context.Context, serviceName string, fds *descriptorpb.FileDescriptorSet, actorPID *actors.PID) error {
	args := m.Called(ctx, serviceName, fds, actorPID)
	return args.Error(0)
}

func (m *mockConnectGateway) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockGraphQLGateway struct {
	mock.Mock
}

func (m *mockGraphQLGateway) Handler() http.Handler {
	args := m.Called()
	return args.Get(0).(http.Handler)
}

func (m *mockGraphQLGateway) UpdateService(ctx context.Context, namespace string, serviceSchema *schema.Schema, actorPID *actors.PID) error {
	args := m.Called(ctx, namespace, serviceSchema, actorPID)
	return args.Error(0)
}

func (m *mockGraphQLGateway) RemoveService(ctx context.Context, namespace string) error {
	args := m.Called(ctx, namespace)
	return args.Error(0)
}

func (m *mockGraphQLGateway) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Mock for LoadPackage function
type packageLoader func(ctx context.Context, source string) (*runtime.ServicePackage, error)

var mockLoadPackage packageLoader

// Helper to set the mock for tests
func setMockLoadPackage(loader packageLoader) {
	mockLoadPackage = loader
}

// Helper to reset after tests
func resetMockLoadPackage() {
	mockLoadPackage = nil
}