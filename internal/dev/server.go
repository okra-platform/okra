package dev

import (
	"context"
	"fmt"
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
	"github.com/okra-platform/okra/internal/schema"
)

// Server represents the development server
type Server struct {
	config      *config.Config
	projectRoot string
	watcher     *FileWatcher
	
	// Mutex to prevent concurrent builds
	buildMutex sync.Mutex
	building   bool
}

// NewServer creates a new development server
func NewServer(cfg *config.Config, projectRoot string) *Server {
	return &Server{
		config:      cfg,
		projectRoot: projectRoot,
	}
}

// Start runs the development server
func (s *Server) Start(ctx context.Context) error {
	// Run initial build
	if err := s.buildAll(); err != nil {
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

	fmt.Println("✅ Initial build completed successfully!")
	return nil
}

// generateCode generates code from the schema
func (s *Server) generateCode(schemaPath string) error {
	start := time.Now()
	
	// Read schema file
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	// Parse schema
	parsedSchema, err := schema.ParseSchema(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	// Get appropriate code generator
	var generator codegen.Generator
	switch s.config.Language {
	case "go":
		generator = golang.NewGenerator("main")
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
	
	// Generate protobuf file
	protoGen := protobuf.NewGenerator("service")
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
	
	// Ensure build directory exists
	buildDir := filepath.Dir(filepath.Join(s.projectRoot, s.config.Build.Output))
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
		
		parsedSchema, err := schema.ParseSchema(string(schemaContent))
		if err != nil {
			return fmt.Errorf("failed to parse schema: %w", err)
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