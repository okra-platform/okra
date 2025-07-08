package dev

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/okra-platform/okra/internal/build"
	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
	"github.com/rs/zerolog"
)

// Server represents the development server
type Server struct {
	config      *config.Config
	projectRoot string
	watcher     *FileWatcher
	logger      zerolog.Logger
	
	// Runtime components
	runtime        runtime.Runtime
	connectGateway runtime.ConnectGateway
	graphqlGateway runtime.GraphQLGateway
	httpServer     *http.Server
	
	// Current deployment state
	currentActorID   string
	currentServiceMu sync.RWMutex
	
	// Mutex to prevent concurrent builds
	buildMutex sync.Mutex
	building   bool
	
	// Shared builder instance
	builder build.Builder
}

// NewServer creates a new development server
func NewServer(cfg *config.Config, projectRoot string) *Server {
	// Create a logger for the dev server
	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("component", "dev-server").
		Logger()
	
	return &Server{
		config:      cfg,
		projectRoot: projectRoot,
		logger:      logger,
		builder:     build.NewServiceBuilder(cfg, projectRoot, logger),
	}
}

// Start runs the development server
func (s *Server) Start(ctx context.Context) error {
	// Check required tools before starting
	if err := s.checkRequiredTools(); err != nil {
		return err
	}
	
	// Validate configuration
	if err := s.validateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Initialize runtime with a logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	s.runtime = runtime.NewOkraRuntime(logger)
	if err := s.runtime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start runtime: %w", err)
	}
	fmt.Println("🚀 Runtime started successfully")
	
	// Initialize gateways
	s.connectGateway = runtime.NewConnectGateway()
	s.graphqlGateway = runtime.NewGraphQLGateway()
	
	// Start HTTP server
	if err := s.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	
	// Run initial build and deployment
	if err := s.buildAndDeploy(); err != nil {
		fmt.Printf("❌ Initial build failed: %v\n", err)
		fmt.Println("\n💡 Troubleshooting tips:")
		fmt.Println("   - Check that your schema file exists and is valid")
		fmt.Println("   - Ensure your source files match the configured language")
		fmt.Println("   - Verify all required tools are installed (tinygo, buf)")
		return fmt.Errorf("initial build failed: %w", err)
	}

	// Set up file watcher
	watcher, err := NewFileWatcher(
		s.config.Dev.Watch,
		s.config.Dev.Exclude,
		s.handleFileChange,
	)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	s.watcher = watcher
	defer s.watcher.Close()

	// Add project root to watcher
	if err := s.watcher.AddDirectory(s.projectRoot); err != nil {
		return fmt.Errorf("failed to watch project directory: %w", err)
	}

	fmt.Println("\n✨ Development server is running. Press Ctrl+C to stop.")
	fmt.Println("👀 Watching for changes...")

	// Start watching
	return s.watcher.Start(ctx)
}

// checkRequiredTools verifies all necessary tools are installed.
// This prevents cryptic errors later in the build process by checking upfront
// that all required development tools are available in the system PATH.
// The function provides helpful error messages with installation instructions
// when tools are missing.
func (s *Server) checkRequiredTools() error {
	fmt.Println("🔍 Checking required tools...")
	s.logger.Debug().Msg("checking required tools for development")
	
	// Check language-specific tools
	switch s.config.Language {
	case "go":
		// Check for TinyGo
		tinygoPath, err := exec.LookPath("tinygo")
		if err != nil {
			s.logger.Error().Err(err).Msg("TinyGo not found")
			return fmt.Errorf("TinyGo is required but not installed.\n   Please install it from: https://tinygo.org/getting-started/install/")
		}
		fmt.Println("   ✅ TinyGo found")
		s.logger.Debug().Str("path", tinygoPath).Msg("TinyGo executable found")
		
		// Check for Go
		goPath, err := exec.LookPath("go")
		if err != nil {
			s.logger.Error().Err(err).Msg("Go not found")
			return fmt.Errorf("Go is required but not installed.\n   Please install it from: https://golang.org/dl/")
		}
		fmt.Println("   ✅ Go found")
		s.logger.Debug().Str("path", goPath).Msg("Go executable found")
		
	case "typescript":
		// Check for Node.js
		if _, err := exec.LookPath("node"); err != nil {
			return fmt.Errorf("Node.js is required but not installed.\n   Please install it from: https://nodejs.org/")
		}
		fmt.Println("   ✅ Node.js found")
	}
	
	// Check for buf (required for all languages)
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf CLI is required but not installed.\n   Please install it from: https://buf.build/docs/installation")
	}
	fmt.Println("   ✅ buf found")
	
	return nil
}

// validateConfig checks that the configuration is valid before starting the dev server.
// This includes verifying that referenced files exist and that the configuration
// uses supported values. Early validation provides better error messages than
// failing during runtime operations.
func (s *Server) validateConfig() error {
	// Check schema file exists
	schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
	if _, err := os.Stat(schemaPath); err != nil {
		return fmt.Errorf("schema file not found: %s", schemaPath)
	}
	
	// Check source file/directory exists
	sourcePath := filepath.Join(s.projectRoot, s.config.Source)
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source path not found: %s", sourcePath)
	}
	
	// Validate language
	switch s.config.Language {
	case "go", "typescript":
		// Valid languages
	default:
		return fmt.Errorf("unsupported language: %s (supported: go, typescript)", s.config.Language)
	}
	
	return nil
}

// Stop gracefully shuts down the development server
func (s *Server) Stop(ctx context.Context) error {
	fmt.Println("\n🛑 Shutting down development server...")
	
	// Stop HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Warning: failed to shutdown HTTP server: %v\n", err)
		}
	}
	
	// Shutdown gateways
	if s.connectGateway != nil {
		if err := s.connectGateway.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Warning: failed to shutdown ConnectRPC gateway: %v\n", err)
		}
	}
	
	if s.graphqlGateway != nil {
		if err := s.graphqlGateway.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Warning: failed to shutdown GraphQL gateway: %v\n", err)
		}
	}
	
	// Shutdown runtime
	if s.runtime != nil {
		if err := s.runtime.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Warning: failed to shutdown runtime: %v\n", err)
		}
	}
	
	fmt.Println("✅ Development server stopped")
	return nil
}

// handleFileChange is called when a watched file changes
func (s *Server) handleFileChange(path string, op fsnotify.Op) {
	// Ignore temporary files and build artifacts
	if strings.Contains(path, ".tmp") || strings.Contains(path, "~") {
		return
	}

	relPath, _ := filepath.Rel(s.projectRoot, path)
	
	var action string
	switch op {
	case fsnotify.Create:
		action = "created"
	case fsnotify.Write:
		action = "modified"
	case fsnotify.Remove:
		action = "deleted"
	case fsnotify.Rename:
		action = "renamed"
	default:
		return
	}

	fmt.Printf("\n📝 File %s: %s\n", action, relPath)

	// Determine what kind of file changed
	if strings.HasSuffix(path, ".okra.gql") {
		s.handleSchemaChange(path)
	} else if s.isSourceFile(path) {
		s.handleSourceChange(path)
	}
}

// isSourceFile checks if a file is a source file based on the project language
func (s *Server) isSourceFile(path string) bool {
	switch s.config.Language {
	case "go":
		return strings.HasSuffix(path, ".go") && 
			!strings.HasSuffix(path, "_test.go") &&
			!strings.HasSuffix(path, "interface.go")
	case "typescript":
		return strings.HasSuffix(path, ".ts") && 
			!strings.HasSuffix(path, ".test.ts") &&
			!strings.HasSuffix(path, ".interface.ts")
	default:
		return false
	}
}

// handleSchemaChange handles changes to .okra.gql files
func (s *Server) handleSchemaChange(path string) {
	fmt.Println("🔄 Schema changed, regenerating interface...")
	
	s.buildMutex.Lock()
	s.building = true
	s.buildMutex.Unlock()
	
	defer func() {
		s.buildMutex.Lock()
		s.building = false
		s.buildMutex.Unlock()
	}()

	// Rebuild and redeploy with new schema
	if err := s.build(); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		return
	}

	// Deploy the updated service
	if err := s.deployServicePackage(); err != nil {
		fmt.Printf("❌ Service deployment failed: %v\n", err)
		return
	}

	fmt.Println("✅ Build and deployment completed successfully!")
}

// handleSourceChange handles changes to source code files
func (s *Server) handleSourceChange(path string) {
	fmt.Println("🔄 Source changed, rebuilding WASM...")
	
	s.buildMutex.Lock()
	if s.building {
		s.buildMutex.Unlock()
		fmt.Println("⏳ Build already in progress, skipping...")
		return
	}
	s.building = true
	s.buildMutex.Unlock()
	
	defer func() {
		s.buildMutex.Lock()
		s.building = false
		s.buildMutex.Unlock()
	}()

	// Only rebuild WASM (no need to regenerate interface for source changes)
	if err := s.buildWASM(); err != nil {
		fmt.Printf("❌ WASM build failed: %v\n", err)
		return
	}
	
	// Deploy the updated service
	if err := s.deployServicePackage(); err != nil {
		fmt.Printf("❌ Service deployment failed: %v\n", err)
		return
	}

	fmt.Println("✅ Build and deployment completed successfully!")
}

// build performs code generation and WASM compilation without deployment
func (s *Server) build() error {
	// Generate service interface from schema
	schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
	if err := s.generateCode(schemaPath); err != nil {
		return fmt.Errorf("interface generation failed: %w", err)
	}

	// Build WASM
	if err := s.buildWASM(); err != nil {
		return fmt.Errorf("WASM build failed: %w", err)
	}

	return nil
}

// buildAndDeploy runs the complete build pipeline including deployment to the runtime
func (s *Server) buildAndDeploy() error {
	fmt.Println("🔨 Running initial build...")
	
	// First build the code
	if err := s.build(); err != nil {
		return err
	}

	// Then deploy the service package
	if err := s.deployServicePackage(); err != nil {
		return fmt.Errorf("failed to deploy service package: %w", err)
	}
	
	fmt.Println("✅ Initial build completed successfully!")
	return nil
}

// startHTTPServer starts the HTTP server for the ConnectGateway
func (s *Server) startHTTPServer(ctx context.Context) error {
	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	
	// Create HTTP server with both gateways
	mux := http.NewServeMux()
	mux.Handle("/connect/", s.connectGateway.Handler())
	mux.Handle("/graphql/", s.graphqlGateway.Handler())
	
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	// Start server in background
	go func() {
		fmt.Printf("🌐 HTTP server listening on http://localhost:%d\n", port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ HTTP server error: %v\n", err)
		}
	}()
	
	return nil
}

// deployServicePackage loads the built artifacts and deploys to the runtime
func (s *Server) deployServicePackage() error {
	ctx := context.Background()
	
	// Load schema
	schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
	schemaContent, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}
	
	parsedSchema, err := schema.ParseSchema(string(schemaContent))
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}
	
	if len(parsedSchema.Services) == 0 {
		return fmt.Errorf("no services defined in schema")
	}
	
	serviceName := parsedSchema.Services[0].Name
	
	// Load WASM module
	wasmPath := filepath.Join(s.projectRoot, s.config.Build.Output)
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return fmt.Errorf("WASM file not found at %s. Build may have failed", wasmPath)
	}
	
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to read WASM file at %s: %w", wasmPath, err)
	}
	
	if len(wasmBytes) == 0 {
		return fmt.Errorf("WASM file is empty: %s", wasmPath)
	}
	
	compiledModule, err := wasm.NewWASMCompiledModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile WASM module: %w", err)
	}
	
	// Create service package
	pkg, err := runtime.NewServicePackage(compiledModule, parsedSchema, s.config)
	if err != nil {
		return fmt.Errorf("failed to create service package: %w", err)
	}
	
	// Load protobuf descriptors if available
	descPath := filepath.Join(s.projectRoot, ".okra", "service.pb.desc")
	s.logger.Debug().Str("path", descPath).Msg("checking for protobuf descriptors")
	
	if _, err := os.Stat(descPath); err == nil {
		fds, err := runtime.LoadFileDescriptors(descPath)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to load protobuf descriptors: %v\n", err)
			s.logger.Error().Err(err).Str("path", descPath).Msg("failed to load protobuf descriptors")
		} else {
			pkg.WithFileDescriptors(fds)
			fmt.Printf("📦 Loaded protobuf descriptors from %s\n", descPath)
			s.logger.Debug().Str("path", descPath).Msg("protobuf descriptors loaded successfully")
		}
	} else {
		fmt.Printf("⚠️  Warning: protobuf descriptor file not found: %s\n", descPath)
		s.logger.Warn().Err(err).Str("path", descPath).Msg("protobuf descriptor file not found")
	}
	
	// Check if service is already deployed
	s.currentServiceMu.RLock()
	currentActorID := s.currentActorID
	s.currentServiceMu.RUnlock()
	
	// Undeploy existing service if needed
	if currentActorID != "" {
		fmt.Printf("🔄 Redeploying service: %s\n", serviceName)
		if err := s.runtime.Undeploy(ctx, currentActorID); err != nil {
			fmt.Printf("⚠️  Warning: failed to undeploy existing service: %v\n", err)
		}
	}
	
	// Deploy the service
	fmt.Printf("🚀 Deploying service: %s\n", serviceName)
	actorID, err := s.runtime.Deploy(ctx, pkg)
	if err != nil {
		return fmt.Errorf("failed to deploy service: %w", err)
	}
	fmt.Printf("✅ Service deployed successfully: %s (actor: %s)\n", serviceName, actorID)
	
	// Update current actor ID
	s.currentServiceMu.Lock()
	s.currentActorID = actorID
	s.currentServiceMu.Unlock()
	
	// Get the actor PID for the service
	actorPID := s.runtime.(*runtime.OkraRuntime).GetActorPID(actorID)
	if actorPID == nil {
		return fmt.Errorf("failed to get actor PID for service %s", serviceName)
	}
	
	// Update ConnectGateway with the service
	if pkg.FileDescriptors != nil {
		s.logger.Debug().
			Str("service", serviceName).
			Str("actorID", actorID).
			Msg("updating gateway with service")
		
		if err := s.connectGateway.UpdateService(ctx, serviceName, pkg.FileDescriptors, actorPID); err != nil {
			return fmt.Errorf("failed to update ConnectRPC gateway: %w", err)
		}
		
		// Update GraphQL gateway
		namespace := pkg.Schema.Meta.Namespace
		if namespace == "" {
			namespace = "default"
		}
		if err := s.graphqlGateway.UpdateService(ctx, namespace, pkg.Schema, actorPID); err != nil {
			s.logger.Warn().Err(err).Msg("failed to update GraphQL gateway")
		}
		
		fmt.Printf("🚀 Service %s deployed and exposed via:\n", serviceName)
		fmt.Printf("   - ConnectRPC: /connect/%s.%s/*\n", namespace, serviceName)
		fmt.Printf("   - GraphQL: /graphql/%s\n", namespace)
		
		// Extract port from server address
		port := s.httpServer.Addr
		if strings.HasPrefix(port, ":") {
			port = port[1:]
		}
		fmt.Printf("   - GraphQL Playground: http://localhost:%s/graphql/%s (open in browser)\n", port, namespace)
	} else {
		fmt.Printf("⚠️  Service %s deployed without FileDescriptors - no ConnectRPC endpoint\n", serviceName)
		s.logger.Warn().
			Str("service", serviceName).
			Msg("service deployed without FileDescriptors")
	}
	
	return nil
}

// generateCode generates code from the schema
func (s *Server) generateCode(schemaPath string) error {
	start := time.Now()
	
	// Use the shared builder to generate code
	if err := s.builder.GenerateCode(schemaPath); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}
	
	// The builder has already generated everything we need
	fmt.Printf("📄 Generated code in %v\n", time.Since(start))
	
	return nil
}


// buildWASM compiles the source code to WASM
func (s *Server) buildWASM() error {
	start := time.Now()
	
	// Use the shared builder to build WASM
	if err := s.builder.BuildWASM(); err != nil {
		return fmt.Errorf("WASM build failed: %w", err)
	}
	
	// Check output file was created
	outputPath := filepath.Join(s.projectRoot, s.config.Build.Output)
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("build succeeded but output file not found: %w", err)
	}
	
	fileInfo, _ := os.Stat(outputPath)
	fmt.Printf("🏗️  Built %s (%d bytes) in %v\n", 
		filepath.Base(outputPath), 
		fileInfo.Size(), 
		time.Since(start))
	
	return nil
}