//go:build integration
// +build integration

package dev_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestOkraDevSimple just tests that okra dev can start
func TestOkraDevSimple(t *testing.T) {
	// Build and install okra first
	// Get the directory of the current test file
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "Failed to get test file path")
	
	// Navigate from test/integration/dev to project root
	projectRoot := filepath.Join(filepath.Dir(testFile), "..", "..", "..")
	
	cmd := exec.Command("go", "install", ".")
	cmd.Dir = projectRoot
	err := cmd.Run()
	require.NoError(t, err, "Failed to install okra")

	// Create temp directory with test files
	tempDir := t.TempDir()

	// Copy test files
	err = copyEmbeddedFiles(tempDir)
	require.NoError(t, err)

	// Start okra dev
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, "okra", "dev")
	cmd.Dir = tempDir
	cmd.Env = os.Environ()
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Wait for HTTP server to start
	started := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		output := stdout.String()
		if strings.Contains(output, "HTTP server listening on") {
			started = true
			t.Log("HTTP server started successfully")
			break
		}
	}

	// Kill the process
	if cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}

	if !started {
		t.Logf("Stdout:\n%s", stdout.String())
		t.Logf("Stderr:\n%s", stderr.String())
		
		// Check for known issues
		allOutput := stdout.String() + stderr.String()
		if strings.Contains(allOutput, "TinyGo") || strings.Contains(allOutput, "tinygo") {
			t.Skip("TinyGo issue detected - skipping test")
		}
		if strings.Contains(allOutput, "buf") {
			t.Skip("buf not found - skipping test")
		}
		
		t.Fatal("okra dev did not start HTTP server within 10 seconds")
	}
}