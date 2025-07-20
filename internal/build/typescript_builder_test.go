package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
)

// Test: TypeScript builder constructor
func TestNewTypeScriptBuilder(t *testing.T) {
	cfg := &config.Config{
		Language: "typescript",
		Source:   "./src",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}

	testSchema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{Name: "testMethod"},
				},
			},
		},
	}

	builder := NewTypeScriptBuilder(cfg, "/project", testSchema)

	assert.NotNil(t, builder)
	assert.Equal(t, cfg, builder.config)
	assert.Equal(t, "/project", builder.projectRoot)
	assert.Equal(t, testSchema, builder.schema)
}

// Test: Extract service methods from schema
func TestTypeScriptBuilder_getServiceMethods(t *testing.T) {
	tests := []struct {
		name     string
		schema   *schema.Schema
		expected []string
	}{
		{
			name: "single service with multiple methods",
			schema: &schema.Schema{
				Services: []schema.Service{
					{
						Name: "UserService",
						Methods: []schema.Method{
							{Name: "getUser"},
							{Name: "createUser"},
							{Name: "updateUser"},
							{Name: "deleteUser"},
						},
					},
				},
			},
			expected: []string{"getUser", "createUser", "updateUser", "deleteUser"},
		},
		{
			name: "multiple services",
			schema: &schema.Schema{
				Services: []schema.Service{
					{
						Name: "UserService",
						Methods: []schema.Method{
							{Name: "getUser"},
							{Name: "createUser"},
						},
					},
					{
						Name: "OrderService",
						Methods: []schema.Method{
							{Name: "createOrder"},
							{Name: "getOrder"},
						},
					},
				},
			},
			expected: []string{"getUser", "createUser", "createOrder", "getOrder"},
		},
		{
			name: "empty schema",
			schema: &schema.Schema{
				Services: []schema.Service{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &TypeScriptBuilder{
				schema: tt.schema,
			}

			methods := builder.getServiceMethods()
			assert.Equal(t, tt.expected, methods)
		})
	}
}

// Test: Generate wrapper file
func TestTypeScriptBuilder_generateWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	wrapperPath := filepath.Join(tmpDir, "service.wrapper.ts")

	builder := &TypeScriptBuilder{
		config: &config.Config{
			Source: "./src",
		},
		projectRoot: "/project",
	}

	methods := []string{"greet", "sayHello", "getUserInfo"}

	err := builder.generateWrapper(wrapperPath, methods)
	require.NoError(t, err)

	// Check wrapper file was created
	assert.FileExists(t, wrapperPath)

	// Read and verify content
	content, err := os.ReadFile(wrapperPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check imports
	assert.Contains(t, contentStr, "import { readInput, writeOutput, log } from './javy-runtime';")
	assert.Contains(t, contentStr, "import * as service from")

	// Check method handlers
	assert.Contains(t, contentStr, "'greet': service.greet,")
	assert.Contains(t, contentStr, "'sayHello': service.sayHello,")
	assert.Contains(t, contentStr, "'getUserInfo': service.getUserInfo,")

	// Check Javy entry point
	assert.Contains(t, contentStr, "(globalThis as any).Shopify = { main };")
}

// Test: Generate build files
func TestTypeScriptBuilder_generateBuildFiles(t *testing.T) {
	tmpDir := t.TempDir()

	builder := &TypeScriptBuilder{
		config: &config.Config{
			Source: "./src",
		},
		projectRoot: "/project",
	}

	methods := []string{"testMethod"}

	err := builder.generateBuildFiles(tmpDir, methods)
	require.NoError(t, err)

	// Check runtime file
	runtimePath := filepath.Join(tmpDir, "javy-runtime.ts")
	assert.FileExists(t, runtimePath)

	runtimeContent, err := os.ReadFile(runtimePath)
	require.NoError(t, err)
	assert.Contains(t, string(runtimeContent), "declare const Javy:")
	assert.Contains(t, string(runtimeContent), "export function readInput()")
	assert.Contains(t, string(runtimeContent), "export function writeOutput(")

	// Check wrapper file
	wrapperPath := filepath.Join(tmpDir, "service.wrapper.ts")
	assert.FileExists(t, wrapperPath)
}

// Test: Dependency checking
func TestTypeScriptBuilder_checkDependencies(t *testing.T) {
	// Test: Plan test cases for different scenarios
	// Test: npm not installed
	// Test: javy not installed
	// Test: node_modules missing
	// Test: all dependencies present

	t.Run("all dependencies present", func(t *testing.T) {
		// This test assumes npm is installed on the test system
		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not found, skipping test")
		}

		tmpDir := t.TempDir()

		// Create fake node_modules
		nodeModulesPath := filepath.Join(tmpDir, "node_modules")
		err := os.MkdirAll(nodeModulesPath, 0755)
		require.NoError(t, err)

		builder := &TypeScriptBuilder{
			projectRoot: tmpDir,
		}

		err = builder.checkDependencies()
		// Will fail on Javy check unless it's installed
		if err != nil {
			assert.Contains(t, err.Error(), "javy")
		}
	})

	t.Run("node_modules missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		builder := &TypeScriptBuilder{
			projectRoot: tmpDir,
		}

		err := builder.checkDependencies()
		if err != nil && strings.Contains(err.Error(), "npm not found") {
			t.Skip("npm not installed")
		}

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "node_modules not found")
	})
}

// Test: Copy file functionality
func TestTypeScriptBuilder_copyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.wasm")
	srcContent := []byte("test wasm content")
	err := os.WriteFile(srcPath, srcContent, 0644)
	require.NoError(t, err)

	// Test copy to new location with directory creation
	dstPath := filepath.Join(tmpDir, "output", "build", "final.wasm")

	builder := &TypeScriptBuilder{}
	err = builder.copyFile(srcPath, dstPath)
	require.NoError(t, err)

	// Verify file was copied
	assert.FileExists(t, dstPath)

	// Verify content matches
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, srcContent, dstContent)
}

// Test: Build integration (requires tools to be installed)
func TestTypeScriptBuilder_Build_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if required tools are available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found, skipping integration test")
	}

	// Check if javy is available (in PATH or tools/bin)
	if _, err := findJavy(); err != nil {
		t.Skip("javy not found, skipping integration test")
	}

	// Create a minimal TypeScript project
	tmpDir := t.TempDir()

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)

	// Create index.ts
	indexContent := `
export function greet(input: { name: string }): { message: string } {
    return { message: "Hello, " + input.name + "!" };
}

export function add(input: { a: number; b: number }): { result: number } {
    return { result: input.a + input.b };
}
`
	err = os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create package.json
	packageJSON := `{
  "name": "test-service",
  "version": "1.0.0",
  "devDependencies": {
    "esbuild": "^0.20.0",
    "typescript": "^5.0.0"
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Run npm install
	cmd := exec.Command("npm", "install")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("npm install failed: %s\n%s", err, string(output))
	}

	// Create config and schema
	cfg := &config.Config{
		Language: "typescript",
		Source:   "./src",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}

	testSchema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{Name: "greet"},
					{Name: "add"},
				},
			},
		},
	}

	// Run the build
	builder := NewTypeScriptBuilder(cfg, tmpDir, testSchema)
	err = builder.Build()

	// The build should succeed now that we have javy
	require.NoError(t, err, "TypeScript build should succeed")
	
	// Verify the output
	wasmPath := filepath.Join(tmpDir, cfg.Build.Output)
	assert.FileExists(t, wasmPath)
	
	// Verify the WASM file is not empty
	info, err := os.Stat(wasmPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "WASM file should not be empty")
}
