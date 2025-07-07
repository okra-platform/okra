package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDev_ConfigNotFound(t *testing.T) {
	// Create a temp directory without okra.json
	tmpDir := t.TempDir()
	
	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err := os.Chdir(tmpDir)
	require.NoError(t, err)
	
	// Create controller
	ctrl := &Controller{
		Flags: &Flags{},
	}
	
	// Run dev command
	err = ctrl.Dev(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load project config")
}

func TestDev_WithConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create a temp directory with okra.json
	tmpDir := t.TempDir()
	
	// Create okra.json
	okraJSON := `{
		"name": "test-service",
		"version": "1.0.0",
		"language": "go",
		"schema": "./service.okra.graphql",
		"source": "./",
		"build": {
			"output": "./build/service.wasm"
		},
		"dev": {
			"watch": ["*.go", "*.okra.graphql"],
			"exclude": ["*_test.go", "build/"]
		}
	}`
	
	err := os.WriteFile(filepath.Join(tmpDir, "okra.json"), []byte(okraJSON), 0644)
	require.NoError(t, err)
	
	// Create schema file
	schema := `
		@okra(namespace: "test", version: "v1")
		service TestService {
			test(): String
		}
	`
	err = os.WriteFile(filepath.Join(tmpDir, "service.okra.graphql"), []byte(schema), 0644)
	require.NoError(t, err)
	
	// Create a simple Go file
	goFile := `package main

func main() {
	println("Hello from OKRA")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goFile), 0644)
	require.NoError(t, err)
	
	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	
	// Create controller
	ctrl := &Controller{
		Flags: &Flags{},
	}
	
	// Run dev command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err = ctrl.Dev(ctx)
	// Should exit cleanly on context cancellation
	assert.NoError(t, err)
}

func TestDev_SignalHandling(t *testing.T) {
	// This test verifies that the dev command sets up signal handling
	// We can't easily test the actual signal handling in a unit test,
	// but we can verify the command starts and can be cancelled
	
	// Create a temp directory with minimal config
	tmpDir := t.TempDir()
	
	okraJSON := `{
		"name": "signal-test",
		"version": "1.0.0",
		"language": "typescript"
	}`
	
	err := os.WriteFile(filepath.Join(tmpDir, "okra.json"), []byte(okraJSON), 0644)
	require.NoError(t, err)
	
	// Create a minimal schema file
	schema := `@okra(namespace: "test", version: "v1")
service TestService {
	test(): String
}`
	err = os.WriteFile(filepath.Join(tmpDir, "service.okra.graphql"), []byte(schema), 0644)
	require.NoError(t, err)
	
	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	
	// Create controller
	ctrl := &Controller{
		Flags: &Flags{},
	}
	
	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// Run dev command - it should handle the TypeScript "not implemented" error
	err = ctrl.Dev(ctx)
	
	// The command should return an error about missing dependencies
	assert.Error(t, err)
	// It will fail on npm, node_modules, or javy check
	assert.True(t, 
		strings.Contains(err.Error(), "npm not found") ||
		strings.Contains(err.Error(), "node_modules not found") ||
		strings.Contains(err.Error(), "javy not found"),
		"Expected dependency error, got: %v", err)
}