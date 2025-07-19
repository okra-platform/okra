package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okra-platform/okra/internal/config"
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
