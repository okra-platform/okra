package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for dev command
type mockConfigLoader struct {
	mock.Mock
}

func (m *mockConfigLoader) LoadConfig() (*config.Config, string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*config.Config), args.String(1), args.Error(2)
}

type mockDevServer struct {
	mock.Mock
}

func (m *mockDevServer) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDevServer) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockDevServerFactory struct {
	mock.Mock
}

func (m *mockDevServerFactory) NewServer(cfg *config.Config, projectRoot string) DevServer {
	args := m.Called(cfg, projectRoot)
	return args.Get(0).(DevServer)
}

// Tests
func TestDevCommand_Execute_Success(t *testing.T) {
	// Test: Successful dev command execution
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create mocks
	mockConfig := &config.Config{
		Name:     "test-service",
		Language: "go",
		Schema:   "./service.okra.gql",
		Source:   "./",
	}
	mockLoader := new(mockConfigLoader)
	mockFactory := new(mockDevServerFactory)
	mockServer := new(mockDevServer)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	// Set up expectations
	mockLoader.On("LoadConfig").Return(mockConfig, "/test/project", nil)
	mockFactory.On("NewServer", mockConfig, "/test/project").Return(mockServer)
	mockServer.On("Start", mock.Anything).Return(nil)
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	// Create command with mocked dependencies
	cmd := &DevCommand{
		deps: DevDependencies{
			ConfigLoader:   mockLoader,
			ServerFactory:  mockFactory,
			SignalNotifier: mockSigNotifier,
			Output:         output,
		},
	}

	// Execute
	err := cmd.Execute(ctx)
	assert.NoError(t, err)

	// Verify output
	assert.Contains(t, output.messages, "🚀 Starting OKRA development server for test-service...\n")
	assert.Contains(t, output.messages, "📁 Project root: /test/project\n")
	assert.Contains(t, output.messages, "📝 Schema: /test/project/service.okra.gql\n")
	assert.Contains(t, output.messages, "🔧 Language: go\n")

	// Verify all mocks were called
	mockLoader.AssertExpectations(t)
	mockFactory.AssertExpectations(t)
	mockServer.AssertExpectations(t)
	mockSigNotifier.AssertExpectations(t)
}

func TestDevCommand_Execute_ConfigLoadError(t *testing.T) {
	// Test: Config loading fails
	ctx := context.Background()

	mockLoader := new(mockConfigLoader)
	output := &mockOutput{}

	mockLoader.On("LoadConfig").Return(nil, "", errors.New("config not found"))

	cmd := &DevCommand{
		deps: DevDependencies{
			ConfigLoader: mockLoader,
			Output:       output,
		},
	}

	err := cmd.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load project config")
	assert.Contains(t, err.Error(), "config not found")

	mockLoader.AssertExpectations(t)
}

func TestDevCommand_Execute_ServerStartError(t *testing.T) {
	// Test: Server fails to start
	ctx := context.Background()

	mockConfig := &config.Config{
		Name:     "test-service",
		Language: "go",
		Schema:   "./service.okra.gql",
	}
	mockLoader := new(mockConfigLoader)
	mockFactory := new(mockDevServerFactory)
	mockServer := new(mockDevServer)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	mockLoader.On("LoadConfig").Return(mockConfig, "/test/project", nil)
	mockFactory.On("NewServer", mockConfig, "/test/project").Return(mockServer)
	mockServer.On("Start", mock.Anything).Return(errors.New("server start failed"))
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	cmd := &DevCommand{
		deps: DevDependencies{
			ConfigLoader:   mockLoader,
			ServerFactory:  mockFactory,
			SignalNotifier: mockSigNotifier,
			Output:         output,
		},
	}

	err := cmd.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dev server error")
	assert.Contains(t, err.Error(), "server start failed")
}

func TestDevCommand_Execute_ContextCancelled(t *testing.T) {
	// Test: Context cancelled returns nil
	ctx, cancel := context.WithCancel(context.Background())

	mockConfig := &config.Config{
		Name:     "test-service",
		Language: "go",
		Schema:   "./service.okra.gql",
	}
	mockLoader := new(mockConfigLoader)
	mockFactory := new(mockDevServerFactory)
	mockServer := new(mockDevServer)
	mockSigNotifier := new(mockSignalNotifier)
	output := &mockOutput{}

	mockLoader.On("LoadConfig").Return(mockConfig, "/test/project", nil)
	mockFactory.On("NewServer", mockConfig, "/test/project").Return(mockServer)
	
	// Simulate context cancelled error
	mockServer.On("Start", mock.Anything).Run(func(args mock.Arguments) {
		cancel() // Cancel the context
	}).Return(context.Canceled)
	
	mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
	mockSigNotifier.On("Stop", mock.Anything).Return()

	cmd := &DevCommand{
		deps: DevDependencies{
			ConfigLoader:   mockLoader,
			ServerFactory:  mockFactory,
			SignalNotifier: mockSigNotifier,
			Output:         output,
		},
	}

	err := cmd.Execute(ctx)
	assert.NoError(t, err) // context.Canceled is handled gracefully
}

func TestDevCommand_Execute_SignalHandling(t *testing.T) {
	// Test: Signal handling
	ctx := context.Background()

	mockConfig := &config.Config{
		Name:     "test-service",
		Language: "go",
		Schema:   "./service.okra.gql",
	}
	mockLoader := new(mockConfigLoader)
	mockFactory := new(mockDevServerFactory)
	mockServer := new(mockDevServer)
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

	mockLoader.On("LoadConfig").Return(mockConfig, "/test/project", nil)
	mockFactory.On("NewServer", mockConfig, "/test/project").Return(mockServer)
	
	// Server should block until signal
	mockServer.On("Start", mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		<-ctx.Done()
	}).Return(context.Canceled)

	cmd := &DevCommand{
		deps: DevDependencies{
			ConfigLoader:   mockLoader,
			ServerFactory:  mockFactory,
			SignalNotifier: mockSigNotifier,
			Output:         output,
		},
	}

	err := cmd.Execute(ctx)
	assert.NoError(t, err)

	// Verify signal handling output - check in combined messages
	found := false
	for _, msg := range output.messages {
		if strings.Contains(msg, "👋 Shutting down development server...") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected shutdown message in output")
}

func TestNewDevCommand(t *testing.T) {
	// Test: NewDevCommand creates command with default dependencies
	cmd := NewDevCommand()
	
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.deps.ConfigLoader)
	assert.NotNil(t, cmd.deps.ServerFactory)
	assert.NotNil(t, cmd.deps.SignalNotifier)
	assert.NotNil(t, cmd.deps.Output)
}

func TestDevCommand_WithDependencies(t *testing.T) {
	// Test: WithDependencies allows injecting custom dependencies
	cmd := NewDevCommand()
	customDeps := DevDependencies{
		Output: &mockOutput{},
	}
	
	cmd.WithDependencies(customDeps)
	assert.Equal(t, customDeps.Output, cmd.deps.Output)
}

func TestController_DevRefactored(t *testing.T) {
	// Test: Controller's DevRefactored method exists
	// Skip the actual test since it uses real dependencies
	t.Skip("Skipping integration test that uses real dependencies")
}

func TestDevCommand_Execute_DifferentLanguages(t *testing.T) {
	// Test: Dev command handles different languages
	testCases := []struct {
		name     string
		language string
	}{
		{"Go project", "go"},
		{"TypeScript project", "typescript"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			mockConfig := &config.Config{
				Name:     "test-service",
				Language: tc.language,
				Schema:   "./service.okra.gql",
			}
			mockLoader := new(mockConfigLoader)
			mockFactory := new(mockDevServerFactory)
			mockServer := new(mockDevServer)
			mockSigNotifier := new(mockSignalNotifier)
			output := &mockOutput{}

			mockLoader.On("LoadConfig").Return(mockConfig, "/test/project", nil)
			mockFactory.On("NewServer", mockConfig, "/test/project").Return(mockServer)
			mockServer.On("Start", mock.Anything).Return(nil)
			mockSigNotifier.On("Notify", mock.Anything, mock.Anything).Return()
			mockSigNotifier.On("Stop", mock.Anything).Return()

			cmd := &DevCommand{
				deps: DevDependencies{
					ConfigLoader:   mockLoader,
					ServerFactory:  mockFactory,
					SignalNotifier: mockSigNotifier,
					Output:         output,
				},
			}

			err := cmd.Execute(ctx)
			assert.NoError(t, err)
			assert.Contains(t, output.messages, fmt.Sprintf("🔧 Language: %s\n", tc.language))
		})
	}
}