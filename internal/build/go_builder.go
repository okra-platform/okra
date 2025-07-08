package build

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
	"github.com/rs/zerolog"
)

// Embed the wrapper template
//go:embed templates/go/wrapper.go.tmpl
var goWrapperTemplate string

// GoBuilder handles the Go to WASM compilation pipeline
type GoBuilder struct {
	config      *config.Config
	projectRoot string
	schema      *schema.Schema
	logger      zerolog.Logger
}

// NewGoBuilder creates a new Go builder
func NewGoBuilder(cfg *config.Config, projectRoot string, s *schema.Schema) *GoBuilder {
	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("component", "go-builder").
		Logger()
		
	return &GoBuilder{
		config:      cfg,
		projectRoot: projectRoot,
		schema:      s,
		logger:      logger,
	}
}

// Build compiles Go source to WASM using TinyGo with hidden wrapper
func (b *GoBuilder) Build() error {
	// Verify TinyGo is available
	if _, err := exec.LookPath("tinygo"); err != nil {
		return fmt.Errorf("TinyGo not found in PATH. Please install TinyGo from https://tinygo.org/getting-started/install/")
	}
	
	// Create temporary build directory
	tmpDir, err := os.MkdirTemp("", "okra-go-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	b.logger.Debug().Str("tmpDir", tmpDir).Msg("created temp directory")
	// Check if we should keep the build directory for debugging
	// OKRA_KEEP_BUILD_DIR environment variable can be set to any non-empty value
	// to preserve the temporary build directory. This is useful for:
	// - Inspecting generated wrapper code
	// - Debugging build failures
	// - Examining intermediate build artifacts
	// See docs/10_development-debugging.md for more details
	keepBuildDir := os.Getenv("OKRA_KEEP_BUILD_DIR") != ""
	
	defer func() {
		if keepBuildDir {
			b.logger.Info().
				Str("path", tmpDir).
				Msg("OKRA_KEEP_BUILD_DIR is set - preserving temp build directory for debugging")
		} else {
			b.logger.Debug().Str("path", tmpDir).Msg("cleaning up temp build directory")
			os.RemoveAll(tmpDir)
		}
	}()
	b.logger.Debug().Str("path", tmpDir).Msg("created temp build directory")

	// Check go.mod exists
	b.logger.Debug().Str("projectRoot", b.projectRoot).Msg("checking go.mod exists")
	goModPath := filepath.Join(b.projectRoot, "go.mod")
	b.logger.Debug().Str("goModPath", goModPath).Msg("looking for go.mod at path")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in project root. Please run 'go mod init' first")
	}

	// Extract module path from go.mod
	b.logger.Debug().Msg("extracting module path from go.mod")
	modulePath, err := b.extractModulePath()
	if err != nil {
		return fmt.Errorf("failed to extract module path from go.mod: %w", err)
	}
	b.logger.Debug().Str("modulePath", modulePath).Msg("extracted module path")

	// Calculate the user service import path
	userServiceImport := modulePath
	if b.config.Source != "./" && b.config.Source != "." {
		// Clean the source path
		cleanSource := strings.TrimPrefix(b.config.Source, "./")
		
		// Check if source is a file or directory
		sourcePath := filepath.Join(b.projectRoot, b.config.Source)
		if info, err := os.Stat(sourcePath); err == nil && !info.IsDir() {
			// If source is a file, use the directory containing it
			cleanSource = filepath.Dir(cleanSource)
			// If the file is in the root, don't append anything
			if cleanSource == "." || cleanSource == "" {
				userServiceImport = modulePath
			} else {
				// Use forward slashes for import paths
				userServiceImport = modulePath + "/" + strings.ReplaceAll(cleanSource, string(filepath.Separator), "/")
			}
		} else {
			// If source is a directory, append it to the module path
			// Use forward slashes for import paths
			userServiceImport = modulePath + "/" + strings.TrimSuffix(strings.ReplaceAll(cleanSource, string(filepath.Separator), "/"), "/")
		}
	}

	// Generate wrapper in temp directory
	b.logger.Debug().
		Str("module_path", modulePath).
		Str("user_service_import", userServiceImport).
		Str("config_source", b.config.Source).
		Str("project_root", b.projectRoot).
		Msg("generating WASI wrapper")
	if err := b.generateWrapper(tmpDir, modulePath, userServiceImport); err != nil {
		return fmt.Errorf("failed to generate wrapper: %w", err)
	}

	// Create go.mod in temp directory
	if err := b.createTempGoMod(tmpDir, modulePath); err != nil {
		return fmt.Errorf("failed to create temp go.mod: %w", err)
	}

	// Copy the types directory to temp directory so TinyGo can find it
	typesDir := filepath.Join(b.projectRoot, "types")
	if _, err := os.Stat(typesDir); err == nil {
		tempTypesDir := filepath.Join(tmpDir, "types")
		b.logger.Debug().
			Str("from", typesDir).
			Str("to", tempTypesDir).
			Msg("copying types directory to temp build")
		if err := b.copyDir(typesDir, tempTypesDir); err != nil {
			return fmt.Errorf("failed to copy types directory: %w", err)
		}
	}
	
	// Copy the service source directory
	sourcePath := filepath.Join(b.projectRoot, b.config.Source)
	b.logger.Debug().
		Str("source_path", sourcePath).
		Str("config_source", b.config.Source).
		Msg("checking service source directory")
	
	if info, err := os.Stat(sourcePath); err == nil {
		if info.IsDir() {
			// Source is a directory, copy it
			// Clean the source path to remove ./ prefix
			cleanSource := strings.TrimPrefix(b.config.Source, "./")
			destPath := filepath.Join(tmpDir, cleanSource)
			b.logger.Debug().
				Str("from", sourcePath).
				Str("to", destPath).
				Msg("copying service source directory")
			if err := b.copyDir(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to copy service directory: %w", err)
			}
		} else {
			// Source is a single file, copy it to temp directory
			filename := filepath.Base(sourcePath)
			destPath := filepath.Join(tmpDir, filename)
			b.logger.Debug().
				Str("from", sourcePath).
				Str("to", destPath).
				Msg("copying service source file")
			if err := b.copyFile(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to copy service file: %w", err)
			}
		}
	} else {
		b.logger.Debug().
			Str("source_path", sourcePath).
			Err(err).
			Msg("service source path not found")
	}

	// Log the temp directory contents for debugging
	if b.logger.Debug().Enabled() {
		b.logger.Debug().Msg("temp build directory contents:")
		b.logDirContents(tmpDir, "  ")
	}
	
	// Run TinyGo build
	b.logger.Debug().Str("tmpDir", tmpDir).Msg("starting TinyGo build")
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
	file.Close()
	
	// Log the generated wrapper for debugging
	if b.logger.Debug().Enabled() {
		wrapperContent, _ := os.ReadFile(wrapperPath)
		b.logger.Debug().
			Str("path", wrapperPath).
			Str("content", string(wrapperContent)).
			Msg("generated WASI wrapper")
	}

	return nil
}

// createTempGoMod creates a go.mod file in the temp directory
func (b *GoBuilder) createTempGoMod(tmpDir string, userModulePath string) error {
	// Read go version from user's go.mod
	userGoMod, err := os.ReadFile(filepath.Join(b.projectRoot, "go.mod"))
	if err != nil {
		return fmt.Errorf("failed to read user's go.mod: %w", err)
	}
	
	// Extract go version
	goVersion := "1.21"
	lines := strings.Split(string(userGoMod), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go"))
			break
		}
	}
	
	goModContent := fmt.Sprintf(`module okra-temp-build

go %s

require %s v0.0.0

replace %s => %s
`, goVersion, userModulePath, userModulePath, b.projectRoot)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}

// runTinyGoBuild executes TinyGo to compile the WASM module
func (b *GoBuilder) runTinyGoBuild(tmpDir string) error {
	fmt.Println("   🔧 Running TinyGo build...")
	
	args := []string{
		"build",
		"-o", "service.wasm",
		"-target", "wasi",
		"-scheduler=none",
		"-gc=conservative",
		"-opt=2",
		"-no-debug",
		".",
	}
	
	b.logger.Debug().
		Str("dir", tmpDir).
		Strs("args", args).
		Msg("executing TinyGo build")
	
	cmd := exec.Command("tinygo", args...)
	cmd.Dir = tmpDir

	b.logger.Debug().
		Str("command", "tinygo " + strings.Join(args, " ")).
		Str("working_dir", tmpDir).
		Msg("executing TinyGo command")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Provide detailed error information
		errorMsg := fmt.Sprintf("TinyGo build failed with error: %v", err)
		if len(output) > 0 {
			errorMsg += fmt.Sprintf("\n\nTinyGo output:\n%s", string(output))
		}
		errorMsg += fmt.Sprintf("\n\nBuild directory: %s", tmpDir)
		errorMsg += "\n\nCommon issues:"
		errorMsg += "\n  - Missing imports in service implementation"
		errorMsg += "\n  - Syntax errors in generated or user code"
		errorMsg += "\n  - Incompatible Go features for TinyGo/WASI target"
		return fmt.Errorf("%s", errorMsg)
	}
	
	// Log successful build
	b.logger.Debug().Msg("TinyGo build completed successfully")

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

// copyDir recursively copies a directory
func (b *GoBuilder) copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip go.mod files to avoid module conflicts
		if entry.Name() == "go.mod" || entry.Name() == "go.sum" {
			b.logger.Debug().
				Str("file", entry.Name()).
				Str("dir", src).
				Msg("skipping go.mod/go.sum file to avoid module conflicts")
			continue
		}
		
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := b.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := b.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
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

// logDirContents recursively logs directory contents for debugging
func (b *GoBuilder) logDirContents(dir string, indent string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		b.logger.Debug().Msgf("%s%s", indent, entry.Name())
		if entry.IsDir() {
			b.logDirContents(filepath.Join(dir, entry.Name()), indent+"  ")
		}
	}
}
