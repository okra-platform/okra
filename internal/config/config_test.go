package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromPath(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with all fields",
			config: Config{
				Name:     "test-service",
				Version:  "1.0.0",
				Language: "go",
				Schema:   "./custom.okra.graphql",
				Source:   "./src",
				Build: BuildConfig{
					Output: "./dist/service.wasm",
				},
				Dev: DevConfig{
					Watch:   []string{"*.go", "*.graphql"},
					Exclude: []string{"vendor/", "*.test.go"},
				},
			},
		},
		{
			name: "config with defaults",
			config: Config{
				Name:     "minimal-service",
				Version:  "0.1.0",
				Language: "typescript",
			},
		},
		{
			name: "empty config file",
			config: Config{
				Language: "go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "okra.json")
			
			data, err := json.MarshalIndent(tt.config, "", "  ")
			require.NoError(t, err)
			
			err = os.WriteFile(configPath, data, 0644)
			require.NoError(t, err)

			// Test loading
			got, err := LoadConfigFromPath(configPath)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			
			require.NoError(t, err)
			require.NotNil(t, got)
			
			// Verify loaded config
			assert.Equal(t, tt.config.Name, got.Name)
			assert.Equal(t, tt.config.Version, got.Version)
			assert.Equal(t, tt.config.Language, got.Language)
			
			// Check defaults were applied
			if tt.config.Source == "" {
				assert.Equal(t, "./", got.Source)
			}
			if tt.config.Schema == "" {
				assert.Equal(t, "./service.okra.graphql", got.Schema)
			}
			if tt.config.Build.Output == "" {
				assert.Equal(t, "./build/service.wasm", got.Build.Output)
			}
			
			// Check language-specific defaults for watch patterns
			if len(tt.config.Dev.Watch) == 0 {
				switch got.Language {
				case "go":
					assert.Contains(t, got.Dev.Watch, "*.go")
					assert.Contains(t, got.Dev.Watch, "*.okra.graphql")
				case "typescript":
					assert.Contains(t, got.Dev.Watch, "*.ts")
					assert.Contains(t, got.Dev.Watch, "*.okra.graphql")
				}
			}
			
			// Check default excludes
			if len(tt.config.Dev.Exclude) == 0 {
				assert.Contains(t, got.Dev.Exclude, "*_test.go")
				assert.Contains(t, got.Dev.Exclude, "build/")
				assert.Contains(t, got.Dev.Exclude, "node_modules/")
				assert.Contains(t, got.Dev.Exclude, ".git/")
			}
		})
	}
}

func TestLoadConfigFromPath_Errors(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(string) string
		errContains string
	}{
		{
			name: "file not found",
			setupFunc: func(tmpDir string) string {
				return filepath.Join(tmpDir, "nonexistent.json")
			},
			errContains: "failed to read config file",
		},
		{
			name: "invalid json",
			setupFunc: func(tmpDir string) string {
				path := filepath.Join(tmpDir, "okra.json")
				os.WriteFile(path, []byte("invalid json"), 0644)
				return path
			},
			errContains: "failed to parse config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := tt.setupFunc(tmpDir)
			
			_, err := LoadConfigFromPath(configPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Test finding okra.json in current directory
	t.Run("config in current dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "okra.json")
		
		config := Config{
			Name:     "current-dir-service",
			Version:  "1.0.0",
			Language: "go",
		}
		
		data, _ := json.MarshalIndent(config, "", "  ")
		err := os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)
		
		// Change to temp dir
		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		err = os.Chdir(tmpDir)
		require.NoError(t, err)
		
		// Load config
		got, projectRoot, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, config.Name, got.Name)
		// Use filepath.EvalSymlinks to resolve any symlinks for comparison
		expectedRoot, _ := filepath.EvalSymlinks(tmpDir)
		actualRoot, _ := filepath.EvalSymlinks(projectRoot)
		assert.Equal(t, expectedRoot, actualRoot)
	})
	
	// Test finding okra.json in parent directory
	t.Run("config in parent dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)
		
		configPath := filepath.Join(tmpDir, "okra.json")
		config := Config{
			Name:     "parent-dir-service",
			Version:  "1.0.0",
			Language: "typescript",
		}
		
		data, _ := json.MarshalIndent(config, "", "  ")
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)
		
		// Change to subdirectory
		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		err = os.Chdir(subDir)
		require.NoError(t, err)
		
		// Load config
		got, projectRoot, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, config.Name, got.Name)
		// Use filepath.EvalSymlinks to resolve any symlinks for comparison
		expectedRoot, _ := filepath.EvalSymlinks(tmpDir)
		actualRoot, _ := filepath.EvalSymlinks(projectRoot)
		assert.Equal(t, expectedRoot, actualRoot)
	})
	
	// Test no okra.json found
	t.Run("no config found", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Change to temp dir
		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		err := os.Chdir(tmpDir)
		require.NoError(t, err)
		
		// Load config
		_, _, err = LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no okra.json found")
	})
}