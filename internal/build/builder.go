// Package build provides shared build functionality for OKRA services
package build

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/rs/zerolog"
)

// BuildArtifacts contains all the generated build artifacts
type BuildArtifacts struct {
	// WASMPath is the path to the compiled WASM module
	WASMPath string
	
	// ProtobufDescriptorPath is the path to the generated protobuf descriptor
	ProtobufDescriptorPath string
	
	// InterfacePath is the path to the generated interface code
	InterfacePath string
	
	// Schema contains the parsed schema
	Schema *schema.Schema
	
	// BuildInfo contains build metadata
	BuildInfo BuildInfo
}

// BuildInfo contains metadata about the build
type BuildInfo struct {
	// Timestamp when the build was created
	Timestamp time.Time
	
	// Version from config
	Version string
	
	// Language used for the build
	Language string
	
	// SchemaChecksum for validation
	SchemaChecksum string
}

// Builder provides the interface for building OKRA services
type Builder interface {
	// GenerateCode generates interface code from the schema
	GenerateCode(schemaPath string) error
	
	// BuildWASM compiles the source code to WASM
	BuildWASM() error
	
	// GetArtifacts returns the build artifacts after a successful build
	GetArtifacts() (*BuildArtifacts, error)
	
	// Clean removes any temporary build artifacts
	Clean() error
}

// ServiceBuilder implements the Builder interface for building OKRA services
type ServiceBuilder struct {
	config      *config.Config
	projectRoot string
	logger      zerolog.Logger
	
	// Build state
	schema     *schema.Schema
	okraDir    string
	buildStart time.Time
}

// NewServiceBuilder creates a new service builder
func NewServiceBuilder(cfg *config.Config, projectRoot string, logger zerolog.Logger) *ServiceBuilder {
	return &ServiceBuilder{
		config:      cfg,
		projectRoot: projectRoot,
		logger:      logger,
		okraDir:     filepath.Join(projectRoot, ".okra"),
	}
}

// GenerateCode generates interface code from the schema
func (b *ServiceBuilder) GenerateCode(schemaPath string) error {
	b.buildStart = time.Now()
	
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
	
	b.logger.Debug().
		Str("path", schemaPath).
		Int("size", len(content)).
		Msg("read schema file")

	// Parse schema
	parsedSchema, err := schema.ParseSchema(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}
	
	b.schema = parsedSchema
	
	// Validate schema has services
	if len(parsedSchema.Services) == 0 {
		return fmt.Errorf("no services defined in schema")
	}
	
	b.logger.Debug().
		Int("service_count", len(parsedSchema.Services)).
		Msg("parsed schema services")

	// Create .okra directory if it doesn't exist
	if err := os.MkdirAll(b.okraDir, 0755); err != nil {
		return fmt.Errorf("failed to create .okra directory: %w", err)
	}

	// Generate language-specific interface
	if err := b.generateLanguageInterface(parsedSchema); err != nil {
		return fmt.Errorf("failed to generate interface: %w", err)
	}

	// Generate protobuf descriptor
	if err := b.generateProtobufDescriptor(parsedSchema); err != nil {
		return fmt.Errorf("failed to generate protobuf: %w", err)
	}
	
	return nil
}

// BuildWASM compiles the source code to WASM
func (b *ServiceBuilder) BuildWASM() error {
	if b.schema == nil {
		return fmt.Errorf("GenerateCode must be called before BuildWASM")
	}
	
	b.logger.Debug().Str("language", b.config.Language).Msg("starting WASM build")
	
	// Ensure build directory exists
	buildDir := filepath.Dir(filepath.Join(b.projectRoot, b.config.Build.Output))
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Delegate to language-specific builder
	switch b.config.Language {
	case "go":
		return b.buildGoWASM()
	case "typescript":
		return b.buildTypeScriptWASM()
	default:
		return fmt.Errorf("unsupported language: %s", b.config.Language)
	}
}

// GetArtifacts returns the build artifacts after a successful build
func (b *ServiceBuilder) GetArtifacts() (*BuildArtifacts, error) {
	if b.schema == nil {
		return nil, fmt.Errorf("no build has been performed")
	}
	
	// Verify WASM output exists
	wasmPath := filepath.Join(b.projectRoot, b.config.Build.Output)
	if _, err := os.Stat(wasmPath); err != nil {
		return nil, fmt.Errorf("WASM output not found: %w", err)
	}
	
	// Get interface path based on language
	var interfacePath string
	switch b.config.Language {
	case "go":
		interfacePath = filepath.Join(b.projectRoot, "types", "interface.go")
	case "typescript":
		interfacePath = filepath.Join(b.projectRoot, "types", "interface.ts")
	}
	
	artifacts := &BuildArtifacts{
		WASMPath:               wasmPath,
		ProtobufDescriptorPath: filepath.Join(b.okraDir, "service.pb.desc"),
		InterfacePath:          interfacePath,
		Schema:                 b.schema,
		BuildInfo: BuildInfo{
			Timestamp:      b.buildStart,
			Version:        b.config.Version,
			Language:       b.config.Language,
			SchemaChecksum: "", // TODO: implement checksum
		},
	}
	
	return artifacts, nil
}

// Clean removes any temporary build artifacts
func (b *ServiceBuilder) Clean() error {
	// For now, we don't clean anything as builds happen in temp directories
	// This could be extended to clean .okra directory if needed
	return nil
}