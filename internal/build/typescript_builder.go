package build

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
)

// Embed the template files
//
//go:embed templates/typescript/javy-runtime.ts
var javyRuntimeTemplate string

//go:embed templates/typescript/wrapper.ts.tmpl
var wrapperTemplateContent string

// TypeScriptBuilder handles the TypeScript to WASM compilation pipeline
type TypeScriptBuilder struct {
	config      *config.Config
	projectRoot string
	schema      *schema.Schema
}

// NewTypeScriptBuilder creates a new TypeScript builder
func NewTypeScriptBuilder(cfg *config.Config, projectRoot string, s *schema.Schema) *TypeScriptBuilder {
	return &TypeScriptBuilder{
		config:      cfg,
		projectRoot: projectRoot,
		schema:      s,
	}
}

// Build compiles TypeScript source to WASM using ESBuild and Javy
func (b *TypeScriptBuilder) Build() error {
	// Check dependencies first
	if err := b.checkDependencies(); err != nil {
		return err
	}

	// Create temporary build directory
	tmpDir, err := os.MkdirTemp("", "okra-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up when done

	// Step 1: Extract method names from schema
	methods := b.getServiceMethods()

	// Step 2: Generate build files in temp directory
	if err := b.generateBuildFiles(tmpDir, methods); err != nil {
		return fmt.Errorf("failed to generate build files: %w", err)
	}

	// Step 3: Run ESBuild
	bundlePath := filepath.Join(tmpDir, "bundle.js")
	if err := b.runESBuild(tmpDir, bundlePath); err != nil {
		return fmt.Errorf("ESBuild failed: %w", err)
	}

	// Step 4: Run Javy
	tempWASMPath := filepath.Join(tmpDir, "output.wasm")
	if err := b.runJavy(bundlePath, tempWASMPath); err != nil {
		return fmt.Errorf("Javy compilation failed: %w", err)
	}

	// Step 5: Copy WASM to final location
	finalPath := filepath.Join(b.projectRoot, b.config.Build.Output)
	if err := b.copyFile(tempWASMPath, finalPath); err != nil {
		return fmt.Errorf("failed to copy WASM file: %w", err)
	}

	return nil
}

// checkDependencies verifies that required tools are installed
func (b *TypeScriptBuilder) checkDependencies() error {
	// Check for Node.js/npm
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found. Please install Node.js: https://nodejs.org/")
	}

	// Check if node_modules exists (user ran npm install)
	nodeModulesPath := filepath.Join(b.projectRoot, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		return fmt.Errorf("node_modules not found. Run 'npm install' in your project directory")
	}

	// Check for Javy last since it's the least likely to be installed
	if _, err := exec.LookPath("javy"); err != nil {
		return fmt.Errorf("javy not found. Install with: npm install -g @shopify/javy")
	}

	return nil
}

// getServiceMethods extracts method names from the parsed schema
func (b *TypeScriptBuilder) getServiceMethods() []string {
	methods := []string{}

	// Get all methods from all services in the schema
	for _, service := range b.schema.Services {
		for _, method := range service.Methods {
			methods = append(methods, method.Name)
		}
	}

	return methods
}

// generateBuildFiles creates all necessary files in the temp directory
func (b *TypeScriptBuilder) generateBuildFiles(tmpDir string, methods []string) error {
	// Write Javy runtime
	runtimePath := filepath.Join(tmpDir, "javy-runtime.ts")
	if err := os.WriteFile(runtimePath, []byte(javyRuntimeTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write runtime: %w", err)
	}

	// Generate and write wrapper
	wrapperPath := filepath.Join(tmpDir, "service.wrapper.ts")
	if err := b.generateWrapper(wrapperPath, methods); err != nil {
		return fmt.Errorf("failed to generate wrapper: %w", err)
	}

	return nil
}

// generateWrapper creates the service wrapper file
func (b *TypeScriptBuilder) generateWrapper(outputPath string, methods []string) error {
	// Calculate relative path from temp dir to user's source
	userSourceDir := filepath.Join(b.projectRoot, b.config.Source)
	relPath, err := filepath.Rel(filepath.Dir(outputPath), userSourceDir)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Ensure forward slashes for import paths
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	if !strings.HasPrefix(relPath, ".") {
		relPath = "./" + relPath
	}

	// Create template data
	data := struct {
		UserModulePath string
		Methods        []string
	}{
		UserModulePath: relPath + "/index",
		Methods:        methods,
	}

	// Parse and execute template
	tmpl, err := template.New("wrapper").Parse(wrapperTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse wrapper template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create wrapper file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute wrapper template: %w", err)
	}

	return nil
}

// runESBuild executes the ESBuild bundler
func (b *TypeScriptBuilder) runESBuild(tmpDir, outputPath string) error {
	wrapperPath := filepath.Join(tmpDir, "service.wrapper.ts")

	args := []string{
		"esbuild",
		wrapperPath,
		"--bundle",
		"--platform=neutral",
		"--format=esm",
		"--target=es2020",
		"--outfile=" + outputPath,
		"--external:javy",
	}

	// Add tsconfig if it exists
	tsconfigPath := filepath.Join(b.projectRoot, "tsconfig.json")
	if _, err := os.Stat(tsconfigPath); err == nil {
		args = append(args, "--tsconfig="+tsconfigPath)
	}

	cmd := exec.Command("npx", args...)
	cmd.Dir = b.projectRoot // Run from project root for node_modules access

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ESBuild error: %s\n%s", err, string(output))
	}

	return nil
}

// runJavy executes the Javy compiler
func (b *TypeScriptBuilder) runJavy(inputPath, outputPath string) error {
	cmd := exec.Command("javy", "compile", inputPath, "-o", outputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Javy error: %s\n%s", err, string(output))
	}

	return nil
}

// copyFile copies a file from source to destination
func (b *TypeScriptBuilder) copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}
