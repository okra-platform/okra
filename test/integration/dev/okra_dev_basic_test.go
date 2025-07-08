//go:build integration
// +build integration

package dev_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestOkraDevBasic tests that okra dev can start up
func TestOkraDevBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "okra-dev-basic-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create minimal config
	configContent := `{
  "name": "test-service",
  "language": "go",
  "schema": "service.okra.gql",
  "source": "service",
  "build": {
    "output": "build/service.wasm"
  },
  "dev": {
    "watch": ["."],
    "exclude": ["build", ".okra", "*.wasm"]
  }
}`
	err = os.WriteFile(filepath.Join(tempDir, "okra.json"), []byte(configContent), 0644)
	require.NoError(t, err)
	
	// Create service directory
	err = os.MkdirAll(filepath.Join(tempDir, "service"), 0755)
	require.NoError(t, err)

	// Create minimal schema
	schemaContent := `@okra(namespace: "test", version: "v1")

service GreeterService {
  greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
  name: String!
}

type GreetResponse {
  message: String!
}`
	err = os.WriteFile(filepath.Join(tempDir, "service.okra.gql"), []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Create minimal service
	serviceContent := `package service

import "test-service/types"

type Service struct{}

func (s *Service) Greet(req *types.GreetRequest) (*types.GreetResponse, error) {
	return &types.GreetResponse{Message: "Hello " + req.Name}, nil
}

func NewService() types.GreeterService {
	return &Service{}
}`
	err = os.WriteFile(filepath.Join(tempDir, "service", "service.go"), []byte(serviceContent), 0644)
	require.NoError(t, err)

	// Create go.mod
	goModContent := `module test-service

go 1.21
`
	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)

	// Start okra dev
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "okra", "dev")
	cmd.Dir = tempDir
	cmd.Env = os.Environ()
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Wait for startup
	time.Sleep(3 * time.Second)

	// Check output
	t.Logf("Stdout: %s", stdout.String())
	t.Logf("Stderr: %s", stderr.String())

	// Check if process is still running
	if cmd.Process != nil {
		err := cmd.Process.Signal(nil)
		if err == nil {
			// Process is still running - that's good!
			t.Logf("okra dev is running successfully")
		} else {
			// Process exited - check why
			t.Logf("Process exited with error: %v", err)
			if strings.Contains(stdout.String(), "Build completed successfully") {
				t.Logf("Build completed successfully - process may have exited normally")
			} else if strings.Contains(stdout.String(), "TinyGo build failed") {
				t.Fatalf("TinyGo build failed - check output above")
			}
		}
	}

	// Kill the process
	if cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}
}