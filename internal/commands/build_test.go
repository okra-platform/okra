package commands

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for Build command:
// 1. Test successful build with valid project
// 2. Test build with missing configuration
// 3. Test build with invalid configuration
// 4. Test build creates correct package structure
// 5. Test build handles missing schema file
// 6. Test build handles missing source files

func TestBuild_MissingConfig(t *testing.T) {
	// Test: Build fails gracefully when no config file exists
	
	// Create temp directory without config
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd)
	
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	
	// Run build command
	controller := &Controller{
		Flags: &Flags{},
	}
	
	err = controller.Build(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load configuration")
	assert.Contains(t, err.Error(), "okra init")
}

func TestBuild_InvalidConfig(t *testing.T) {
	// Test: Build validates configuration properly
	
	tests := []struct {
		name        string
		config      string
		expectedErr string
	}{
		{
			name: "missing name",
			config: `{
				"version": "1.0.0",
				"language": "go",
				"schema": "./service.okra.gql",
				"source": "./service"
			}`,
			expectedErr: "service name is required",
		},
		{
			name: "missing version",
			config: `{
				"name": "test-service",
				"language": "go",
				"schema": "./service.okra.gql",
				"source": "./service"
			}`,
			expectedErr: "service version is required",
		},
		{
			name: "unsupported language",
			config: `{
				"name": "test-service",
				"version": "1.0.0",
				"language": "rust",
				"schema": "./service.okra.gql",
				"source": "./service"
			}`,
			expectedErr: "unsupported language: rust",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory with invalid config
			tempDir := t.TempDir()
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(oldWd)
			
			err = os.Chdir(tempDir)
			require.NoError(t, err)
			
			// Write config
			err = os.WriteFile("okra.json", []byte(tt.config), 0644)
			require.NoError(t, err)
			
			// Run build command
			controller := &Controller{
				Flags: &Flags{},
			}
			
			err = controller.Build(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestBuild_MissingSchema(t *testing.T) {
	// Test: Build fails when schema file doesn't exist
	
	// Create temp directory with config but no schema
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd)
	
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	
	// Write valid config
	config := `{
		"name": "test-service",
		"version": "1.0.0",
		"language": "go",
		"schema": "./service.okra.gql",
		"source": "./service"
	}`
	err = os.WriteFile("okra.json", []byte(config), 0644)
	require.NoError(t, err)
	
	// Create source directory
	err = os.MkdirAll("service", 0755)
	require.NoError(t, err)
	
	// Run build command
	controller := &Controller{
		Flags: &Flags{},
	}
	
	err = controller.Build(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema file not found")
}

func TestFormatBytes(t *testing.T) {
	// Test: formatBytes formats sizes correctly
	
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}