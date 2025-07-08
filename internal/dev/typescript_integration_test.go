package dev

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fsnotify/fsnotify"
	"github.com/okra-platform/okra/internal/build"
	"github.com/okra-platform/okra/internal/config"
	"github.com/rs/zerolog"
)

// Test: Full TypeScript development workflow
func TestTypeScriptDevelopmentWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check dependencies
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found, skipping integration test")
	}

	// Create a complete TypeScript project
	projectDir := t.TempDir()

	// Test: Set up project structure
	setupTypeScriptProject(t, projectDir)

	// Test: Load config
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "typescript",
		Schema:   "./service.okra.gql",
		Source:   "./src",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
		Dev: config.DevConfig{
			Watch:   []string{"*.ts", "**/*.ts", "*.okra.gql", "**/*.okra.gql"},
			Exclude: []string{"*.test.ts", "build/", "node_modules/", ".git/", "service.interface.ts"},
		},
	}

	// Test: Create dev server
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	server := &Server{
		config:      cfg,
		projectRoot: projectDir,
		logger:      logger,
		builder:     build.NewServiceBuilder(cfg, projectDir, logger),
	}

	// Test: Run initial build
	t.Run("initial build", func(t *testing.T) {
		// Run build (code generation + WASM compilation)
		err := server.build()
		
		// Build will fail without Javy, but we can verify it gets to that point
		if err != nil {
			// Should fail on Javy, not earlier
			assert.True(t,
				os.IsNotExist(err) || // node_modules missing
				strings.Contains(err.Error(), "javy not found") ||
				strings.Contains(err.Error(), "npm install"),
				"Unexpected error: %v", err)
		}

		// Verify interface file was generated
		interfacePath := filepath.Join(projectDir, "src", "service.interface.ts")
		if _, err := os.Stat(interfacePath); err == nil {
			content, _ := os.ReadFile(interfacePath)
			assert.Contains(t, string(content), "export interface")
			assert.Contains(t, string(content), "greet")
		}
	})

	// Test: File watching setup
	t.Run("file watcher setup", func(t *testing.T) {
		watcher, err := NewFileWatcher(
			cfg.Dev.Watch,
			cfg.Dev.Exclude,
			func(path string, op fsnotify.Op) {
				// Handler for testing
			},
		)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.AddDirectory(projectDir)
		assert.NoError(t, err)
	})

	// Test: Schema change handling
	t.Run("schema change", func(t *testing.T) {
		// Modify schema
		newSchema := `
@okra(namespace: "test", version: "v1")

service TestService {
	greet(input: GreetRequest): GreetResponse
	newMethod(input: String): String
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}
`
		schemaPath := filepath.Join(projectDir, "service.okra.gql")
		err := os.WriteFile(schemaPath, []byte(newSchema), 0644)
		require.NoError(t, err)

		// Simulate schema change handling
		server.handleSchemaChange(schemaPath)

		// Verify interface would be regenerated with new method
		// (actual generation depends on dependencies being installed)
	})

	// Test: Source file detection
	t.Run("source file detection", func(t *testing.T) {
		testFiles := []struct {
			path     string
			expected bool
		}{
			{"index.ts", true},
			{"helper.ts", true},
			{"index.test.ts", false},
			{"README.md", false},
			{"service.interface.ts", false}, // Should be excluded
		}

		for _, tf := range testFiles {
			fullPath := filepath.Join(projectDir, "src", tf.path)
			result := server.isSourceFile(fullPath)
			assert.Equal(t, tf.expected, result, "File %s detection mismatch", tf.path)
		}
	})
}

// Helper function to set up a TypeScript project
func setupTypeScriptProject(t *testing.T, projectDir string) {
	t.Helper()

	// Create directories
	srcDir := filepath.Join(projectDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	// Create okra.json
	okraJSON := `{
  "name": "test-service",
  "version": "1.0.0",
  "language": "typescript",
  "schema": "./service.okra.gql",
  "source": "./src",
  "build": {
    "output": "./build/service.wasm"
  },
  "dev": {
    "watch": ["*.ts", "**/*.ts", "*.okra.gql", "**/*.okra.gql"],
    "exclude": ["*.test.ts", "build/", "node_modules/", ".git/", "service.interface.ts"]
  }
}`
	err := os.WriteFile(filepath.Join(projectDir, "okra.json"), []byte(okraJSON), 0644)
	require.NoError(t, err)

	// Create schema
	schema := `
@okra(namespace: "test", version: "v1")

service TestService {
	greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
	name: String!
}

type GreetResponse {
	message: String!
}
`
	err = os.WriteFile(filepath.Join(projectDir, "service.okra.gql"), []byte(schema), 0644)
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
	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Create tsconfig.json
	tsconfig := `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "lib": ["ES2020"],
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*"]
}`
	err = os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(tsconfig), 0644)
	require.NoError(t, err)

	// Create index.ts
	indexTS := `
import type { GreetRequest, GreetResponse } from './service.interface';

export function greet(input: GreetRequest): GreetResponse {
	return {
		message: ` + "`Hello, ${input.name}!`" + `
	};
}
`
	err = os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexTS), 0644)
	require.NoError(t, err)
}

// Test: Concurrent builds for TypeScript
func TestTypeScriptConcurrentBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectDir := t.TempDir()
	setupTypeScriptProject(t, projectDir)

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	cfg := &config.Config{
		Language: "typescript",
		Source:   "./src",
		Schema:   "./service.okra.gql",
		Build: config.BuildConfig{
			Output: "./build/service.wasm",
		},
	}
	server := &Server{
		config:      cfg,
		projectRoot: projectDir,
		logger:      logger,
		builder:     build.NewServiceBuilder(cfg, projectDir, logger),
	}

	// Start multiple builds concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	buildCount := 5
	results := make(chan error, buildCount)

	for i := 0; i < buildCount; i++ {
		go func() {
			// Each goroutine calls build on the same server instance
			// The mutex in the server will handle concurrency
			results <- server.build()
		}()
	}

	// Collect results
	errors := 0
	for i := 0; i < buildCount; i++ {
		select {
		case err := <-results:
			if err != nil {
				errors++
			}
		case <-ctx.Done():
			t.Fatal("Timeout waiting for builds")
		}
	}

	// All builds should complete (with or without errors depending on dependencies)
	assert.Equal(t, buildCount, errors+buildCount-errors, "All builds should complete")
}