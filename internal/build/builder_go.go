package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/okra-platform/okra/internal/codegen/golang"
	"github.com/okra-platform/okra/internal/codegen/protobuf"
	"github.com/okra-platform/okra/internal/schema"
)

// buildGoWASM builds Go source code to WASM
func (b *ServiceBuilder) buildGoWASM() error {
	// Use the existing Go builder with hidden wrapper
	builder := NewGoBuilder(b.config, b.projectRoot, b.schema)
	if err := builder.Build(); err != nil {
		return fmt.Errorf("Go build failed: %w", err)
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

// generateLanguageInterface generates Go interface code
func (b *ServiceBuilder) generateLanguageInterface(parsedSchema *schema.Schema) error {
	if b.config.Language != "go" {
		return b.generateTypeScriptInterface(parsedSchema)
	}
	
	// Create types directory
	typesDir := filepath.Join(b.projectRoot, "types")
	if err := os.MkdirAll(typesDir, 0755); err != nil {
		return fmt.Errorf("failed to create types directory: %w", err)
	}

	// Generate Go interface using the existing generator
	generator := golang.NewGenerator("types")
	code, err := generator.Generate(parsedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate Go interface: %w", err)
	}

	// Write interface file
	interfacePath := filepath.Join(typesDir, "interface.go")
	if err := os.WriteFile(interfacePath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write interface file: %w", err)
	}
	
	b.logger.Info().
		Str("path", interfacePath).
		Dur("duration", time.Since(b.buildStart)).
		Msg("generated interface code")
	
	return nil
}

// generateProtobufDescriptor generates protobuf descriptor from schema
func (b *ServiceBuilder) generateProtobufDescriptor(parsedSchema *schema.Schema) error {
	// Extract namespace from schema or use default
	namespace := parsedSchema.Meta.Namespace
	if namespace == "" {
		namespace = "service"
	}
	b.logger.Debug().
		Str("namespace", namespace).
		Msg("generating protobuf with namespace")
	
	// Generate protobuf using the existing generator
	protoGen := protobuf.NewGenerator(namespace)
	protoContent, err := protoGen.Generate(parsedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate protobuf: %w", err)
	}
	
	// Write protobuf file
	protoPath := filepath.Join(b.okraDir, "service.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0644); err != nil {
		return fmt.Errorf("failed to write protobuf file: %w", err)
	}
	
	// Check if buf is installed
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf CLI is not installed. Please install it from https://buf.build/docs/installation")
	}
	
	// Create buf.yaml if it doesn't exist
	bufYamlPath := filepath.Join(b.okraDir, "buf.yaml")
	bufYamlContent := `version: v1
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
`
	if err := os.WriteFile(bufYamlPath, []byte(bufYamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write buf.yaml: %w", err)
	}
	
	// Compile protobuf to descriptor set
	descPath := filepath.Join(b.okraDir, "service.pb.desc")
	cmd := exec.Command("buf", "build", "--output", descPath, "--as-file-descriptor-set", protoPath)
	cmd.Dir = b.okraDir
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to compile protobuf with buf: %w\nOutput: %s", err, output)
	}
	
	b.logger.Info().
		Str("descriptor", "service.pb.desc").
		Msg("generated protobuf descriptor")
	
	return nil
}