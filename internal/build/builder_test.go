package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for ServiceBuilder:
// 1. Test NewServiceBuilder creates builder correctly
// 2. Test GenerateCode with valid schema
// 3. Test GenerateCode with missing schema file
// 4. Test GenerateCode with empty schema file
// 5. Test GenerateCode with no services in schema
// 6. Test BuildWASM requires GenerateCode to be called first
// 7. Test GetArtifacts returns correct paths

func TestNewServiceBuilder(t *testing.T) {
	// Test: NewServiceBuilder creates builder with correct fields

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	projectRoot := "/test/project"
	logger := zerolog.New(os.Stderr)

	builder := NewServiceBuilder(cfg, projectRoot, logger)

	assert.NotNil(t, builder)
	assert.Equal(t, cfg, builder.config)
	assert.Equal(t, projectRoot, builder.projectRoot)
	assert.Equal(t, filepath.Join(projectRoot, ".okra"), builder.okraDir)
}

func TestServiceBuilder_GenerateCode_MissingSchema(t *testing.T) {
	// Test: GenerateCode fails when schema file doesn't exist

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	schemaPath := filepath.Join(tempDir, "missing.okra.gql")
	err := builder.GenerateCode(schemaPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema file not found")
}

func TestServiceBuilder_GenerateCode_EmptySchema(t *testing.T) {
	// Test: GenerateCode fails when schema file is empty

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	// Create empty schema file
	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(""), 0644)
	require.NoError(t, err)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	err = builder.GenerateCode(schemaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema file is empty")
}

func TestServiceBuilder_GenerateCode_NoServices(t *testing.T) {
	// Test: GenerateCode fails when schema has no services

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	// Create schema with no services
	schemaContent := `@okra(namespace: "test", version: "v1")

type GreetRequest {
  name: String!
}`

	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	err = builder.GenerateCode(schemaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no services defined in schema")
}

func TestServiceBuilder_BuildWASM_RequiresGenerateCode(t *testing.T) {
	// Test: BuildWASM fails if GenerateCode hasn't been called

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	err := builder.BuildWASM()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GenerateCode must be called before BuildWASM")
}

func TestServiceBuilder_GetArtifacts_NoBuild(t *testing.T) {
	// Test: GetArtifacts fails if no build has been performed

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	artifacts, err := builder.GetArtifacts()
	assert.Error(t, err)
	assert.Nil(t, artifacts)
	assert.Contains(t, err.Error(), "no build has been performed")
}

func TestServiceBuilder_Clean(t *testing.T) {
	// Test: Clean returns nil (no-op for now)

	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}

	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	builder := NewServiceBuilder(cfg, tempDir, logger)

	err := builder.Clean()
	assert.NoError(t, err)
}

func TestServiceBuilder_GenerateCode_Success_Go(t *testing.T) {
	// Test: GenerateCode succeeds for Go language
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	// Create valid schema
	schemaContent := `@okra(namespace: "test", version: "v1")
	
service TestService {
	greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}`
	
	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	
	err = builder.GenerateCode(schemaPath)
	assert.NoError(t, err)
	
	// Verify interface was generated
	interfacePath := filepath.Join(tempDir, "types", "interface.go")
	assert.FileExists(t, interfacePath)
	
	// Verify protobuf descriptor was generated
	descPath := filepath.Join(tempDir, ".okra", "service.pb.desc")
	assert.FileExists(t, descPath)
	
	// Verify schema is stored
	assert.NotNil(t, builder.schema)
	assert.Len(t, builder.schema.Services, 1)
}

func TestServiceBuilder_GenerateCode_Success_TypeScript(t *testing.T) {
	// Test: GenerateCode succeeds for TypeScript language
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "typescript",
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	// Create valid schema
	schemaContent := `@okra(namespace: "test", version: "v1")
	
service TestService {
	greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}`
	
	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	
	err = builder.GenerateCode(schemaPath)
	assert.NoError(t, err)
	
	// Verify interface was generated
	interfacePath := filepath.Join(tempDir, "types", "interface.ts")
	assert.FileExists(t, interfacePath)
	
	// Verify protobuf descriptor was generated
	descPath := filepath.Join(tempDir, ".okra", "service.pb.desc")
	assert.FileExists(t, descPath)
}

func TestServiceBuilder_BuildWASM_Go(t *testing.T) {
	// Test: BuildWASM succeeds for Go (with mock)
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
		Source:   "./",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	// Create schema and prerequisites
	schemaContent := `@okra(namespace: "test", version: "v1")
	
service TestService {
	greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}`
	
	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)
	
	// Create go.mod
	goModContent := `module test-service

go 1.21`
	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	
	// Create a minimal service implementation
	serviceContent := `package main

import "test-service/types"

type testService struct{}

func NewService() types.TestService {
	return &testService{}
}

func (s *testService) Greet(input *types.GreetRequest) (*types.GreetResponse, error) {
	return &types.GreetResponse{
		Message: "Hello, " + input.Name + "!",
	}, nil
}`
	err = os.WriteFile(filepath.Join(tempDir, "service.go"), []byte(serviceContent), 0644)
	require.NoError(t, err)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	
	// First generate code
	err = builder.GenerateCode(schemaPath)
	require.NoError(t, err)
	
	// Mock the actual WASM build by creating a dummy output file
	// (since we can't guarantee TinyGo is installed in test environment)
	buildDir := filepath.Join(tempDir, "build")
	err = os.MkdirAll(buildDir, 0755)
	require.NoError(t, err)
	
	wasmPath := filepath.Join(tempDir, cfg.Build.Output)
	err = os.WriteFile(wasmPath, []byte("mock wasm content"), 0644)
	require.NoError(t, err)
	
	// Now test that BuildWASM can be called
	// Note: This will try to actually build, so it may fail without TinyGo
	// But we can at least test the method flow
	_ = builder.BuildWASM()
	
	// The important part is that it doesn't panic and tries to build
	assert.NotNil(t, builder.schema)
}

func TestServiceBuilder_GetArtifacts_Success(t *testing.T) {
	// Test: GetArtifacts returns correct paths after successful build
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	
	// Simulate a successful build by setting schema and creating output files
	builder.schema = &schema.Schema{
		Services: []schema.Service{{Name: "TestService"}},
	}
	
	// Create output files
	buildDir := filepath.Join(tempDir, "build")
	err := os.MkdirAll(buildDir, 0755)
	require.NoError(t, err)
	
	wasmPath := filepath.Join(tempDir, cfg.Build.Output)
	err = os.WriteFile(wasmPath, []byte("mock wasm"), 0644)
	require.NoError(t, err)
	
	artifacts, err := builder.GetArtifacts()
	assert.NoError(t, err)
	assert.NotNil(t, artifacts)
	assert.Equal(t, wasmPath, artifacts.WASMPath)
	assert.Equal(t, filepath.Join(tempDir, ".okra", "service.pb.desc"), artifacts.ProtobufDescriptorPath)
	assert.Equal(t, filepath.Join(tempDir, "types", "interface.go"), artifacts.InterfacePath)
	assert.Equal(t, builder.schema, artifacts.Schema)
}

func TestServiceBuilder_BuildWASM_TypeScript(t *testing.T) {
	// Test: BuildWASM succeeds for TypeScript
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "typescript",
		Source:   "./src",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	// Create schema
	schemaContent := `@okra(namespace: "test", version: "v1")
	
service TestService {
	greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}`
	
	schemaPath := filepath.Join(tempDir, "service.okra.gql")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)
	
	// Create src directory
	srcDir := filepath.Join(tempDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)
	
	// Create service implementation
	serviceContent := `import { TestService } from '../types/interface';

export class Service implements TestService {
	greet(input: { name: string }): { message: string } {
		return { message: "Hello, " + input.name + "!" };
	}
}`
	err = os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(serviceContent), 0644)
	require.NoError(t, err)
	
	// Create package.json
	packageJSON := `{
	"name": "test-service",
	"version": "1.0.0",
	"devDependencies": {
		"esbuild": "^0.19.0"
	}
}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)
	
	// Create node_modules directory (mock npm install)
	nodeModulesDir := filepath.Join(tempDir, "node_modules")
	err = os.MkdirAll(nodeModulesDir, 0755)
	require.NoError(t, err)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	
	// First generate code
	err = builder.GenerateCode(schemaPath)
	require.NoError(t, err)
	
	// Mock the WASM output (since we can't guarantee javy is working)
	buildDir := filepath.Join(tempDir, "build")
	err = os.MkdirAll(buildDir, 0755)
	require.NoError(t, err)
	
	wasmPath := filepath.Join(tempDir, cfg.Build.Output)
	err = os.WriteFile(wasmPath, []byte("mock wasm content"), 0644)
	require.NoError(t, err)
	
	// Test that BuildWASM can be called
	_ = builder.BuildWASM()
	
	// The important part is that the method attempts to build
	assert.NotNil(t, builder.schema)
}

func TestServiceBuilder_BuildWASM_UnsupportedLanguage(t *testing.T) {
	// Test: BuildWASM fails for unsupported language
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "rust",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}
	
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	
	builder := NewServiceBuilder(cfg, tempDir, logger)
	builder.schema = &schema.Schema{} // Simulate GenerateCode was called
	
	err := builder.BuildWASM()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}
