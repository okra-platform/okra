package build

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/okra-platform/okra/internal/codegen/typescript"
	"github.com/okra-platform/okra/internal/schema"
)

// buildTypeScriptWASM builds TypeScript source code to WASM
func (b *ServiceBuilder) buildTypeScriptWASM() error {
	// Use the existing TypeScript builder
	builder := NewTypeScriptBuilder(b.config, b.projectRoot, b.schema)
	if err := builder.Build(); err != nil {
		return fmt.Errorf("TypeScript build failed: %w", err)
	}
	
	// Check output file was created
	outputPath := filepath.Join(b.projectRoot, b.config.Build.Output)
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("build succeeded but output file not found: %w", err)
	}
	
	fileInfo, _ := os.Stat(outputPath)
	b.logger.Info().
		Str("output", filepath.Base(outputPath)).
		Int64("size", fileInfo.Size()).
		Dur("duration", time.Since(b.buildStart)).
		Msg("WASM build completed")
	
	return nil
}

// generateTypeScriptInterface generates TypeScript interface code
func (b *ServiceBuilder) generateTypeScriptInterface(parsedSchema *schema.Schema) error {
	// Create types directory
	typesDir := filepath.Join(b.projectRoot, "types")
	if err := os.MkdirAll(typesDir, 0755); err != nil {
		return fmt.Errorf("failed to create types directory: %w", err)
	}

	// Generate TypeScript interface using the existing generator
	generator := typescript.NewGenerator("types")
	code, err := generator.Generate(parsedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate TypeScript interface: %w", err)
	}

	// Write interface file
	interfacePath := filepath.Join(typesDir, "interface.ts")
	if err := os.WriteFile(interfacePath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write interface file: %w", err)
	}
	
	b.logger.Info().
		Str("path", interfacePath).
		Dur("duration", time.Since(b.buildStart)).
		Msg("generated interface code")
	
	return nil
}