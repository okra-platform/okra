package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the okra.json configuration file
type Config struct {
	Name     string      `json:"name"`
	Version  string      `json:"version"`
	Language string      `json:"language"`
	Schema   string      `json:"schema"`
	Source   string      `json:"source"`
	Build    BuildConfig `json:"build"`
	Dev      DevConfig   `json:"dev"`
}

// BuildConfig contains build-specific configuration
type BuildConfig struct {
	Output string `json:"output"`
}

// DevConfig contains development server configuration
type DevConfig struct {
	Watch   []string `json:"watch"`
	Exclude []string `json:"exclude"`
}

// LoadConfig loads the okra.json configuration from the current directory or a parent directory
func LoadConfig() (*Config, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return loadConfigFromDir(dir)
}

// LoadConfigFromPath loads the okra.json configuration from a specific path
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Source == "" {
		config.Source = "./"
	}
	if config.Schema == "" {
		config.Schema = "./service.okra.gql"
	}
	if config.Build.Output == "" {
		config.Build.Output = "./build/service.wasm"
	}
	if len(config.Dev.Watch) == 0 {
		// Set default watch patterns based on language
		switch config.Language {
		case "go":
			config.Dev.Watch = []string{"*.go", "**/*.go", "*.okra.gql", "**/*.okra.gql"}
		case "typescript":
			config.Dev.Watch = []string{"*.ts", "**/*.ts", "*.okra.gql", "**/*.okra.gql"}
		default:
			config.Dev.Watch = []string{"*.okra.gql", "**/*.okra.gql"}
		}
	}
	if len(config.Dev.Exclude) == 0 {
		config.Dev.Exclude = []string{"*_test.go", "build/", "node_modules/", ".git/", "service.interface.go", "service.interface.ts"}
	}

	return &config, nil
}

// loadConfigFromDir searches for okra.json in the given directory and its parents
func loadConfigFromDir(startDir string) (*Config, string, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, "okra.json")
		if _, err := os.Stat(configPath); err == nil {
			config, err := LoadConfigFromPath(configPath)
			if err != nil {
				return nil, "", err
			}
			return config, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	return nil, "", fmt.Errorf("no okra.json found in %s or any parent directory", startDir)
}
