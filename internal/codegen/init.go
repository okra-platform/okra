package codegen

import (
	"github.com/okra-platform/okra/internal/codegen/golang"
	"github.com/okra-platform/okra/internal/codegen/typescript"
)

// DefaultRegistry is the global registry instance with pre-registered generators
var DefaultRegistry = NewRegistry()

func init() {
	// Register Go generator
	DefaultRegistry.Register("go", func(packageName string) Generator {
		return golang.NewGenerator(packageName)
	})
	
	// Register TypeScript generator
	DefaultRegistry.Register("typescript", func(packageName string) Generator {
		return typescript.NewGenerator(packageName)
	})
	
	// Register ts as an alias for typescript
	DefaultRegistry.Register("ts", func(packageName string) Generator {
		return typescript.NewGenerator(packageName)
	})
}