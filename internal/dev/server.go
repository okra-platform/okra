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
	"github.com/okra-platform/okra/internal/codegen"
	"github.com/okra-platform/okra/internal/codegen/golang"
	"github.com/okra-platform/okra/internal/codegen/protobuf"
	"github.com/okra-platform/okra/internal/codegen/typescript"
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
	runtime    runtime.Runtime
	gateway    runtime.ConnectGateway
	httpServer *http.Server
	
	// Current deployment state
	currentActorID   string
	currentServiceMu sync.RWMutex
	
	// Mutex to prevent concurrent builds
	buildMutex sync.Mutex
	building   bool
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
	
	// Initialize ConnectGateway
	s.gateway = runtime.NewConnectGateway()
	
	// Start HTTP server
	if err := s.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	
	// Run initial build
	if err := s.buildAll(); err != nil {
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
	
	// Shutdown gateway
	if s.gateway != nil {
		if err := s.gateway.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Warning: failed to shutdown gateway: %v\n", err)
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

	// Regenerate service interface from schema
	schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
	if err := s.generateCode(schemaPath); err != nil {
		fmt.Printf("❌ Interface generation failed: %v\n", err)
		return
	}

	// Rebuild WASM with new interface
	if err := s.buildWASM(); err != nil {
		fmt.Printf("❌ WASM build failed: %v\n", err)
		return
	}

	fmt.Println("✅ Build completed successfully!")
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

	// Rebuild WASM
	if err := s.buildWASM(); err != nil {
		fmt.Printf("❌ WASM build failed: %v\n", err)
		return
	}
	
	// Deploy the service package
	if err := s.deployServicePackage(); err != nil {
		fmt.Printf("❌ Service deployment failed: %v\n", err)
		return
	}

	fmt.Println("✅ Build completed successfully!")
}

// buildAll runs the complete build pipeline
func (s *Server) buildAll() error {
	fmt.Println("🔨 Running initial build...")
	
	// Generate service interface from schema
	schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
	if err := s.generateCode(schemaPath); err != nil {
		return fmt.Errorf("interface generation failed: %w", err)
	}

	// Build WASM
	if err := s.buildWASM(); err != nil {
		return fmt.Errorf("WASM build failed: %w", err)
	}

	// Deploy the service package
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
	
	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.gateway.Handler(),
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
		
		if err := s.gateway.UpdateService(ctx, serviceName, pkg.FileDescriptors, actorPID); err != nil {
			return fmt.Errorf("failed to update gateway: %w", err)
		}
		fmt.Printf("🚀 Service %s deployed and exposed via ConnectRPC\n", serviceName)
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
	
	// Check schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return fmt.Errorf("schema file not found: %s", schemaPath)
	}
	
	// Read schema file
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}
	
	if len(content) == 0 {
		return fmt.Errorf("schema file is empty: %s", schemaPath)
	}
	
	// Debug: log schema content
	s.logger.Debug().
		Str("path", schemaPath).
		Str("content", string(content)).
		Int("size", len(content)).
		Msg("read schema file")

	// Parse schema
	parsedSchema, err := schema.ParseSchema(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}
	
	// Debug: log services found
	s.logger.Debug().
		Int("service_count", len(parsedSchema.Services)).
		Interface("services", parsedSchema.Services).
		Msg("parsed schema services")
	if len(parsedSchema.Services) == 0 {
		s.logger.Warn().Msg("no services found in parsed schema")
	}

	// Get appropriate code generator
	var generator codegen.Generator
	switch s.config.Language {
	case "go":
		generator = golang.NewGenerator("types")
	case "typescript":
		generator = typescript.NewGenerator("service")
	default:
		return fmt.Errorf("unsupported language: %s", s.config.Language)
	}

	// Generate code
	code, err := generator.Generate(parsedSchema)
	if err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	// Write generated interface file
	var outputPath string
	switch s.config.Language {
	case "go":
		// For Go, create types directory and write interface there
		typesDir := filepath.Join(s.projectRoot, "types")
		if err := os.MkdirAll(typesDir, 0755); err != nil {
			return fmt.Errorf("failed to create types directory: %w", err)
		}
		outputPath = filepath.Join(typesDir, "interface.go")
	case "typescript":
		outputPath = filepath.Join(s.projectRoot, s.config.Source, "service.interface.ts")
	default:
		outputPath = filepath.Join(s.projectRoot, "service.interface"+generator.FileExtension())
	}
	
	if err := os.WriteFile(outputPath, code, 0644); err != nil {
		return fmt.Errorf("failed to write generated interface: %w", err)
	}

	fmt.Printf("📄 Generated %s in %v\n", filepath.Base(outputPath), time.Since(start))
	
	// Generate protobuf definitions
	if err := s.generateProtobuf(parsedSchema); err != nil {
		return fmt.Errorf("failed to generate protobuf: %w", err)
	}
	
	return nil
}

// generateProtobuf generates protobuf definitions and compiles them with buf
func (s *Server) generateProtobuf(parsedSchema *schema.Schema) error {
	// Create .okra directory for temporary files
	okraDir := filepath.Join(s.projectRoot, ".okra")
	if err := os.MkdirAll(okraDir, 0755); err != nil {
		return fmt.Errorf("failed to create .okra directory: %w", err)
	}
	
	// Generate protobuf file using namespace from schema
	namespace := parsedSchema.Meta.Namespace
	if namespace == "" {
		namespace = "service"
	}
	protoGen := protobuf.NewGenerator(namespace)
	protoContent, err := protoGen.Generate(parsedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate protobuf: %w", err)
	}
	
	// Write protobuf file
	protoPath := filepath.Join(okraDir, "service.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0644); err != nil {
		return fmt.Errorf("failed to write protobuf file: %w", err)
	}
	
	// Check if buf is installed
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf CLI is not installed. Please install it from https://buf.build/docs/installation")
	}
	
	// Create buf.yaml if it doesn't exist
	bufYamlPath := filepath.Join(okraDir, "buf.yaml")
	bufYamlContent := `version: v1
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
`
	if err := os.WriteFile(bufYamlPath, []byte(bufYamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write buf.yaml: %w", err)
	}
	
	// Compile protobuf to descriptor set
	descPath := filepath.Join(okraDir, "service.pb.desc")
	cmd := exec.Command("buf", "build", "--output", descPath, "--as-file-descriptor-set", protoPath)
	cmd.Dir = okraDir
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to compile protobuf with buf: %w\nOutput: %s", err, output)
	}
	
	fmt.Printf("🔧 Generated protobuf descriptor: service.pb.desc\n")
	return nil
}

// buildWASM compiles the source code to WASM
func (s *Server) buildWASM() error {
	start := time.Now()
	s.logger.Debug().Str("language", s.config.Language).Msg("starting WASM build")
	
	// Ensure build directory exists
	buildDir := filepath.Dir(filepath.Join(s.projectRoot, s.config.Build.Output))
	s.logger.Debug().Str("build_dir", buildDir).Msg("ensuring build directory exists")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	switch s.config.Language {
	case "go":
		// Parse schema first to get service methods
		schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
		schemaContent, err := os.ReadFile(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to read schema: %w", err)
		}
		
		// Debug: log schema content for WASM build
		s.logger.Debug().
			Str("path", schemaPath).
			Str("content", string(schemaContent)).
			Int("size", len(schemaContent)).
			Msg("read schema file for WASM build")
		
		parsedSchema, err := schema.ParseSchema(string(schemaContent))
		if err != nil {
			return fmt.Errorf("failed to parse schema: %w", err)
		}
		
		// Debug: log services found
		s.logger.Debug().
			Int("service_count", len(parsedSchema.Services)).
			Interface("services", parsedSchema.Services).
			Msg("parsed schema services in buildWASM")
		if len(parsedSchema.Services) == 0 {
			s.logger.Error().Msg("no services found in parsed schema for WASM build")
		}
		
		// Use Go builder with hidden wrapper
		builder := NewGoBuilder(s.config, s.projectRoot, parsedSchema)
		if err := builder.Build(); err != nil {
			return fmt.Errorf("Go build failed: %w", err)
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
		
	case "typescript":
		// Parse schema first to get service methods
		schemaPath := filepath.Join(s.projectRoot, s.config.Schema)
		schemaContent, err := os.ReadFile(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to read schema: %w", err)
		}
		
		parsedSchema, err := schema.ParseSchema(string(schemaContent))
		if err != nil {
			return fmt.Errorf("failed to parse schema: %w", err)
		}
		
		// Use TypeScript builder
		builder := NewTypeScriptBuilder(s.config, s.projectRoot, parsedSchema)
		if err := builder.Build(); err != nil {
			return fmt.Errorf("TypeScript build failed: %w", err)
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
		
	default:
		return fmt.Errorf("unsupported language for WASM build: %s", s.config.Language)
	}
}