package commands

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for Deploy command:
// 1. Test Deploy executes successfully
// 2. Test Deploy outputs expected message
// 3. Test Deploy with context cancellation

func TestController_Deploy(t *testing.T) {
	// Test: Deploy executes successfully
	tmpDir := t.TempDir()
	oldPwd, _ := os.Getwd()
	defer os.Chdir(oldPwd)
	os.Chdir(tmpDir)

	controller := &Controller{}
	ctx := context.Background()

	err := controller.Deploy(ctx)
	assert.NoError(t, err)
}

func TestController_Deploy_OutputsMessage(t *testing.T) {
	// Test: Deploy outputs expected message
	tmpDir := t.TempDir()
	oldPwd, _ := os.Getwd()
	defer os.Chdir(oldPwd)
	os.Chdir(tmpDir)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	controller := &Controller{}
	ctx := context.Background()

	err := controller.Deploy(ctx)
	require.NoError(t, err)

	// Restore stdout and read output
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	assert.Contains(t, output, "Deploying OKRA service...")
}

func TestController_Deploy_WithCancelledContext(t *testing.T) {
	// Test: Deploy with cancelled context
	tmpDir := t.TempDir()
	oldPwd, _ := os.Getwd()
	defer os.Chdir(oldPwd)
	os.Chdir(tmpDir)

	controller := &Controller{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Current implementation doesn't check context, so it will still succeed
	err := controller.Deploy(ctx)
	assert.NoError(t, err)
}