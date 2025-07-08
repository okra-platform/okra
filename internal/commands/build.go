package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okra-platform/okra/internal/build"
	"github.com/okra-platform/okra/internal/config"
	"github.com/rs/zerolog"
)

// Build compiles OKRA services into packages
func (c *Controller) Build(ctx context.Context) error {
	// Set up logger
	logLevel := zerolog.InfoLevel
	if c.Flags.LogLevel != "" {
		level, err := zerolog.ParseLevel(c.Flags.LogLevel)
		if err == nil {
			logLevel = level
		}
	}
	
	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("component", "build").
		Logger().
		Level(logLevel)
	
	fmt.Println("🔨 Building OKRA service package...")
	logger.Info().Msg("starting build process")
	
	// Get current directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	// Load configuration
	cfg, configPath, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w\n💡 Run 'okra init' to create a new project", err)
	}
	
	logger.Debug().Str("path", configPath).Msg("loaded configuration")
	
	logger.Info().
		Str("name", cfg.Name).
		Str("version", cfg.Version).
		Str("language", cfg.Language).
		Msg("loaded project configuration")
	
	// Validate configuration
	if err := validateBuildConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create builder
	builder := build.NewServiceBuilder(cfg, projectRoot, logger)
	
	// Generate code from schema
	schemaPath := filepath.Join(projectRoot, cfg.Schema)
	fmt.Printf("📝 Generating code from schema: %s\n", cfg.Schema)
	if err := builder.GenerateCode(schemaPath); err != nil {
		logger.Error().Err(err).Msg("code generation failed")
		return fmt.Errorf("code generation failed: %w", err)
	}
	
	// Build WASM
	fmt.Printf("🏗️  Building WASM module...\n")
	if err := builder.BuildWASM(); err != nil {
		logger.Error().Err(err).Msg("WASM build failed")
		return fmt.Errorf("WASM build failed: %w", err)
	}
	
	// Get build artifacts
	artifacts, err := builder.GetArtifacts()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get build artifacts")
		return fmt.Errorf("failed to get build artifacts: %w", err)
	}
	
	// Create package
	packageName := fmt.Sprintf("%s-%s.okra.pkg", cfg.Name, cfg.Version)
	packagePath := filepath.Join(projectRoot, "dist", packageName)
	
	fmt.Printf("📦 Creating package: %s\n", packageName)
	packager := build.NewPackager(cfg)
	if err := packager.CreatePackage(artifacts, packagePath); err != nil {
		logger.Error().Err(err).Msg("package creation failed")
		return fmt.Errorf("package creation failed: %w", err)
	}
	
	// Get package info
	packageInfo, err := os.Stat(packagePath)
	if err != nil {
		return fmt.Errorf("failed to stat package: %w", err)
	}
	
	// Success!
	fmt.Println("\n✅ Build completed successfully!")
	fmt.Printf("📦 Package: %s\n", packagePath)
	fmt.Printf("📏 Size: %s\n", formatBytes(packageInfo.Size()))
	
	// Provide deployment hint
	fmt.Println("\n💡 Deploy your service with:")
	fmt.Printf("   okra deploy %s\n", packagePath)
	
	logger.Info().
		Str("package", packagePath).
		Int64("size", packageInfo.Size()).
		Msg("build completed successfully")
	
	return nil
}

// validateBuildConfig validates the configuration for building
func validateBuildConfig(cfg *config.Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("service name is required")
	}
	
	if cfg.Version == "" {
		return fmt.Errorf("service version is required")
	}
	
	if cfg.Schema == "" {
		return fmt.Errorf("schema path is required")
	}
	
	if cfg.Source == "" {
		return fmt.Errorf("source path is required")
	}
	
	if cfg.Language == "" {
		return fmt.Errorf("language is required")
	}
	
	// Validate language
	switch cfg.Language {
	case "go", "typescript":
		// Valid
	default:
		return fmt.Errorf("unsupported language: %s (supported: go, typescript)", cfg.Language)
	}
	
	return nil
}

// formatBytes formats bytes into human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}