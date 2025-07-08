//go:build integration
// +build integration

package dev_test

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/okra.json testdata/service.okra.gql
//go:embed testdata/service/*
//go:embed testdata/types/*
//go:embed testdata/build/.gitkeep
var testServiceFiles embed.FS

// TestOkraDev tests the full okra dev workflow
func TestOkraDev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "okra-dev-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Copy embedded files to temp directory
	err = copyEmbeddedFiles(tempDir)
	require.NoError(t, err)
	
	// Create go.mod in the project root (not in service directory)
	goModContent := `module test-service

go 1.21
`
	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	
	// List files in temp directory for debugging
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	t.Logf("Files in temp directory:")
	for _, f := range files {
		t.Logf("  - %s", f.Name())
	}
	
	// Also verify okra.json exists and is valid
	configPath := filepath.Join(tempDir, "okra.json")
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	t.Logf("Config file content: %s", string(configData))

	// Start okra dev in background
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "okra", "dev")
	cmd.Dir = tempDir
	cmd.Env = os.Environ() // Inherit parent environment
	
	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log the command we're running
	t.Logf("Running command: %s in directory: %s", cmd.String(), tempDir)
	
	// Verify config file exists
	okraConfigPath := filepath.Join(tempDir, "okra.json")
	if _, err := os.Stat(okraConfigPath); err != nil {
		t.Fatalf("Config file not found: %v", err)
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start okra dev: %v", err)
	}
	
	// Create a goroutine to monitor the process
	processExited := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		processExited <- err
	}()
	
	// Give it a moment to start up - use a shorter timeout first
	startupComplete := false
	for i := 0; i < 10; i++ {
		select {
		case err := <-processExited:
			// Process exited early
			t.Logf("Stdout:\n%s", stdout.String())
			t.Logf("Stderr:\n%s", stderr.String())
			
			// Check if TinyGo is missing
			if strings.Contains(stderr.String(), "tinygo") || strings.Contains(stdout.String(), "tinygo") {
				t.Skip("TinyGo not found in PATH - skipping integration test")
			}
			
			// Check for common build errors
			output := stdout.String() + stderr.String()
			if strings.Contains(output, "TinyGo error") || strings.Contains(output, "TinyGo build failed") {
				t.Logf("TinyGo build error detected")
			}
			if strings.Contains(output, "failed to generate protobuf") {
				t.Logf("Protobuf generation error detected")
			}
			if strings.Contains(output, "buf") && strings.Contains(output, "not found") {
				t.Skip("buf CLI not found - skipping test")
			}
			
			if err != nil {
				t.Fatalf("okra dev exited with error: %v", err)
			} else {
				t.Fatalf("okra dev exited with code 0")
			}
		case <-time.After(500 * time.Millisecond):
			// Check if we see the "Build completed successfully" message
			if strings.Contains(stdout.String(), "Initial build completed successfully") {
				startupComplete = true
				t.Logf("Initial build completed successfully!")
				break
			}
			// Continue waiting
		}
	}
	
	if !startupComplete {
		t.Logf("Current stdout:\n%s", stdout.String())
		t.Logf("Current stderr:\n%s", stderr.String())
		t.Fatal("Initial build did not complete within 5 seconds")
	}
	
	// Ensure we kill the process when done
	defer func() {
		if cmd.Process != nil {
			// Kill the process
			cmd.Process.Kill()
			// Wait for it to exit, but don't block forever
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case <-done:
				// Process exited cleanly
			case <-time.After(2 * time.Second):
				// Force kill if it doesn't exit in time
				cmd.Process.Signal(os.Kill)
			}
		}
	}()

	// Wait for server to be ready by looking for the HTTP server message
	var serverURL string
	deadline := time.Now().Add(15 * time.Second)
	
	for time.Now().Before(deadline) {
		outputStr := stdout.String() + stderr.String()
		if strings.Contains(outputStr, "HTTP server listening on") {
			// Extract the URL
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				if strings.Contains(line, "HTTP server listening on") {
					// Extract URL from line like "🌐 HTTP server listening on http://localhost:12345"
					parts := strings.Split(line, " ")
					for _, part := range parts {
						if strings.HasPrefix(part, "http://") {
							serverURL = strings.TrimSpace(part)
							break
						}
					}
				}
			}
			if serverURL != "" {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	if serverURL == "" {
		t.Logf("Stdout:\n%s", stdout.String())
		t.Logf("Stderr:\n%s", stderr.String())
		
		// Check if process is still running
		if cmd.Process != nil {
			if err := cmd.Process.Signal(nil); err != nil {
				t.Logf("Process has exited: %v", err)
			}
		}
		
		// Check if there was an error
		fullOutput := stdout.String() + stderr.String()
		if strings.Contains(fullOutput, "error") || strings.Contains(fullOutput, "Error") {
			t.Fatalf("okra dev failed with error. Output:\n%s", fullOutput)
		}
	}
	
	require.NotEmpty(t, serverURL, "Failed to find server URL in output")
	t.Logf("Server started at: %s", serverURL)

	// Wait a bit more for service deployment
	time.Sleep(2 * time.Second)
	

	// Test the service using ConnectRPC JSON protocol
	testConnectRPCCall(t, serverURL)
}

// copyEmbeddedFiles copies all embedded test files to the target directory
func copyEmbeddedFiles(targetDir string) error {
	// Copy all files and directories recursively
	return copyDir("testdata", targetDir)
}

// copyDir recursively copies embedded files
func copyDir(src, dst string) error {
	entries, err := testServiceFiles.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Create directory
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			// Recursively copy directory contents
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			content, err := testServiceFiles.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, content, 0644); err != nil {
				return err
			}
		}
	}
	
	// Don't create go.mod here - let okra dev handle module setup
	return nil
}

// testConnectRPCCall tests calling the service using ConnectRPC's JSON protocol
func testConnectRPCCall(t *testing.T, serverURL string) {
	// ConnectRPC JSON request format
	request := map[string]interface{}{
		"name": "Integration Test",
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	// Make HTTP request to the gRPC endpoint using Connect protocol
	// The URL format is: http://host:port/package.Service/Method
	// Note: method names must match exactly as defined in the schema
	url := fmt.Sprintf("%s/test.Service/greet", serverURL)
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	require.NoError(t, err)
	
	// Set Connect protocol headers for unary JSON
	// ConnectRPC supports both JSON and Protobuf. For JSON, we need the right content type
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response body: %s", string(body))

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK, got %d. Response: %s", resp.StatusCode, string(body))

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	// Validate response
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "timestamp")
	
	message, ok := response["message"].(string)
	require.True(t, ok, "message should be a string")
	assert.Equal(t, "Hello, Integration Test! Welcome to OKRA.", message)

	timestamp, ok := response["timestamp"].(string)
	require.True(t, ok, "timestamp should be a string")
	assert.NotEmpty(t, timestamp)
	
	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err, "timestamp should be valid RFC3339")
}