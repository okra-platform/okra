package dev

import (
	_ "embed"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
)

// Embed the wrapper template
//go:embed templates/go/wrapper.go.tmpl
var goWrapperTemplate string

// GoBuilder handles the Go to WASM compilation pipeline
type GoBuilder struct {
	config      *config.Config
	projectRoot string
	schema      *schema.Schema
}

// NewGoBuilder creates a new Go builder
func NewGoBuilder(cfg *config.Config, projectRoot string, s *schema.Schema) *GoBuilder {
	return &GoBuilder{
		config:      cfg,
		projectRoot: projectRoot,
		schema:      s,
	}
}

// Build compiles Go source to WASM using TinyGo with hidden wrapper
func (b *GoBuilder) Build() error {
	// Create temporary build directory
	tmpDir, err := os.MkdirTemp("", "okra-go-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract module path from go.mod
	modulePath, err := b.extractModulePath()
	if err != nil {
		return fmt.Errorf("failed to extract module path: %w", err)
	}

	// Calculate the user service import path
	userServiceImport := modulePath
	if b.config.Source != "./" && b.config.Source != "." {
		// If source is a subdirectory, append it to the module path
		// Use forward slashes for import paths, not filepath.Join
		userServiceImport = modulePath + "/" + strings.TrimPrefix(b.config.Source, "./")
	}

	// Generate wrapper in temp directory
	if err := b.generateWrapper(tmpDir, modulePath, userServiceImport); err != nil {
		return fmt.Errorf("failed to generate wrapper: %w", err)
	}

	// Create go.mod in temp directory
	if err := b.createTempGoMod(tmpDir, modulePath); err != nil {
		return fmt.Errorf("failed to create temp go.mod: %w", err)
	}

	// Run TinyGo build
	if err := b.runTinyGoBuild(tmpDir); err != nil {
		return fmt.Errorf("TinyGo build failed: %w", err)
	}

	// Copy WASM to final location
	tempWASMPath := filepath.Join(tmpDir, "service.wasm")
	finalPath := filepath.Join(b.projectRoot, b.config.Build.Output)
	if err := b.copyFile(tempWASMPath, finalPath); err != nil {
		return fmt.Errorf("failed to copy WASM file: %w", err)
	}

	return nil
}

// extractModulePath reads the module path from go.mod
func (b *GoBuilder) extractModulePath() (string, error) {
	goModPath := filepath.Join(b.projectRoot, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	modulePattern := regexp.MustCompile(`^\s*module\s+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := modulePattern.FindStringSubmatch(line); len(matches) > 1 {
			return strings.TrimSpace(matches[1]), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// generateWrapper creates the WASI wrapper file
func (b *GoBuilder) generateWrapper(tmpDir string, modulePath string, userServiceImport string) error {
	// Get service information from schema
	if len(b.schema.Services) == 0 {
		return fmt.Errorf("no services found in schema")
	}

	// Use the first service (OKRA typically has one service per schema)
	service := b.schema.Services[0]

	// Prepare template data
	type MethodData struct {
		Name       string
		LowerName  string
		InputType  string
		OutputType string
	}

	methods := make([]MethodData, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, MethodData{
			Name:       b.exportName(method.Name),
			LowerName:  strings.ToLower(method.Name[:1]) + method.Name[1:],
			InputType:  b.mapToGoType(method.InputType),
			OutputType: b.mapToGoType(method.OutputType),
		})
	}

	data := struct {
		UserPackageImport    string
		ModulePath           string
		ServiceInterfaceName string
		ConstructorName      string
		Methods              []MethodData
	}{
		UserPackageImport:    userServiceImport,
		ModulePath:           modulePath, // Base module path for types import
		ServiceInterfaceName: service.Name,
		ConstructorName:      "NewService", // Convention: user must export NewService()
		Methods:              methods,
	}

	// Parse and execute template
	tmpl, err := template.New("wrapper").Parse(goWrapperTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse wrapper template: %w", err)
	}

	wrapperPath := filepath.Join(tmpDir, "main.go")
	file, err := os.Create(wrapperPath)
	if err != nil {
		return fmt.Errorf("failed to create wrapper file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute wrapper template: %w", err)
	}

	return nil
}

// createTempGoMod creates a go.mod file in the temp directory
func (b *GoBuilder) createTempGoMod(tmpDir string, userModulePath string) error {
	goModContent := fmt.Sprintf(`module okra-temp-build

go 1.21

require %s v0.0.0

replace %s => %s
`, userModulePath, userModulePath, b.projectRoot)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}

// runTinyGoBuild executes TinyGo to compile the WASM module
func (b *GoBuilder) runTinyGoBuild(tmpDir string) error {
	cmd := exec.Command(
		"tinygo", "build",
		"-o", "service.wasm",
		"-target", "wasi",
		"-scheduler=none",
		"-gc=conservative",
		"-opt=2",
		"-no-debug",
		".",
	)
	cmd.Dir = tmpDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TinyGo error: %s\n%s", err, string(output))
	}

	return nil
}

// copyFile copies a file from source to destination
func (b *GoBuilder) copyFile(src, dst string) error {
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

// exportName converts a name to exported Go name
func (b *GoBuilder) exportName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// mapToGoType maps schema types to Go types (simplified version)
func (b *GoBuilder) mapToGoType(typ string) string {
	// Remove array brackets if present
	if strings.HasPrefix(typ, "[") && strings.HasSuffix(typ, "]") {
		inner := typ[1 : len(typ)-1]
		return "[]" + b.mapToGoType(inner)
	}

	// For custom types, just use the name
	// The actual type mapping is handled by the code generator
	return typ
}