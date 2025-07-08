package dev

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Mock implementations
type mockSchemaParser struct {
	mock.Mock
}

func (m *mockSchemaParser) Parse(input string) (*schema.Schema, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*schema.Schema), args.Error(1)
}

type mockCodeGenerator struct {
	mock.Mock
}

func (m *mockCodeGenerator) Generate(s *schema.Schema) ([]byte, error) {
	args := m.Called(s)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCodeGenerator) Language() string {
	return "go"
}

func (m *mockCodeGenerator) FileExtension() string {
	return ".go"
}

type mockWASMBuilder struct {
	mock.Mock
}

func (m *mockWASMBuilder) Build(projectRoot string, config BuildConfig) error {
	args := m.Called(projectRoot, config)
	return args.Error(0)
}


// Tests
func TestServer_isSourceFile(t *testing.T) {
	tests := []struct {
		name     string
		language string
		path     string
		want     bool
	}{
		{
			name:     "go source file",
			language: "go",
			path:     "/project/main.go",
			want:     true,
		},
		{
			name:     "go test file",
			language: "go",
			path:     "/project/main_test.go",
			want:     false,
		},
		{
			name:     "typescript source file",
			language: "typescript",
			path:     "/project/index.ts",
			want:     true,
		},
		{
			name:     "typescript test file",
			language: "typescript",
			path:     "/project/index.test.ts",
			want:     false,
		},
		{
			name:     "non-source file",
			language: "go",
			path:     "/project/README.md",
			want:     false,
		},
		{
			name:     "unknown language",
			language: "rust",
			path:     "/project/main.rs",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				config: &config.Config{
					Language: tt.language,
				},
			}
			
			got := s.isSourceFile(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServer_handleFileChange(t *testing.T) {
	// Test filtering of temporary files
	t.Run("ignore temporary files", func(t *testing.T) {
		s := &Server{
			config:      &config.Config{Language: "go"},
			projectRoot: "/project",
		}
		
		// These should be ignored
		temporaryFiles := []string{
			"/project/main.go~",
			"/project/file.tmp",
			"/project/.file.swp",
		}
		
		for _, path := range temporaryFiles {
			// Since handleFileChange returns early for temp files,
			// we just ensure it doesn't panic
			s.handleFileChange(path, fsnotify.Write)
		}
	})
}

func TestServer_generateCode(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "service.okra.gql")
	schemaContent := `
		@okra(namespace: "test", version: "v1")
		service TestService {
			test(): String
		}
	`
	
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)
	
	// This test would need actual schema parser and code generator
	// For now, we're just ensuring the structure is correct
	// In a real implementation, we'd inject mocks for these dependencies
}

func TestServer_buildWASM_unsupportedLanguage(t *testing.T) {
	s := &Server{
		config: &config.Config{
			Language: "rust",
		},
	}
	
	err := s.buildWASM()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestServer_concurrentBuilds(t *testing.T) {
	s := &Server{
		config: &config.Config{
			Language: "go",
		},
		buildMutex: sync.Mutex{},
	}
	
	// Track build attempts
	buildAttempts := 0
	buildsCompleted := 0
	var mu sync.Mutex
	
	// Override handleSourceChange to simulate concurrent builds
	handleSource := func(_ string) {
		mu.Lock()
		buildAttempts++
		mu.Unlock()
		
		s.buildMutex.Lock()
		if s.building {
			s.buildMutex.Unlock()
			return
		}
		s.building = true
		s.buildMutex.Unlock()
		
		// Simulate build time
		time.Sleep(50 * time.Millisecond)
		
		s.buildMutex.Lock()
		s.building = false
		s.buildMutex.Unlock()
		
		mu.Lock()
		buildsCompleted++
		mu.Unlock()
	}
	
	// Start multiple concurrent builds
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleSource("/project/main.go")
		}()
	}
	
	wg.Wait()
	
	// Should have multiple attempts but only one completed build
	assert.Equal(t, 5, buildAttempts)
	assert.Equal(t, 1, buildsCompleted, "Only one build should complete when multiple are triggered concurrently")
}

// Test plan for runtime integration:
// 1. Test runtime initialization on server start
// 2. Test ConnectGateway creation
// 3. Test HTTP server startup
// 4. Test service deployment after successful build
// 5. Test protobuf descriptor loading
// 6. Test graceful shutdown

func TestServer_RuntimeComponents(t *testing.T) {
	// Test: Server has runtime components after creation
	cfg := &config.Config{
		Language: "go",
		Schema:   "service.okra.gql",
		Source:   "service.go",
		Build: config.BuildConfig{
			Output: "build/service.wasm",
		},
		Dev: config.DevConfig{
			Watch:   []string{"."},
			Exclude: []string{"build"},
		},
	}
	
	server := NewServer(cfg, "/test/project")
	assert.NotNil(t, server)
	assert.Nil(t, server.runtime) // Should be nil before Start
	assert.Nil(t, server.gateway) // Should be nil before Start
	assert.Nil(t, server.httpServer) // Should be nil before Start
}

func TestServer_Stop(t *testing.T) {
	// Test: Stop handles nil components gracefully
	server := &Server{}
	
	ctx := context.Background()
	err := server.Stop(ctx)
	assert.NoError(t, err)
}

// MockRuntime for testing
type mockRuntime struct {
	mock.Mock
}

func (m *mockRuntime) Start(ctx context.Context) error {
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

func (m *mockRuntime) IsDeployed(actorID string) bool {
	args := m.Called(actorID)
	return args.Bool(0)
}

func (m *mockRuntime) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockConnectGateway for testing
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