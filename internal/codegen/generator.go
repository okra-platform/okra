package codegen

import "github.com/okra-platform/okra/internal/schema"

// Generator is the interface that all language-specific code generators must implement
type Generator interface {
	// Generate generates code from the schema and returns the generated code as bytes
	Generate(schema *schema.Schema) ([]byte, error)
	
	// Language returns the name of the target language (e.g., "go", "typescript")
	Language() string
	
	// FileExtension returns the file extension for generated files (e.g., ".go", ".ts")
	FileExtension() string
}

// Options contains common options for code generation
type Options struct {
	// PackageName is the package/module name for the generated code
	PackageName string
	
	// OutputPath is the directory where files should be generated
	OutputPath string
	
	// IncludeComments determines whether to include documentation comments
	IncludeComments bool
	
	// CustomOptions allows language-specific options
	CustomOptions map[string]interface{}
}