package dev

import (
	"os"

	"github.com/okra-platform/okra/internal/codegen"
	"github.com/okra-platform/okra/internal/schema"
)

// SchemaParser defines the interface for parsing schemas
type SchemaParser interface {
	Parse(input string) (*schema.Schema, error)
}

// CodeGeneratorFactory creates code generators for different languages
type CodeGeneratorFactory interface {
	GetGenerator(language string) (codegen.Generator, error)
}

// WASMBuilder defines the interface for building WASM modules
type WASMBuilder interface {
	Build(projectRoot string, config BuildConfig) error
}

// BuildConfig contains configuration for building WASM
type BuildConfig struct {
	Language string
	Source   string
	Output   string
}

// FileSystem defines file system operations
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Stat(path string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
}