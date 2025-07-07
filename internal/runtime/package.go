package runtime

import (
	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
)

// Type aliases to work around the compilation issue
type Method = schema.Method

// ServicePackage encapsulates everything needed to run an OKRA service
type ServicePackage struct {
	// Module is the compiled WASM module
	Module wasm.WASMCompiledModule
	
	// Schema describes the service interface
	Schema *schema.Schema
	
	// Config contains service configuration
	Config *config.Config
	
	// ServiceName is the primary service name from the schema
	ServiceName string
	
	// Methods maps method names to their definitions for quick lookup
	Methods map[string]*Method
}

// NewServicePackage creates a new service package with validation
func NewServicePackage(module wasm.WASMCompiledModule, schema *schema.Schema, config *config.Config) (*ServicePackage, error) {
	if module == nil {
		return nil, ErrNilModule
	}
	if schema == nil {
		return nil, ErrNilSchema
	}
	if config == nil {
		return nil, ErrNilConfig
	}
	if len(schema.Services) == 0 {
		return nil, ErrNoServices
	}
	
	// Build method lookup map from the first service
	// (OKRA typically has one service per schema)
	service := schema.Services[0]
	methods := make(map[string]*Method)
	for i := range service.Methods {
		methods[service.Methods[i].Name] = &service.Methods[i]
	}
	
	return &ServicePackage{
		Module:      module,
		Schema:      schema,
		Config:      config,
		ServiceName: service.Name,
		Methods:     methods,
	}, nil
}

// GetMethod returns the method definition for the given name
func (sp *ServicePackage) GetMethod(name string) (*Method, bool) {
	method, ok := sp.Methods[name]
	return method, ok
}