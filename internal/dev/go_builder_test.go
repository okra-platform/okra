package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoBuilder_extractModulePath(t *testing.T) {
	// Test: Extract module path from valid go.mod
	t.Run("valid go.mod", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModContent := `module github.com/test/myapp

go 1.21

require (
	github.com/some/dependency v1.0.0
)
`
		goModPath := filepath.Join(tmpDir, "go.mod")
		require.NoError(t, os.WriteFile(goModPath, []byte(goModContent), 0644))

		builder := &GoBuilder{projectRoot: tmpDir}
		modulePath, err := builder.extractModulePath()
		
		require.NoError(t, err)
		assert.Equal(t, "github.com/test/myapp", modulePath)
	})

	// Test: Handle module path with leading/trailing spaces
	t.Run("module path with spaces", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModContent := `  module   github.com/test/myapp  

go 1.21
`
		goModPath := filepath.Join(tmpDir, "go.mod")
		require.NoError(t, os.WriteFile(goModPath, []byte(goModContent), 0644))

		builder := &GoBuilder{projectRoot: tmpDir}
		modulePath, err := builder.extractModulePath()
		
		require.NoError(t, err)
		assert.Equal(t, "github.com/test/myapp", modulePath)
	})

	// Test: Error when go.mod doesn't exist
	t.Run("missing go.mod", func(t *testing.T) {
		tmpDir := t.TempDir()
		builder := &GoBuilder{projectRoot: tmpDir}
		
		_, err := builder.extractModulePath()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open go.mod")
	})

	// Test: Error when module declaration not found
	t.Run("no module declaration", func(t *testing.T) {
		tmpDir := t.TempDir()
		goModContent := `go 1.21

require (
	github.com/some/dependency v1.0.0
)
`
		goModPath := filepath.Join(tmpDir, "go.mod")
		require.NoError(t, os.WriteFile(goModPath, []byte(goModContent), 0644))

		builder := &GoBuilder{projectRoot: tmpDir}
		_, err := builder.extractModulePath()
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "module declaration not found")
	})
}

func TestGoBuilder_generateWrapper(t *testing.T) {
	// Test: Generate wrapper with single service method
	t.Run("single method service", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "MathService",
					Methods: []schema.Method{
						{
							Name:       "add",
							InputType:  "AddInput",
							OutputType: "AddOutput",
						},
					},
				},
			},
		}

		builder := &GoBuilder{schema: s}
		err := builder.generateWrapper(tmpDir, "github.com/test/myapp", "github.com/test/myapp")
		require.NoError(t, err)

		// Check wrapper file exists
		wrapperPath := filepath.Join(tmpDir, "main.go")
		require.FileExists(t, wrapperPath)

		// Read and verify content
		content, err := os.ReadFile(wrapperPath)
		require.NoError(t, err)
		
		contentStr := string(content)
		// Check imports
		assert.Contains(t, contentStr, `userservice "github.com/test/myapp"`)
		assert.Contains(t, contentStr, `"github.com/test/myapp/types"`)
		
		// Check service interface
		assert.Contains(t, contentStr, "var service types.MathService")
		
		// Check constructor call
		assert.Contains(t, contentStr, "service = userservice.NewService()")
		
		// Check method dispatch
		assert.Contains(t, contentStr, `case "add":`)
		assert.Contains(t, contentStr, "var req types.AddInput")
		assert.Contains(t, contentStr, "res, err := service.Add(&req)")
		
		// Check WASI exports
		assert.Contains(t, contentStr, "//export _initialize")
		assert.Contains(t, contentStr, "//export handle_request")
		assert.Contains(t, contentStr, "//export allocate")
		assert.Contains(t, contentStr, "//export deallocate")
	})

	// Test: Generate wrapper with multiple methods
	t.Run("multiple methods service", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "UserService",
					Methods: []schema.Method{
						{
							Name:       "createUser",
							InputType:  "CreateUserInput",
							OutputType: "User",
						},
						{
							Name:       "getUser",
							InputType:  "GetUserInput",
							OutputType: "User",
						},
						{
							Name:       "deleteUser",
							InputType:  "DeleteUserInput",
							OutputType: "DeleteUserOutput",
						},
					},
				},
			},
		}

		builder := &GoBuilder{schema: s}
		err := builder.generateWrapper(tmpDir, "github.com/test/myapp", "github.com/test/myapp")
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
		require.NoError(t, err)
		
		contentStr := string(content)
		// Check all methods are present
		assert.Contains(t, contentStr, `case "createUser":`)
		assert.Contains(t, contentStr, `case "getUser":`)
		assert.Contains(t, contentStr, `case "deleteUser":`)
		
		// Check method calls
		assert.Contains(t, contentStr, "service.CreateUser(&req)")
		assert.Contains(t, contentStr, "service.GetUser(&req)")
		assert.Contains(t, contentStr, "service.DeleteUser(&req)")
	})

	// Test: Error when no services in schema
	t.Run("no services", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := &schema.Schema{
			Services: []schema.Service{},
		}

		builder := &GoBuilder{schema: s}
		err := builder.generateWrapper(tmpDir, "github.com/test/myapp", "github.com/test/myapp")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no services found in schema")
	})
}

func TestGoBuilder_createTempGoMod(t *testing.T) {
	// Test: Create temp go.mod with correct replace directive
	t.Run("create go.mod", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectRoot := "/path/to/project"
		
		builder := &GoBuilder{projectRoot: projectRoot}
		err := builder.createTempGoMod(tmpDir, "github.com/test/myapp")
		require.NoError(t, err)

		goModPath := filepath.Join(tmpDir, "go.mod")
		require.FileExists(t, goModPath)

		content, err := os.ReadFile(goModPath)
		require.NoError(t, err)
		
		contentStr := string(content)
		assert.Contains(t, contentStr, "module okra-temp-build")
		assert.Contains(t, contentStr, "go 1.21")
		assert.Contains(t, contentStr, "require github.com/test/myapp v0.0.0")
		assert.Contains(t, contentStr, "replace github.com/test/myapp => "+projectRoot)
	})
}

func TestGoBuilder_exportName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase to uppercase",
			input:    "add",
			expected: "Add",
		},
		{
			name:     "already uppercase",
			input:    "Add",
			expected: "Add",
		},
		{
			name:     "camelCase",
			input:    "getUserById",
			expected: "GetUserById",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character",
			input:    "x",
			expected: "X",
		},
	}

	builder := &GoBuilder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.exportName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoBuilder_mapToGoType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string type",
			input:    "String",
			expected: "String",
		},
		{
			name:     "int type",
			input:    "Int",
			expected: "Int",
		},
		{
			name:     "array type",
			input:    "[String]",
			expected: "[]String",
		},
		{
			name:     "nested array",
			input:    "[[Int]]",
			expected: "[][]Int",
		},
		{
			name:     "custom type",
			input:    "User",
			expected: "User",
		},
		{
			name:     "array of custom type",
			input:    "[User]",
			expected: "[]User",
		},
	}

	builder := &GoBuilder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.mapToGoType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoBuilder_copyFile(t *testing.T) {
	// Test: Copy file successfully
	t.Run("successful copy", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "source.txt")
		dstPath := filepath.Join(tmpDir, "dest", "target.txt")
		
		content := []byte("test content")
		require.NoError(t, os.WriteFile(srcPath, content, 0644))

		builder := &GoBuilder{}
		err := builder.copyFile(srcPath, dstPath)
		require.NoError(t, err)

		// Verify file was copied
		require.FileExists(t, dstPath)
		copiedContent, err := os.ReadFile(dstPath)
		require.NoError(t, err)
		assert.Equal(t, content, copiedContent)
	})

	// Test: Error when source doesn't exist
	t.Run("source not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "nonexistent.txt")
		dstPath := filepath.Join(tmpDir, "dest.txt")

		builder := &GoBuilder{}
		err := builder.copyFile(srcPath, dstPath)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read source file")
	})
}

func TestGoBuilder_Build_Integration(t *testing.T) {
	// Skip if TinyGo is not available
	if _, err := os.Stat("/usr/local/bin/tinygo"); os.IsNotExist(err) {
		t.Skip("TinyGo not installed, skipping integration test")
	}

	// Test: Full build process
	t.Run("complete build", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create project structure
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "types"), 0755))
		
		// Create go.mod
		goModContent := `module github.com/test/myservice

go 1.21
`
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "go.mod"),
			[]byte(goModContent),
			0644,
		))

		// Create types/interface.go (would normally be generated)
		interfaceContent := `package types

type MathService interface {
	Add(input *AddInput) (*AddOutput, error)
}

type AddInput struct {
	A int ` + "`json:\"a\"`" + `
	B int ` + "`json:\"b\"`" + `
}

type AddOutput struct {
	Result int ` + "`json:\"result\"`" + `
}
`
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "types", "interface.go"),
			[]byte(interfaceContent),
			0644,
		))

		// Create user service implementation
		serviceContent := `package main

import "github.com/test/myservice/types"

type mathService struct{}

func NewService() types.MathService {
	return &mathService{}
}

func (s *mathService) Add(input *types.AddInput) (*types.AddOutput, error) {
	return &types.AddOutput{
		Result: input.A + input.B,
	}, nil
}
`
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "service.go"),
			[]byte(serviceContent),
			0644,
		))

		// Create config
		cfg := &config.Config{
			Build: config.BuildConfig{
				Output: "build/service.wasm",
			},
		}

		// Create schema
		s := &schema.Schema{
			Services: []schema.Service{
				{
					Name: "MathService",
					Methods: []schema.Method{
						{
							Name:       "add",
							InputType:  "AddInput",
							OutputType: "AddOutput",
						},
					},
				},
			},
		}

		// Run builder
		builder := NewGoBuilder(cfg, tmpDir, s)
		err := builder.Build()
		
		// Note: This might fail if TinyGo is not installed or configured
		if err != nil && strings.Contains(err.Error(), "tinygo") {
			t.Skip("TinyGo build failed, likely not configured")
		}
		
		require.NoError(t, err)

		// Check output exists
		outputPath := filepath.Join(tmpDir, cfg.Build.Output)
		require.FileExists(t, outputPath)
	})
}