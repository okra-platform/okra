package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tochemey/goakt/v2/actors"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Mock implementations
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

type mockRuntimeFactory struct {
	mock.Mock
}

func (m *mockRuntimeFactory) NewRuntime(logger zerolog.Logger) runtime.Runtime {
	args := m.Called(logger)
	return args.Get(0).(runtime.Runtime)
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

type mockGatewayFactory struct {
	mock.Mock
}

func (m *mockGatewayFactory) NewConnectGateway() runtime.ConnectGateway {
	args := m.Called()
	return args.Get(0).(runtime.ConnectGateway)
}

func (m *mockGatewayFactory) NewGraphQLGateway() runtime.GraphQLGateway {
	args := m.Called()
	return args.Get(0).(runtime.GraphQLGateway)
}

type mockAdminServer struct {
	mock.Mock
}

func (m *mockAdminServer) Start(ctx context.Context, port int) error {
	args := m.Called(ctx, port)
	return args.Error(0)
}

type mockAdminServerFactory struct {
	mock.Mock
}

func (m *mockAdminServerFactory) NewAdminServer(runtime runtime.Runtime, connectGateway runtime.ConnectGateway, graphqlGateway runtime.GraphQLGateway) AdminServer {
	args := m.Called(runtime, connectGateway, graphqlGateway)
	return args.Get(0).(AdminServer)
}

type mockHTTPServer struct {
	mock.Mock
}

func (m *mockHTTPServer) ListenAndServe() error {
	args := m.Called()
	return args.Error(0)
}

type mockHTTPServerFactory struct {
	mock.Mock
}

func (m *mockHTTPServerFactory) NewHTTPServer(addr string, handler http.Handler) HTTPServer {
	args := m.Called(addr, handler)
	return args.Get(0).(HTTPServer)
}

type mockSignalNotifier struct {
	mock.Mock
}

func (m *mockSignalNotifier) Notify(c chan<- os.Signal, sig ...os.Signal) {
	m.Called(c, sig)
}

func (m *mockSignalNotifier) Stop(c chan<- os.Signal) {
	m.Called(c)
}

type mockOutput struct {
	messages []string
}

func (m *mockOutput) Printf(format string, a ...interface{}) {
	m.messages = append(m.messages, fmt.Sprintf(format, a...))
}

func (m *mockOutput) Println(a ...interface{}) {
	m.messages = append(m.messages, fmt.Sprint(a...))
}

// Tests
func TestServeCommand_Execute_Success(t *testing.T) {
	// Test: Successful serve command execution
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create mocks
	mockRT := new(mockRuntime)
	mockRTFactory := new(mockRuntimeFactory)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)
	mockGWFactory := new(mockGatewayFactory)
	mockAdminSrv := new(mockAdminServer)
	mockAdminFactory := new(mockAdminServerFactory)
	mockHTTPSrv := new(mockHTTPServer)
	mockHTTPFactory := new(mockHTTPServerFactory)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	// Set up expectations
	mockRTFactory.On("NewRuntime", mock.Anything).Return(mockRT)
	mockRT.On("Start", mock.Anything).Return(nil)
	mockRT.On("Shutdown", mock.Anything).Return(nil)
	
	mockGWFactory.On("NewConnectGateway").Return(mockConnectGW)
	mockGWFactory.On("NewGraphQLGateway").Return(mockGraphQLGW)
	mockConnectGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mockGraphQLGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	mockAdminFactory.On("NewAdminServer", mockRT, mockConnectGW, mockGraphQLGW).Return(mockAdminSrv)
	mockAdminSrv.On("Start", mock.Anything, 8081).Return(nil)
	
	mockHTTPFactory.On("NewHTTPServer", ":8080", mock.Anything).Return(mockHTTPSrv)
	mockHTTPSrv.On("ListenAndServe").Return(nil)
	
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	// Create command with mocked dependencies
	cmd := &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory:     mockRTFactory,
			GatewayFactory:     mockGWFactory,
			AdminServerFactory: mockAdminFactory,
			HTTPServerFactory:  mockHTTPFactory,
			SignalNotifier:     mockSigNotifier,
			Logger:             zerolog.Nop(),
			Output:             output,
		},
	}

	// Execute
	err := cmd.Execute(ctx, ServeOptions{})
	assert.NoError(t, err)

	// Verify output
	assert.Contains(t, output.messages, "Starting service gateway on port 8080...\n")
	assert.Contains(t, output.messages, "Starting admin server on port 8081...\n")
	assert.Contains(t, output.messages, "Context cancelled, shutting down...")
	assert.Contains(t, output.messages, "Serve shutdown complete")

	// Verify all mocks were called
	mockRTFactory.AssertExpectations(t)
	mockRT.AssertExpectations(t)
	mockGWFactory.AssertExpectations(t)
	mockAdminFactory.AssertExpectations(t)
}

func TestServeCommand_Execute_RuntimeStartError(t *testing.T) {
	// Test: Runtime fails to start
	ctx := context.Background()

	mockRT := new(mockRuntime)
	mockRTFactory := new(mockRuntimeFactory)
	output := &mockOutput{}

	mockRTFactory.On("NewRuntime", mock.Anything).Return(mockRT)
	mockRT.On("Start", mock.Anything).Return(errors.New("runtime start failed"))

	cmd := &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory: mockRTFactory,
			Logger:         zerolog.Nop(),
			Output:         output,
		},
	}

	err := cmd.Execute(ctx, ServeOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start runtime")
}

func TestServeCommand_Execute_CustomPorts(t *testing.T) {
	// Test: Custom ports are used
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create minimal mocks
	mockRT := new(mockRuntime)
	mockRTFactory := new(mockRuntimeFactory)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)
	mockGWFactory := new(mockGatewayFactory)
	mockAdminSrv := new(mockAdminServer)
	mockAdminFactory := new(mockAdminServerFactory)
	mockHTTPSrv := new(mockHTTPServer)
	mockHTTPFactory := new(mockHTTPServerFactory)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	// Set up expectations with custom ports
	mockRTFactory.On("NewRuntime", mock.Anything).Return(mockRT)
	mockRT.On("Start", mock.Anything).Return(nil)
	mockRT.On("Shutdown", mock.Anything).Return(nil)
	
	mockGWFactory.On("NewConnectGateway").Return(mockConnectGW)
	mockGWFactory.On("NewGraphQLGateway").Return(mockGraphQLGW)
	mockConnectGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mockGraphQLGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	mockAdminFactory.On("NewAdminServer", mockRT, mockConnectGW, mockGraphQLGW).Return(mockAdminSrv)
	mockAdminSrv.On("Start", mock.Anything, 9091).Return(nil) // Custom admin port
	
	mockHTTPFactory.On("NewHTTPServer", ":9090", mock.Anything).Return(mockHTTPSrv) // Custom service port
	mockHTTPSrv.On("ListenAndServe").Return(nil)
	
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	cmd := &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory:     mockRTFactory,
			GatewayFactory:     mockGWFactory,
			AdminServerFactory: mockAdminFactory,
			HTTPServerFactory:  mockHTTPFactory,
			SignalNotifier:     mockSigNotifier,
			Logger:             zerolog.Nop(),
			Output:             output,
		},
	}

	// Execute with custom ports
	err := cmd.Execute(ctx, ServeOptions{
		ServicePort: 9090,
		AdminPort:   9091,
	})
	assert.NoError(t, err)

	// Verify custom ports in output
	assert.Contains(t, output.messages, "Starting service gateway on port 9090...\n")
	assert.Contains(t, output.messages, "Starting admin server on port 9091...\n")
}

func TestServeCommand_Execute_ServerError(t *testing.T) {
	// Test: Server returns error
	ctx := context.Background()

	// Create mocks
	mockRT := new(mockRuntime)
	mockRTFactory := new(mockRuntimeFactory)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)
	mockGWFactory := new(mockGatewayFactory)
	mockAdminSrv := new(mockAdminServer)
	mockAdminFactory := new(mockAdminServerFactory)
	mockHTTPSrv := new(mockHTTPServer)
	mockHTTPFactory := new(mockHTTPServerFactory)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	// Set up expectations
	mockRTFactory.On("NewRuntime", mock.Anything).Return(mockRT)
	mockRT.On("Start", mock.Anything).Return(nil)
	mockRT.On("Shutdown", mock.Anything).Return(nil)
	
	mockGWFactory.On("NewConnectGateway").Return(mockConnectGW)
	mockGWFactory.On("NewGraphQLGateway").Return(mockGraphQLGW)
	mockConnectGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mockGraphQLGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	mockAdminFactory.On("NewAdminServer", mockRT, mockConnectGW, mockGraphQLGW).Return(mockAdminSrv)
	mockAdminSrv.On("Start", mock.Anything, 8081).Return(nil)
	
	mockHTTPFactory.On("NewHTTPServer", ":8080", mock.Anything).Return(mockHTTPSrv)
	mockHTTPSrv.On("ListenAndServe").Return(errors.New("port already in use"))
	
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	cmd := &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory:     mockRTFactory,
			GatewayFactory:     mockGWFactory,
			AdminServerFactory: mockAdminFactory,
			HTTPServerFactory:  mockHTTPFactory,
			SignalNotifier:     mockSigNotifier,
			Logger:             zerolog.Nop(),
			Output:             output,
		},
	}

	// Execute
	err := cmd.Execute(ctx, ServeOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service gateway error")
	
	// Check that error message was printed
	found := false
	for _, msg := range output.messages {
		if strings.Contains(msg, "Server error:") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected 'Server error:' in output messages")
}

func TestServeCommand_Execute_SignalHandling(t *testing.T) {
	// Test: Signal handling
	ctx := context.Background()

	// Create mocks
	mockRT := new(mockRuntime)
	mockRTFactory := new(mockRuntimeFactory)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)
	mockGWFactory := new(mockGatewayFactory)
	mockAdminSrv := new(mockAdminServer)
	mockAdminFactory := new(mockAdminServerFactory)
	mockHTTPSrv := new(mockHTTPServer)
	mockHTTPFactory := new(mockHTTPServerFactory)
	output := &mockOutput{}

	// Custom signal notifier that sends signal immediately
	mockSigNotifier := &mockSignalNotifier{}
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		c := args.Get(0).(chan<- os.Signal)
		// Send signal after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			c <- os.Interrupt
		}()
	})
	mockSigNotifier.On("Stop", mock.Anything).Return()

	// Set up expectations
	mockRTFactory.On("NewRuntime", mock.Anything).Return(mockRT)
	mockRT.On("Start", mock.Anything).Return(nil)
	mockRT.On("Shutdown", mock.Anything).Return(nil)
	
	mockGWFactory.On("NewConnectGateway").Return(mockConnectGW)
	mockGWFactory.On("NewGraphQLGateway").Return(mockGraphQLGW)
	mockConnectGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mockGraphQLGW.On("Handler").Return(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	mockAdminFactory.On("NewAdminServer", mockRT, mockConnectGW, mockGraphQLGW).Return(mockAdminSrv)
	mockAdminSrv.On("Start", mock.Anything, 8081).Return(nil)
	
	mockHTTPFactory.On("NewHTTPServer", ":8080", mock.Anything).Return(mockHTTPSrv)
	mockHTTPSrv.On("ListenAndServe").Return(nil)

	cmd := &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory:     mockRTFactory,
			GatewayFactory:     mockGWFactory,
			AdminServerFactory: mockAdminFactory,
			HTTPServerFactory:  mockHTTPFactory,
			SignalNotifier:     mockSigNotifier,
			Logger:             zerolog.Nop(),
			Output:             output,
		},
	}

	// Execute
	err := cmd.Execute(ctx, ServeOptions{})
	assert.NoError(t, err)

	// Verify signal handling output - check in combined messages
	allMessages := strings.Join(output.messages, "")
	assert.Contains(t, allMessages, "Received signal")
	assert.Contains(t, allMessages, "shutting down...")
}

func TestNewServeCommand(t *testing.T) {
	// Test: NewServeCommand creates command with default dependencies
	cmd := NewServeCommand()
	
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.deps.RuntimeFactory)
	assert.NotNil(t, cmd.deps.GatewayFactory)
	assert.NotNil(t, cmd.deps.AdminServerFactory)
	assert.NotNil(t, cmd.deps.HTTPServerFactory)
	assert.NotNil(t, cmd.deps.SignalNotifier)
	assert.NotNil(t, cmd.deps.Logger)
	assert.NotNil(t, cmd.deps.Output)
}

func TestServeCommand_WithDependencies(t *testing.T) {
	// Test: WithDependencies allows injecting custom dependencies
	cmd := NewServeCommand()
	customDeps := ServeDependencies{
		Output: &mockOutput{},
	}
	
	cmd.WithDependencies(customDeps)
	assert.Equal(t, customDeps.Output, cmd.deps.Output)
}

// Test that we can create ServeOptions
func TestServeOptions(t *testing.T) {
	// Test: ServeOptions can be created and used
	opts := ServeOptions{
		ServicePort: 8090,
		AdminPort:   8091,
	}
	
	assert.Equal(t, 8090, opts.ServicePort)
	assert.Equal(t, 8091, opts.AdminPort)
}